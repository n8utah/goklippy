package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"goklippy/backend/logic"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type authRouteStore struct {
	user      *logic.AuthUser
	refreshes map[string]*logic.RefreshTokenRecord
}

func newAuthRouteStore(user *logic.AuthUser) *authRouteStore {
	return &authRouteStore{user: user, refreshes: map[string]*logic.RefreshTokenRecord{}}
}

func (s *authRouteStore) GetUserByEmail(_ context.Context, _ string) (*logic.AuthUser, error) {
	if s.user == nil {
		return nil, logic.ErrInvalidCredentials
	}
	return s.user, nil
}

func (s *authRouteStore) SaveRefreshToken(_ context.Context, tokenID string, userID int64, expiresAt time.Time) error {
	s.refreshes[tokenID] = &logic.RefreshTokenRecord{TokenID: tokenID, UserID: userID, ExpiresAt: expiresAt}
	return nil
}

func (s *authRouteStore) GetRefreshToken(_ context.Context, tokenID string) (*logic.RefreshTokenRecord, error) {
	token, ok := s.refreshes[tokenID]
	if !ok {
		return nil, logic.ErrInvalidToken
	}
	return token, nil
}

func (s *authRouteStore) RevokeRefreshToken(_ context.Context, tokenID string) error {
	if token, ok := s.refreshes[tokenID]; ok {
		now := time.Now()
		token.RevokedAt = &now
	}
	return nil
}

func TestLoginRouteSuccess(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("secret123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash generation failed: %v", err)
	}

	service := logic.NewAuthService(newAuthRouteStore(&logic.AuthUser{
		ID:           4,
		Email:        "user@example.com",
		Role:         "user",
		PasswordHash: string(passwordHash),
	}), logic.AuthConfig{
		AccessSecret:    "access-secret",
		RefreshSecret:   "refresh-secret",
		AccessTTL:       15 * time.Minute,
		RefreshTokenTTL: 24 * time.Hour,
	})

	gin.SetMode(gin.TestMode)
	r := gin.New()
	InitAuthService(service)
	api := r.Group("/api/v1")
	api.POST("/auth/login", LoginHandler())
	api.POST("/auth/refresh", RefreshHandler())
	api.POST("/auth/logout", LogoutHandler())

	body := `{"email":"user@example.com","password":"secret123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
}

func TestLoginRouteBadBody(t *testing.T) {
	service := logic.NewAuthService(newAuthRouteStore(nil), logic.AuthConfig{
		AccessSecret:    "access-secret",
		RefreshSecret:   "refresh-secret",
		AccessTTL:       15 * time.Minute,
		RefreshTokenTTL: 24 * time.Hour,
	})

	gin.SetMode(gin.TestMode)
	r := gin.New()
	InitAuthService(service)
	api := r.Group("/api/v1")
	api.POST("/auth/login", LoginHandler())
	api.POST("/auth/refresh", RefreshHandler())
	api.POST("/auth/logout", LogoutHandler())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader("{"))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
}
