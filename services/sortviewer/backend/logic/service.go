package logic

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Store is the database (models.Store).
type Store interface {
	ListCollections(ctx context.Context, viewerID int64) ([]Collection, error)
	GetCollection(ctx context.Context, id int64) (Collection, error)
	CanSee(ctx context.Context, viewerID, collectionID int64) (bool, error)
	UpsertCollection(ctx context.Context, id int64, name string, plan json.RawMessage, at time.Time) error
	EnsureCollection(ctx context.Context, id int64, at time.Time) error
	TouchCollection(ctx context.Context, id int64, at time.Time) error
	DeleteCollection(ctx context.Context, id int64) ([]string, error)
	UpsertRun(ctx context.Context, id int64, r PushRun) error
	ListRuns(ctx context.Context, collectionID int64) ([]Run, error)

	UpsertPiece(ctx context.Context, id string, p PushPiece, at time.Time) error
	GetPiece(ctx context.Context, id string) (Piece, string, error)
	SetPiecePhoto(ctx context.Context, id, photo string) (string, error)
	ListPieces(ctx context.Context, collectionID int64, f PieceFilter) ([]Piece, int, error)
	PartCounts(ctx context.Context, collectionID int64) ([]PartCount, error)
	BinCounts(ctx context.Context, collectionID int64, since *time.Time) ([]BinCount, error)
	ClearUnknowns(ctx context.Context, collectionID int64) (int, []string, error)
	ClearPhotos(ctx context.Context, collectionID int64) ([]string, error)

	CreateViewer(ctx context.Context, email, name, passwordHash string) (int64, error)
	ViewerLogin(ctx context.Context, email string) (int64, string, error)
	ListViewers(ctx context.Context) ([]Viewer, error)
	UpdateViewer(ctx context.Context, id int64, name, passwordHash string) error
	SetViewerCollections(ctx context.Context, id int64, collections []int64) error
	DeleteViewer(ctx context.Context, id int64) error

	CreateSession(ctx context.Context, tokenHash string, s Session) error
	GetSession(ctx context.Context, tokenHash string, now time.Time) (Session, error)
	DeleteSession(ctx context.Context, tokenHash string) error
	DeleteAdminSessions(ctx context.Context, email, sid string) error
	DeleteExpiredSessions(ctx context.Context, now time.Time) error

	CreateAPIKey(ctx context.Context, name, keyHash, prefix string) (int64, error)
	UseAPIKey(ctx context.Context, keyHash string, now time.Time) (bool, error)
	ListAPIKeys(ctx context.Context) ([]APIKey, error)
	RevokeAPIKey(ctx context.Context, id int64, now time.Time) error
}

// Service is the sort viewer: what the sorter pushes in, and what people see.
type Service struct {
	store   Store
	dataDir string
	Events  *Hub
	logins  *loginLimiter
	now     func() time.Time
}

// NewService keeps photos under dataDir/photos.
func NewService(store Store, dataDir string) *Service {
	return &Service{store: store, dataDir: dataDir, Events: NewHub(), logins: newLoginLimiter(), now: time.Now}
}

// PhotoPath is a stored photo's file, refusing paths outside the photos folder.
func (s *Service) PhotoPath(rel string) (string, bool) {
	if rel == "" || filepath.IsAbs(rel) || strings.Contains(rel, "..") {
		return "", false
	}
	return filepath.Join(s.dataDir, filepath.FromSlash(rel)), true
}

// removePhotos deletes photo files; a missing file is fine.
func (s *Service) removePhotos(photos []string) {
	for _, rel := range photos {
		if path, ok := s.PhotoPath(rel); ok {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				slog.Warn("removing a photo failed", "path", rel, "error", err)
			}
		}
	}
}

// CleanSessions drops expired sessions now and then until ctx ends.
func (s *Service) CleanSessions(ctx context.Context) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		if err := s.store.DeleteExpiredSessions(ctx, s.now()); err != nil && ctx.Err() == nil {
			slog.Warn("dropping expired sessions failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
