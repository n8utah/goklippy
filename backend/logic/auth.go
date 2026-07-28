package logic

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidToken       = errors.New("invalid token")
)

type AuthConfig struct {
	AccessSecret    string
	RefreshSecret   string
	AccessTTL       time.Duration
	RefreshTokenTTL time.Duration
}

type AuthUser struct {
	ID           int64  `db:"id"`
	Email        string `db:"email"`
	Role         string `db:"role"`
	PasswordHash string `db:"password_hash"`
}

type RefreshTokenRecord struct {
	TokenID   string     `db:"token_id"`
	UserID    int64      `db:"user_id"`
	ExpiresAt time.Time  `db:"expires_at"`
	RevokedAt *time.Time `db:"revoked_at"`
}

type AccessClaims struct {
	UserID int64
	Email  string
	Role   string
}

type AuthStore interface {
	GetUserByEmail(ctx context.Context, email string) (*AuthUser, error)
	SaveRefreshToken(ctx context.Context, tokenID string, userID int64, expiresAt time.Time) error
	GetRefreshToken(ctx context.Context, tokenID string) (*RefreshTokenRecord, error)
	RevokeRefreshToken(ctx context.Context, tokenID string) error
}

type AuthService struct {
	store  AuthStore
	config AuthConfig
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

func NewAuthService(store AuthStore, cfg AuthConfig) *AuthService {
	return &AuthService{store: store, config: cfg}
}

func (s *AuthService) Login(ctx context.Context, email string, password string) (*TokenPair, error) {
	user, err := s.store.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	return s.issueTokenPair(ctx, user)
}

func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	claims, err := s.parseRefreshToken(refreshToken)
	if err != nil {
		return nil, ErrInvalidToken
	}

	tokenIDClaim, ok := claims["jti"].(string)
	if !ok || tokenIDClaim == "" {
		return nil, ErrInvalidToken
	}

	tokenRecord, err := s.store.GetRefreshToken(ctx, tokenIDClaim)
	if err != nil {
		return nil, ErrInvalidToken
	}

	if tokenRecord.RevokedAt != nil || time.Now().After(tokenRecord.ExpiresAt) {
		return nil, ErrInvalidToken
	}

	if err := s.store.RevokeRefreshToken(ctx, tokenIDClaim); err != nil {
		return nil, err
	}

	emailClaim, _ := claims["email"].(string)
	roleClaim, _ := claims["role"].(string)
	subClaim, _ := claims["sub"].(string)

	userID, err := parseInt64(subClaim)
	if err != nil {
		return nil, ErrInvalidToken
	}

	return s.issueTokenPair(ctx, &AuthUser{
		ID:    userID,
		Email: emailClaim,
		Role:  roleClaim,
	})
}

func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	claims, err := s.parseRefreshToken(refreshToken)
	if err != nil {
		return ErrInvalidToken
	}
	tokenIDClaim, ok := claims["jti"].(string)
	if !ok || tokenIDClaim == "" {
		return ErrInvalidToken
	}
	return s.store.RevokeRefreshToken(ctx, tokenIDClaim)
}

func (s *AuthService) issueTokenPair(ctx context.Context, user *AuthUser) (*TokenPair, error) {
	now := time.Now()
	accessExpiresAt := now.Add(s.config.AccessTTL)
	refreshExpiresAt := now.Add(s.config.RefreshTokenTTL)

	tokenID, err := randomTokenID()
	if err != nil {
		return nil, err
	}

	accessToken, err := s.signAccessToken(user, accessExpiresAt)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.signRefreshToken(user, tokenID, refreshExpiresAt)
	if err != nil {
		return nil, err
	}

	if err := s.store.SaveRefreshToken(ctx, tokenID, user.ID, refreshExpiresAt); err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(s.config.AccessTTL.Seconds()),
	}, nil
}

func (s *AuthService) signAccessToken(user *AuthUser, expiresAt time.Time) (string, error) {
	claims := jwt.MapClaims{
		"sub":   fmt.Sprintf("%d", user.ID),
		"email": user.Email,
		"role":  user.Role,
		"exp":   expiresAt.Unix(),
		"iat":   time.Now().Unix(),
		"typ":   "access",
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.config.AccessSecret))
}

func (s *AuthService) signRefreshToken(user *AuthUser, tokenID string, expiresAt time.Time) (string, error) {
	claims := jwt.MapClaims{
		"sub":   fmt.Sprintf("%d", user.ID),
		"email": user.Email,
		"role":  user.Role,
		"jti":   tokenID,
		"exp":   expiresAt.Unix(),
		"iat":   time.Now().Unix(),
		"typ":   "refresh",
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.config.RefreshSecret))
}

func (s *AuthService) parseRefreshToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, ErrInvalidToken
		}
		return []byte(s.config.RefreshSecret), nil
	})
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrInvalidToken
	}
	if claims["typ"] != "refresh" {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

func (s *AuthService) ParseAccessToken(tokenString string) (*AccessClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, ErrInvalidToken
		}
		return []byte(s.config.AccessSecret), nil
	})
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrInvalidToken
	}
	if claims["typ"] != "access" {
		return nil, ErrInvalidToken
	}

	subClaim, _ := claims["sub"].(string)
	userID, err := parseInt64(subClaim)
	if err != nil {
		return nil, ErrInvalidToken
	}

	emailClaim, _ := claims["email"].(string)
	roleClaim, _ := claims["role"].(string)

	return &AccessClaims{
		UserID: userID,
		Email:  emailClaim,
		Role:   roleClaim,
	}, nil
}

func randomTokenID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func parseInt64(value string) (int64, error) {
	var result int64
	_, err := fmt.Sscanf(value, "%d", &result)
	return result, err
}
