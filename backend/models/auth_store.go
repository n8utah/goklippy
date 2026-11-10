package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"goklippy/backend/logic"

	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"
)

type InitialUserSeed struct {
	Enabled  bool
	Email    string
	Password string
	Role     string
}

type AuthStore struct {
	db *sqlx.DB
}

func NewAuthStore(db *sqlx.DB) *AuthStore {
	return &AuthStore{db: db}
}

func (s *AuthStore) GetUserByEmail(ctx context.Context, email string) (*logic.AuthUser, error) {
	if s.db == nil {
		return nil, ErrDBNotConfigured
	}

	const query = `
SELECT
  id,
  email,
  COALESCE(role, 'user') AS role,
  password_hash
FROM app_users
WHERE LOWER(email) = LOWER(?)
  AND is_active = TRUE
LIMIT 1
`

	var user logic.AuthUser
	if err := s.db.GetContext(ctx, &user, query, email); err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *AuthStore) SaveRefreshToken(ctx context.Context, tokenID string, userID int64, expiresAt time.Time) error {
	if s.db == nil {
		return ErrDBNotConfigured
	}

	const selectQuery = `
SELECT token_id
FROM auth_refresh_tokens
WHERE token_id = ?
LIMIT 1
`

	var existingTokenID string
	err := s.db.GetContext(ctx, &existingTokenID, selectQuery, tokenID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	if errors.Is(err, sql.ErrNoRows) {
		const insertQuery = `
INSERT INTO auth_refresh_tokens (
  token_id,
  user_id,
  expires_at,
  created_at,
  updated_at,
  revoked_at
)
VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, NULL)
`
		_, execErr := s.db.ExecContext(ctx, insertQuery, tokenID, userID, expiresAt)
		return execErr
	}

	const updateQuery = `
UPDATE auth_refresh_tokens
SET user_id = ?,
    expires_at = ?,
    revoked_at = NULL,
    updated_at = CURRENT_TIMESTAMP
WHERE token_id = ?
`
	_, execErr := s.db.ExecContext(ctx, updateQuery, userID, expiresAt, tokenID)
	return execErr
}

func (s *AuthStore) GetRefreshToken(ctx context.Context, tokenID string) (*logic.RefreshTokenRecord, error) {
	if s.db == nil {
		return nil, ErrDBNotConfigured
	}

	const query = `
SELECT
  token_id,
  user_id,
  expires_at,
  revoked_at
FROM auth_refresh_tokens
WHERE token_id = ?
LIMIT 1
`

	type refreshTokenRow struct {
		TokenID   string       `db:"token_id"`
		UserID    int64        `db:"user_id"`
		ExpiresAt time.Time    `db:"expires_at"`
		RevokedAt sql.NullTime `db:"revoked_at"`
	}

	var row refreshTokenRow
	if err := s.db.GetContext(ctx, &row, query, tokenID); err != nil {
		return nil, err
	}

	var revokedAt *time.Time
	if row.RevokedAt.Valid {
		revokedAt = &row.RevokedAt.Time
	}

	return &logic.RefreshTokenRecord{
		TokenID:   row.TokenID,
		UserID:    row.UserID,
		ExpiresAt: row.ExpiresAt,
		RevokedAt: revokedAt,
	}, nil
}

func (s *AuthStore) RevokeRefreshToken(ctx context.Context, tokenID string) error {
	if s.db == nil {
		return ErrDBNotConfigured
	}

	const query = `
UPDATE auth_refresh_tokens
SET revoked_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE token_id = ?
`
	_, err := s.db.ExecContext(ctx, query, tokenID)
	return err
}

func EnsureInitialUser(ctx context.Context, db *sqlx.DB, seed InitialUserSeed) (bool, error) {
	if db == nil || !seed.Enabled {
		return false, nil
	}

	email := strings.TrimSpace(seed.Email)
	password := seed.Password
	role := strings.TrimSpace(seed.Role)
	if role == "" {
		role = "admin"
	}

	if email == "" || password == "" {
		return false, nil
	}

	type existingUser struct {
		ID           int64  `db:"id"`
		PasswordHash string `db:"password_hash"`
		Role         string `db:"role"`
		IsActive     bool   `db:"is_active"`
	}

	const existingUserQuery = `
SELECT id, password_hash, COALESCE(role, 'user') AS role, is_active
FROM app_users
WHERE LOWER(email) = LOWER(?)
LIMIT 1
`

	var user existingUser
	err := db.GetContext(ctx, &user, existingUserQuery, email)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("lookup initial user: %w", err)
	}

	passwordMatches := false
	if err == nil {
		passwordMatches = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) == nil
	}

	if err == nil && passwordMatches && user.Role == role && user.IsActive {
		return false, nil
	}

	passwordHash, hashErr := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if hashErr != nil {
		return false, fmt.Errorf("hash initial user password: %w", hashErr)
	}

	if errors.Is(err, sql.ErrNoRows) {
		const insertQuery = `
INSERT INTO app_users (email, password_hash, role, is_active, created_at, updated_at)
VALUES (?, ?, ?, TRUE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
`

		if _, execErr := db.ExecContext(ctx, insertQuery, email, string(passwordHash), role); execErr != nil {
			return false, fmt.Errorf("insert initial user: %w", execErr)
		}
		return true, nil
	}

	const updateQuery = `
UPDATE app_users
SET password_hash = ?,
    role = ?,
    is_active = TRUE,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?
`

	if _, execErr := db.ExecContext(ctx, updateQuery, string(passwordHash), role, user.ID); execErr != nil {
		return false, fmt.Errorf("update initial user: %w", execErr)
	}

	return true, nil
}
