package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"goklippy/backend/logic"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type testRouteStore struct {
	user *logic.AuthUser
}

func (s testRouteStore) GetUserByEmail(_ context.Context, _ string) (*logic.AuthUser, error) {
	if s.user == nil {
		return nil, logic.ErrInvalidCredentials
	}
	return s.user, nil
}

func (testRouteStore) SaveRefreshToken(_ context.Context, _ string, _ int64, _ time.Time) error {
	return nil
}

func (testRouteStore) GetRefreshToken(_ context.Context, _ string) (*logic.RefreshTokenRecord, error) {
	return nil, logic.ErrInvalidToken
}
func (testRouteStore) RevokeRefreshToken(_ context.Context, _ string) error { return nil }

func TestMeRouteRequiresAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	service := logic.NewAuthService(testRouteStore{}, logic.AuthConfig{
		AccessSecret:    "access-secret",
		RefreshSecret:   "refresh-secret",
		AccessTTL:       15 * time.Minute,
		RefreshTokenTTL: 24 * time.Hour,
	})
	InitAuthService(service)

	api := r.Group("/api/v1")
	api.GET("/auth/me", RequireJWT(), Me)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.Code)
	}
}

func TestMeRouteReturnsClaimsWithValidAccessToken(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("secret123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to build hash: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	service := logic.NewAuthService(testRouteStore{user: &logic.AuthUser{
		ID:           9,
		Email:        "valid@example.com",
		Role:         "user",
		PasswordHash: string(passwordHash),
	}}, logic.AuthConfig{
		AccessSecret:    "access-secret",
		RefreshSecret:   "refresh-secret",
		AccessTTL:       15 * time.Minute,
		RefreshTokenTTL: 24 * time.Hour,
	})
	InitAuthService(service)

	pair, err := service.Login(context.Background(), "valid@example.com", "secret123")
	if err != nil {
		t.Fatalf("failed to login: %v", err)
	}

	api := r.Group("/api/v1")
	api.GET("/auth/me", RequireJWT(), Me)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
}
