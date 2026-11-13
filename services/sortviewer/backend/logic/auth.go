package logic

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// SessionTTL is how long a sign-in lasts.
const SessionTTL = 30 * 24 * time.Hour

// MinPasswordLen is the shortest viewer password accepted.
const MinPasswordLen = 8

// RandomToken is 32 random bytes, URL-safe.
func RandomToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// HashToken is how session tokens and API keys are kept: SHA-256, hex.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// Login signs a viewer in with email and password and returns the session token.
func (s *Service) Login(ctx context.Context, email, password, client string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if !s.logins.allow(email, client, s.now()) {
		return "", ErrTooManyLogins
	}
	id, hash, err := s.store.ViewerLogin(ctx, email)
	if errors.Is(err, ErrNotFound) {
		// Spend the same time as a real check, so a missing email doesn't stand out.
		_ = bcrypt.CompareHashAndPassword(dummyHash(), []byte(password))
		return "", ErrBadLogin
	}
	if err != nil {
		return "", err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return "", ErrBadLogin
	}
	s.logins.reset(email, client)
	return s.newSession(ctx, Session{ViewerID: id})
}

// dummyHash is a real bcrypt hash to check against when the email isn't known.
var dummyHash = sync.OnceValue(func() []byte {
	h, _ := bcrypt.GenerateFromPassword([]byte(RandomToken()), bcrypt.DefaultCost)
	return h
})

// AdminLogin opens an admin session for someone the Gangway signed in.
func (s *Service) AdminLogin(ctx context.Context, email, name, sid string) (string, error) {
	return s.newSession(ctx, Session{AdminEmail: email, AdminName: name, OIDCSid: sid})
}

func (s *Service) newSession(ctx context.Context, sess Session) (string, error) {
	token := RandomToken()
	sess.ExpiresAt = s.now().Add(SessionTTL)
	if err := s.store.CreateSession(ctx, HashToken(token), sess); err != nil {
		return "", err
	}
	return token, nil
}

// Session is the session a cookie's token belongs to, or ErrNotFound.
func (s *Service) Session(ctx context.Context, token string) (Session, error) {
	if token == "" {
		return Session{}, ErrNotFound
	}
	return s.store.GetSession(ctx, HashToken(token), s.now())
}

func (s *Service) Logout(ctx context.Context, token string) error {
	return s.store.DeleteSession(ctx, HashToken(token))
}

// EndAdminSessions ends the admin sessions of a Gangway portal session (back-channel logout).
func (s *Service) EndAdminSessions(ctx context.Context, email, sid string) error {
	return s.store.DeleteAdminSessions(ctx, email, sid)
}

// CheckAPIKey reports whether key is a live push key.
func (s *Service) CheckAPIKey(ctx context.Context, key string) (bool, error) {
	if !strings.HasPrefix(key, apiKeyPrefix) {
		return false, nil
	}
	return s.store.UseAPIKey(ctx, HashToken(key), s.now())
}

const apiKeyPrefix = "sv_"

// loginLimiter allows a few wrong passwords per email and per client a minute.
type loginLimiter struct {
	mu    sync.Mutex
	tries map[string][]time.Time
}

const (
	loginTries  = 5
	loginWindow = time.Minute
)

func newLoginLimiter() *loginLimiter { return &loginLimiter{tries: map[string][]time.Time{}} }

func (l *loginLimiter) allow(email, client string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	ok := true
	for _, k := range []string{"e:" + email, "c:" + client} {
		var kept []time.Time
		for _, t := range l.tries[k] {
			if now.Sub(t) < loginWindow {
				kept = append(kept, t)
			}
		}
		if len(kept) >= loginTries {
			ok = false
		}
		l.tries[k] = append(kept, now)
	}
	return ok
}

func (l *loginLimiter) reset(email, client string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.tries, "e:"+email)
	delete(l.tries, "c:"+client)
}

// Admin: viewers and keys.

// ViewerInput is what the admin sets on a viewer. Password "" keeps the old one.
type ViewerInput struct {
	Email       string  `json:"email"`
	Name        string  `json:"name"`
	Password    string  `json:"password"`
	Collections []int64 `json:"collections"`
}

func (s *Service) Viewers(ctx context.Context) ([]Viewer, error) { return s.store.ListViewers(ctx) }

func (s *Service) CreateViewer(ctx context.Context, in ViewerInput) (int64, error) {
	addr, err := mail.ParseAddress(strings.TrimSpace(in.Email))
	if err != nil {
		return 0, fmt.Errorf("%w: that isn't an email address", ErrBadInput)
	}
	if len(in.Password) < MinPasswordLen {
		return 0, fmt.Errorf("%w: the password needs at least %d characters", ErrBadInput, MinPasswordLen)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return 0, err
	}
	id, err := s.store.CreateViewer(ctx, strings.ToLower(addr.Address), strings.TrimSpace(in.Name), string(hash))
	if err != nil {
		return 0, err
	}
	return id, s.store.SetViewerCollections(ctx, id, in.Collections)
}

func (s *Service) UpdateViewer(ctx context.Context, id int64, in ViewerInput) error {
	hash := ""
	if in.Password != "" {
		if len(in.Password) < MinPasswordLen {
			return fmt.Errorf("%w: the password needs at least %d characters", ErrBadInput, MinPasswordLen)
		}
		h, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		hash = string(h)
	}
	if err := s.store.UpdateViewer(ctx, id, strings.TrimSpace(in.Name), hash); err != nil {
		return err
	}
	return s.store.SetViewerCollections(ctx, id, in.Collections)
}

func (s *Service) DeleteViewer(ctx context.Context, id int64) error {
	return s.store.DeleteViewer(ctx, id)
}

func (s *Service) APIKeys(ctx context.Context) ([]APIKey, error) { return s.store.ListAPIKeys(ctx) }

// CreateAPIKey makes a push key and returns it; only its hash is kept, so this is the
// one time it's seen.
func (s *Service) CreateAPIKey(ctx context.Context, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("%w: give the key a name, like the machine it's for", ErrBadInput)
	}
	key := apiKeyPrefix + RandomToken()
	if _, err := s.store.CreateAPIKey(ctx, name, HashToken(key), key[:len(apiKeyPrefix)+6]); err != nil {
		return "", err
	}
	return key, nil
}

func (s *Service) RevokeAPIKey(ctx context.Context, id int64) error {
	return s.store.RevokeAPIKey(ctx, id, s.now())
}

// Admin: clearing out.

// ClearPhotos removes a collection's photos (every collection's for id 0), keeping
// the pieces.
func (s *Service) ClearPhotos(ctx context.Context, collectionID int64) (int, error) {
	photos, err := s.store.ClearPhotos(ctx, collectionID)
	if err != nil {
		return 0, err
	}
	s.removePhotos(photos)
	if collectionID != 0 {
		s.Events.Publish(Event{Type: "photos", CollectionID: collectionID})
	}
	return len(photos), nil
}

// DeleteCollection removes a collection, its pieces and their photos. If the sorter
// shares it again, it comes back from then on.
func (s *Service) DeleteCollection(ctx context.Context, id int64) error {
	photos, err := s.store.DeleteCollection(ctx, id)
	if err != nil {
		return err
	}
	s.removePhotos(photos)
	return nil
}
