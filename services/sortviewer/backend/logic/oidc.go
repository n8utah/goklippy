package logic

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

var (
	ErrNoOIDC      = errors.New("admin sign-in isn't set up (OIDC settings are missing)")
	ErrNotAdmin    = errors.New("that account isn't an admin of this site")
	ErrLoginExpired = errors.New("the sign-in took too long or was already used; try again")
)

// OIDCConfig is how the admin signs in through Harbor Keeper's Gangway.
type OIDCConfig struct {
	Issuer       string // https://login.n8p.xyz
	ClientID     string
	ClientSecret string
	RedirectURL  string // <app>/auth/callback
	// AdminEmails may be admin; empty lets no one in.
	AdminEmails []string
}

func (c OIDCConfig) Enabled() bool {
	return c.Issuer != "" && c.ClientID != "" && c.RedirectURL != ""
}

// AdminSignIn runs the OIDC code flow with PKCE. The provider is looked up on first
// use, so the site starts even when the Gangway can't be reached.
type AdminSignIn struct {
	cfg OIDCConfig

	mu       sync.Mutex
	provider *oidc.Provider
	pending  map[string]pendingLogin // by state
	now      func() time.Time
}

type pendingLogin struct {
	verifier string
	nonce    string
	expires  time.Time
}

const loginPendingTTL = 10 * time.Minute

func NewAdminSignIn(cfg OIDCConfig) *AdminSignIn {
	return &AdminSignIn{cfg: cfg, pending: map[string]pendingLogin{}, now: time.Now}
}

func (a *AdminSignIn) Enabled() bool { return a.cfg.Enabled() }

func (a *AdminSignIn) setup(ctx context.Context) (*oidc.Provider, *oauth2.Config, error) {
	if !a.cfg.Enabled() {
		return nil, nil, ErrNoOIDC
	}
	a.mu.Lock()
	p := a.provider
	a.mu.Unlock()
	if p == nil {
		var err error
		if p, err = oidc.NewProvider(ctx, a.cfg.Issuer); err != nil {
			return nil, nil, fmt.Errorf("reach the Gangway: %w", err)
		}
		a.mu.Lock()
		a.provider = p
		a.mu.Unlock()
	}
	return p, &oauth2.Config{
		ClientID: a.cfg.ClientID, ClientSecret: a.cfg.ClientSecret, RedirectURL: a.cfg.RedirectURL,
		Endpoint: p.Endpoint(), Scopes: []string{oidc.ScopeOpenID, "email", "profile"},
	}, nil
}

// Begin returns the Gangway URL to send the browser to.
func (a *AdminSignIn) Begin(ctx context.Context) (string, error) {
	_, conf, err := a.setup(ctx)
	if err != nil {
		return "", err
	}
	state, nonce, verifier := RandomToken(), RandomToken(), oauth2.GenerateVerifier()
	now := a.now()
	a.mu.Lock()
	for k, v := range a.pending {
		if now.After(v.expires) {
			delete(a.pending, k)
		}
	}
	a.pending[state] = pendingLogin{verifier: verifier, nonce: nonce, expires: now.Add(loginPendingTTL)}
	a.mu.Unlock()
	return conf.AuthCodeURL(state, oidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier)), nil
}

// AdminIdentity is who the Gangway signed in.
type AdminIdentity struct {
	Email string
	Name  string
	Sid   string // the portal session, for back-channel logout
}

// Finish trades the callback's code for the user and checks they may be admin.
func (a *AdminSignIn) Finish(ctx context.Context, state, code string) (AdminIdentity, error) {
	p, conf, err := a.setup(ctx)
	if err != nil {
		return AdminIdentity{}, err
	}
	a.mu.Lock()
	pend, ok := a.pending[state]
	delete(a.pending, state)
	a.mu.Unlock()
	if !ok || a.now().After(pend.expires) {
		return AdminIdentity{}, ErrLoginExpired
	}
	tok, err := conf.Exchange(ctx, code, oauth2.VerifierOption(pend.verifier))
	if err != nil {
		return AdminIdentity{}, fmt.Errorf("finish signing in: %w", err)
	}
	raw, _ := tok.Extra("id_token").(string)
	idTok, err := p.Verifier(&oidc.Config{ClientID: a.cfg.ClientID}).Verify(ctx, raw)
	if err != nil {
		return AdminIdentity{}, fmt.Errorf("check the sign-in: %w", err)
	}
	if idTok.Nonce != pend.nonce {
		return AdminIdentity{}, ErrLoginExpired
	}
	var claims struct {
		Email    string `json:"email"`
		Verified bool   `json:"email_verified"`
		Name     string `json:"name"`
		Sid      string `json:"sid"`
	}
	if err := idTok.Claims(&claims); err != nil {
		return AdminIdentity{}, err
	}
	if !a.isAdmin(claims.Email) {
		return AdminIdentity{}, ErrNotAdmin
	}
	return AdminIdentity{Email: strings.ToLower(claims.Email), Name: claims.Name, Sid: claims.Sid}, nil
}

func (a *AdminSignIn) isAdmin(email string) bool {
	for _, e := range a.cfg.AdminEmails {
		if email != "" && strings.EqualFold(strings.TrimSpace(e), email) {
			return true
		}
	}
	return false
}

// LogoutURL ends the Gangway portal session too, then comes back to back.
func (a *AdminSignIn) LogoutURL(ctx context.Context, back string) string {
	p, _, err := a.setup(ctx)
	if err != nil {
		return back
	}
	var meta struct {
		EndSession string `json:"end_session_endpoint"`
	}
	if p.Claims(&meta) != nil || meta.EndSession == "" {
		return back
	}
	q := url.Values{"client_id": {a.cfg.ClientID}, "post_logout_redirect_uri": {back}}
	return meta.EndSession + "?" + q.Encode()
}

// BackchannelLogout checks a logout token the Gangway posted and returns whose
// sessions to end: a portal session (sid) or, without one, a user (email).
func (a *AdminSignIn) BackchannelLogout(ctx context.Context, raw string) (email, sid string, err error) {
	p, _, err := a.setup(ctx)
	if err != nil {
		return "", "", err
	}
	tok, err := p.Verifier(&oidc.Config{ClientID: a.cfg.ClientID}).Verify(ctx, raw)
	if err != nil {
		return "", "", fmt.Errorf("%w: %v", ErrBadInput, err)
	}
	var claims struct {
		Sid    string         `json:"sid"`
		Email  string         `json:"email"`
		Events map[string]any `json:"events"`
	}
	if err := tok.Claims(&claims); err != nil {
		return "", "", err
	}
	if _, ok := claims.Events["http://schemas.openid.net/event/backchannel-logout"]; !ok {
		return "", "", fmt.Errorf("%w: not a logout token", ErrBadInput)
	}
	if claims.Sid == "" && claims.Email == "" {
		return "", "", fmt.Errorf("%w: the logout token names no session", ErrBadInput)
	}
	return strings.ToLower(claims.Email), claims.Sid, nil
}
