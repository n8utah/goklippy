package logic

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

type testAuthStore struct {
	usersByEmail      map[string]*AuthUser
	refreshTokens     map[string]*RefreshTokenRecord
	tokenToUser       map[string]int64
	revokedTokenIDs   []string
	saveRefreshCalled bool
}

func newTestAuthStore() *testAuthStore {
	return &testAuthStore{
		usersByEmail:  map[string]*AuthUser{},
		refreshTokens: map[string]*RefreshTokenRecord{},
		tokenToUser:   map[string]int64{},
	}
}

func (s *testAuthStore) GetUserByEmail(_ context.Context, email string) (*AuthUser, error) {
	user, ok := s.usersByEmail[email]
	if !ok {
		return nil, ErrInvalidCredentials
	}
	return user, nil
}

func (s *testAuthStore) SaveRefreshToken(_ context.Context, tokenID string, userID int64, expiresAt time.Time) error {
	s.saveRefreshCalled = true
	s.tokenToUser[tokenID] = userID
	s.refreshTokens[tokenID] = &RefreshTokenRecord{
		TokenID:   tokenID,
		UserID:    userID,
		ExpiresAt: expiresAt,
		RevokedAt: nil,
	}
	return nil
}

func (s *testAuthStore) GetRefreshToken(_ context.Context, tokenID string) (*RefreshTokenRecord, error) {
	token, ok := s.refreshTokens[tokenID]
	if !ok {
		return nil, ErrInvalidToken
	}
	return token, nil
}

func (s *testAuthStore) RevokeRefreshToken(_ context.Context, tokenID string) error {
	s.revokedTokenIDs = append(s.revokedTokenIDs, tokenID)
	if token, ok := s.refreshTokens[tokenID]; ok {
		now := time.Now()
		token.RevokedAt = &now
	}
	return nil
}

func TestLoginReturnsTokenPair(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("secret123"), bcrypt.DefaultCost)
	require.NoError(t, err)

	store := newTestAuthStore()
	store.usersByEmail["user@example.com"] = &AuthUser{
		ID:           7,
		Email:        "user@example.com",
		Role:         "user",
		PasswordHash: string(passwordHash),
	}

	service := NewAuthService(store, AuthConfig{
		AccessSecret:    "access-secret",
		RefreshSecret:   "refresh-secret",
		AccessTTL:       15 * time.Minute,
		RefreshTokenTTL: 24 * time.Hour,
	})

	pair, err := service.Login(context.Background(), "user@example.com", "secret123")
	require.NoError(t, err)
	assert.NotEmpty(t, pair.AccessToken)
	assert.NotEmpty(t, pair.RefreshToken)
	assert.True(t, store.saveRefreshCalled)
}

func TestRefreshRevokesOldTokenAndReturnsNewPair(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("secret123"), bcrypt.DefaultCost)
	require.NoError(t, err)

	store := newTestAuthStore()
	store.usersByEmail["user@example.com"] = &AuthUser{
		ID:           11,
		Email:        "user@example.com",
		Role:         "admin",
		PasswordHash: string(passwordHash),
	}

	service := NewAuthService(store, AuthConfig{
		AccessSecret:    "access-secret",
		RefreshSecret:   "refresh-secret",
		AccessTTL:       15 * time.Minute,
		RefreshTokenTTL: 24 * time.Hour,
	})

	firstPair, err := service.Login(context.Background(), "user@example.com", "secret123")
	require.NoError(t, err)

	_, err = service.Refresh(context.Background(), firstPair.RefreshToken)
	require.NoError(t, err)
	assert.Len(t, store.revokedTokenIDs, 1)
}

func TestParseAccessTokenRejectsRefreshToken(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("secret123"), bcrypt.DefaultCost)
	require.NoError(t, err)

	store := newTestAuthStore()
	store.usersByEmail["user@example.com"] = &AuthUser{
		ID:           3,
		Email:        "user@example.com",
		Role:         "user",
		PasswordHash: string(passwordHash),
	}

	service := NewAuthService(store, AuthConfig{
		AccessSecret:    "access-secret",
		RefreshSecret:   "refresh-secret",
		AccessTTL:       15 * time.Minute,
		RefreshTokenTTL: 24 * time.Hour,
	})

	pair, err := service.Login(context.Background(), "user@example.com", "secret123")
	require.NoError(t, err)

	_, err = service.ParseAccessToken(pair.RefreshToken)
	assert.Error(t, err)
}
