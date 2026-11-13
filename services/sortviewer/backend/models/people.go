package models

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"goklippy/services/sortviewer/backend/logic"
)

// Viewers, sessions and API keys.

func (s *Store) CreateViewer(ctx context.Context, email, name, passwordHash string) (int64, error) {
	res, err := s.db.ExecContext(ctx, `INSERT INTO viewers (email, name, password_hash) VALUES (?, ?, ?)`,
		email, name, passwordHash)
	if isUnique(err) {
		return 0, logic.ErrEmailTaken
	}
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ViewerLogin is a viewer's id and password hash by email (any case).
func (s *Store) ViewerLogin(ctx context.Context, email string) (int64, string, error) {
	var row struct {
		ID   int64  `db:"id"`
		Hash string `db:"password_hash"`
	}
	err := s.db.GetContext(ctx, &row, `SELECT id, password_hash FROM viewers WHERE email = ?`, email)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, "", logic.ErrNotFound
	}
	return row.ID, row.Hash, err
}

func (s *Store) ListViewers(ctx context.Context) ([]logic.Viewer, error) {
	var out []logic.Viewer
	if err := s.db.SelectContext(ctx, &out,
		`SELECT id, email, name, created_at FROM viewers ORDER BY email`); err != nil {
		return nil, err
	}
	var links []struct {
		ViewerID     int64 `db:"viewer_id"`
		CollectionID int64 `db:"collection_id"`
	}
	if err := s.db.SelectContext(ctx, &links,
		`SELECT viewer_id, collection_id FROM viewer_collections ORDER BY collection_id`); err != nil {
		return nil, err
	}
	byViewer := map[int64][]int64{}
	for _, l := range links {
		byViewer[l.ViewerID] = append(byViewer[l.ViewerID], l.CollectionID)
	}
	for i := range out {
		out[i].Collections = byViewer[out[i].ID]
		if out[i].Collections == nil {
			out[i].Collections = []int64{}
		}
	}
	if out == nil {
		out = []logic.Viewer{}
	}
	return out, nil
}

// UpdateViewer changes a viewer's name and, when passwordHash isn't "", password.
func (s *Store) UpdateViewer(ctx context.Context, id int64, name, passwordHash string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE viewers SET name = ?, password_hash = CASE WHEN ? = '' THEN password_hash ELSE ? END WHERE id = ?`,
		name, passwordHash, passwordHash, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return logic.ErrNotFound
	}
	if passwordHash != "" {
		// A new password signs them out everywhere.
		_, err = s.db.ExecContext(ctx, `DELETE FROM sessions WHERE viewer_id = ?`, id)
	}
	return err
}

// SetViewerCollections replaces the collections a viewer may see.
func (s *Store) SetViewerCollections(ctx context.Context, id int64, collections []int64) error {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var n int
	if err := tx.GetContext(ctx, &n, `SELECT COUNT(*) FROM viewers WHERE id = ?`, id); err != nil {
		return err
	}
	if n == 0 {
		return logic.ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM viewer_collections WHERE viewer_id = ?`, id); err != nil {
		return err
	}
	for _, c := range collections {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO viewer_collections (viewer_id, collection_id) SELECT ?, id FROM collections WHERE id = ?`, id, c); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) DeleteViewer(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM viewers WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return logic.ErrNotFound
	}
	return nil
}

func (s *Store) CreateSession(ctx context.Context, tokenHash string, sess logic.Session) error {
	var viewer any
	if sess.ViewerID != 0 {
		viewer = sess.ViewerID
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO sessions (token_hash, viewer_id, admin_email, admin_name, oidc_sid, expires_at)
VALUES (?, ?, ?, ?, ?, ?)`, tokenHash, viewer, sess.AdminEmail, sess.AdminName, sess.OIDCSid, sess.ExpiresAt.UTC())
	return err
}

// GetSession is an unexpired session by its token's hash.
func (s *Store) GetSession(ctx context.Context, tokenHash string, now time.Time) (logic.Session, error) {
	var row struct {
		ViewerID   sql.NullInt64  `db:"viewer_id"`
		ViewerName sql.NullString `db:"viewer_name"`
		AdminEmail string         `db:"admin_email"`
		AdminName  string         `db:"admin_name"`
		OIDCSid    string         `db:"oidc_sid"`
		ExpiresAt  time.Time      `db:"expires_at"`
	}
	err := s.db.GetContext(ctx, &row, `
SELECT s.viewer_id, COALESCE(NULLIF(v.name, ''), v.email) AS viewer_name, s.admin_email, s.admin_name,
  s.oidc_sid, s.expires_at
FROM sessions s LEFT JOIN viewers v ON v.id = s.viewer_id
WHERE s.token_hash = ? AND s.expires_at > ?`, tokenHash, now.UTC())
	if errors.Is(err, sql.ErrNoRows) {
		return logic.Session{}, logic.ErrNotFound
	}
	if err != nil {
		return logic.Session{}, err
	}
	return logic.Session{ViewerID: row.ViewerID.Int64, ViewerName: row.ViewerName.String,
		AdminEmail: row.AdminEmail, AdminName: row.AdminName, OIDCSid: row.OIDCSid, ExpiresAt: row.ExpiresAt}, nil
}

func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}

// DeleteAdminSessions ends the admin sessions from one Gangway portal session (sid), or
// all of an admin's when sid is "".
func (s *Store) DeleteAdminSessions(ctx context.Context, email, sid string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM sessions WHERE admin_email != '' AND (oidc_sid = ? AND ? != '' OR ? = '' AND admin_email = ?)`,
		sid, sid, sid, email)
	return err
}

func (s *Store) DeleteExpiredSessions(ctx context.Context, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, now.UTC())
	return err
}

func (s *Store) CreateAPIKey(ctx context.Context, name, keyHash, prefix string) (int64, error) {
	res, err := s.db.ExecContext(ctx, `INSERT INTO api_keys (name, key_hash, prefix) VALUES (?, ?, ?)`,
		name, keyHash, prefix)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UseAPIKey checks a key's hash is an unrevoked key and notes it was used.
func (s *Store) UseAPIKey(ctx context.Context, keyHash string, now time.Time) (bool, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE api_keys SET last_used_at = ? WHERE key_hash = ? AND revoked_at IS NULL`, now.UTC(), keyHash)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (s *Store) ListAPIKeys(ctx context.Context) ([]logic.APIKey, error) {
	var rows []struct {
		ID         int64        `db:"id"`
		Name       string       `db:"name"`
		Prefix     string       `db:"prefix"`
		CreatedAt  time.Time    `db:"created_at"`
		LastUsedAt sql.NullTime `db:"last_used_at"`
		RevokedAt  sql.NullTime `db:"revoked_at"`
	}
	if err := s.db.SelectContext(ctx, &rows,
		`SELECT id, name, prefix, created_at, last_used_at, revoked_at FROM api_keys ORDER BY id DESC`); err != nil {
		return nil, err
	}
	out := make([]logic.APIKey, 0, len(rows))
	for _, r := range rows {
		k := logic.APIKey{ID: r.ID, Name: r.Name, Prefix: r.Prefix, CreatedAt: r.CreatedAt}
		if r.LastUsedAt.Valid {
			t := r.LastUsedAt.Time
			k.LastUsedAt = &t
		}
		if r.RevokedAt.Valid {
			t := r.RevokedAt.Time
			k.RevokedAt = &t
		}
		out = append(out, k)
	}
	return out, nil
}

func (s *Store) RevokeAPIKey(ctx context.Context, id int64, now time.Time) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE api_keys SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL`, now.UTC(), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return logic.ErrNotFound
	}
	return nil
}
