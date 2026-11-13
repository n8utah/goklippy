package sorter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/image/draw"
)

// Sharing sends shared collections to the sort viewer site (services/sortviewer, its
// README's "Push API"), so the people whose LEGO it is can watch.
//
// Changes don't send anything themselves: they mark the thing in an outbox
// (share_outbox), and one sender goroutine works through the marks in order, reading
// each thing as it is when sent. So many changes to a piece make one send, a backfill is
// marking everything, a restart or the site being down loses nothing, and sorting never
// waits on the site.

// What a mark is for.
const (
	ShareCollection    = "collection" // name and plan
	ShareRun           = "run"
	SharePiece         = "piece"
	SharePhoto         = "photo"
	ShareClearUnknowns = "clear_unknowns" // ref is the collection
)

// ErrNotShared: the thing a mark is for is gone; the mark is dropped.
var ErrNotShared = errors.New("no longer there to share")

var ErrSharingNotSetUp = errors.New("sharing isn't set up: give the site's address and key")

// ShareMark is one thing waiting to be sent.
type ShareMark struct {
	Seq      int64  `db:"seq"`
	Kind     string `db:"kind"`
	Ref      string `db:"ref"`
	Attempts int    `db:"attempts"`
}

// SharingSettings is where to send: the site and the key it gave this sorter.
type SharingSettings struct {
	URL    string `json:"url"`
	APIKey string `json:"api_key"`
}

// ShareStore keeps the outbox (models.ShareStore).
type ShareStore interface {
	MarkCollection(ctx context.Context, id int64) error
	MarkRun(ctx context.Context, id int64) error
	MarkPiece(ctx context.Context, id string) error
	MarkPhoto(ctx context.Context, id string) error
	MarkClearUnknowns(ctx context.Context, collectionID int64) error
	SetShared(ctx context.Context, collectionID int64, shared bool) error
	NextMarks(ctx context.Context, limit int) ([]ShareMark, error)
	DeleteMark(ctx context.Context, seq int64) error
	FailMark(ctx context.Context, seq int64, reason string) error
	CountMarks(ctx context.Context) (int, error)
	GetRun(ctx context.Context, id int64) (Run, error)
	PieceCollection(ctx context.Context, pieceID string) (int64, error)
	SharingSettings(ctx context.Context) (SharingSettings, error)
	SaveSharingSettings(ctx context.Context, s SharingSettings) error
}

// SharingStatus is how sending is going, for the Collections page.
type SharingStatus struct {
	URL       string     `json:"url"`
	KeySet    bool       `json:"key_set"`
	Waiting   int        `json:"waiting"`
	Connected bool       `json:"connected"` // the last send (or test) worked
	LastError string     `json:"last_error,omitempty"`
	LastSent  *time.Time `json:"last_sent,omitempty"`
}

// Sharing marks changes and sends them.
type Sharing struct {
	store ShareStore
	// Set by Attach before Run.
	records     RecordsStore
	collections *Collections
	images      func(rel string) (string, error)
	colorNames  func(ctx context.Context) (map[string]string, error)

	client *http.Client
	wake   chan struct{}

	mu        sync.Mutex
	connected bool
	lastError string
	lastSent  *time.Time
}

// Backoff after a failed send: doubling from shareRetryMin up to shareRetryMax.
var (
	shareRetryMin = 2 * time.Second
	shareRetryMax = time.Minute
)

const (
	shareBatch     = 50
	sharePhotoSide = 800
	shareIdleCheck = 30 * time.Second
)

func NewSharing(store ShareStore) *Sharing {
	return &Sharing{store: store, client: &http.Client{Timeout: 30 * time.Second}, wake: make(chan struct{}, 1)}
}

// Attach gives the sender what it reads from. images resolves a stored photo's path;
// colorNames names colours (for a review's corrected colour).
func (s *Sharing) Attach(records RecordsStore, collections *Collections, images func(string) (string, error),
	colorNames func(context.Context) (map[string]string, error)) {
	s.records, s.collections, s.images, s.colorNames = records, collections, images, colorNames
}

func (s *Sharing) poke() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *Sharing) note(what string, err error) {
	if err != nil {
		slog.Warn("marking for sharing failed", "what", what, "error", err)
		return
	}
	s.poke()
}

// The marks. Each is a no-op unless the collection is shared. They never fail the
// change they follow; a failure is logged.

func (s *Sharing) CollectionChanged(ctx context.Context, id int64) {
	s.note("collection", s.store.MarkCollection(ctx, id))
}

func (s *Sharing) RunChanged(ctx context.Context, id int64) {
	s.note("run", s.store.MarkRun(ctx, id))
}

func (s *Sharing) PieceChanged(ctx context.Context, id string) {
	s.note("piece", s.store.MarkPiece(ctx, id))
}

func (s *Sharing) PhotoChanged(ctx context.Context, pieceID string) {
	s.note("photo", s.store.MarkPhoto(ctx, pieceID))
}

func (s *Sharing) UnknownsCleared(ctx context.Context, collectionID int64) {
	s.note("clear unknowns", s.store.MarkClearUnknowns(ctx, collectionID))
}

// SetShared turns sharing a collection on (sending everything already in it) or off.
func (s *Sharing) SetShared(ctx context.Context, collectionID int64, shared bool) error {
	if err := s.store.SetShared(ctx, collectionID, shared); err != nil {
		return err
	}
	slog.Info("collection sharing", "collection", collectionID, "shared", shared)
	s.poke()
	return nil
}

// Status is how sending is going.
func (s *Sharing) Status(ctx context.Context) (SharingStatus, error) {
	set, err := s.store.SharingSettings(ctx)
	if err != nil {
		return SharingStatus{}, err
	}
	n, err := s.store.CountMarks(ctx)
	if err != nil {
		return SharingStatus{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return SharingStatus{URL: set.URL, KeySet: set.APIKey != "", Waiting: n,
		Connected: s.connected, LastError: s.lastError, LastSent: s.lastSent}, nil
}

// SaveSettings sets the site and key. An empty key keeps the one saved.
func (s *Sharing) SaveSettings(ctx context.Context, siteURL, key string) error {
	siteURL = strings.TrimRight(strings.TrimSpace(siteURL), "/")
	if siteURL != "" {
		u, err := url.Parse(siteURL)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
			return fmt.Errorf("%w: the site's address should look like https://lego.n8p.xyz", ErrInvalidSettings)
		}
	}
	set, err := s.store.SharingSettings(ctx)
	if err != nil {
		return err
	}
	set.URL = siteURL
	if key = strings.TrimSpace(key); key != "" {
		set.APIKey = key
	}
	if err := s.store.SaveSharingSettings(ctx, set); err != nil {
		return err
	}
	s.mu.Lock()
	s.lastError = ""
	s.mu.Unlock()
	s.poke()
	return nil
}

// Test checks the site answers and takes the key.
func (s *Sharing) Test(ctx context.Context) error {
	err := s.send(ctx, http.MethodGet, "/api/push/ping", "", nil)
	s.noteResult(err)
	return err
}

// Run sends marks until ctx ends.
func (s *Sharing) Run(ctx context.Context) {
	backoff := time.Duration(0)
	for {
		wait := shareIdleCheck
		sent, err := s.sendSome(ctx)
		switch {
		case err != nil:
			backoff = min(max(backoff*2, shareRetryMin), shareRetryMax)
			wait = backoff
		case sent:
			backoff, wait = 0, 0 // more may be waiting
		default:
			backoff = 0
		}
		if wait == 0 {
			if ctx.Err() != nil {
				return
			}
			continue
		}
		t := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-s.wake:
			// A new mark wakes an idle sender, but not one backing off from a failure.
			if err != nil {
				select {
				case <-t.C:
				case <-ctx.Done():
					return
				}
			}
		case <-t.C:
		}
		t.Stop()
	}
}

// sendSome sends the next batch of marks in order. A record the site refuses as bad is
// dropped; any other failure (unreachable, a server error) stops the batch, so marks
// keep their order and are tried again after a backoff. sent reports any progress.
func (s *Sharing) sendSome(ctx context.Context) (sent bool, err error) {
	set, err := s.store.SharingSettings(ctx)
	if err != nil || set.URL == "" || set.APIKey == "" {
		return false, err // nothing to do until it's set up
	}
	marks, err := s.store.NextMarks(ctx, shareBatch)
	if err != nil {
		return false, err
	}
	for _, m := range marks {
		err := s.sendMark(ctx, m)
		var rejected *shareRejected
		switch {
		case err == nil || errors.Is(err, ErrNotShared):
			if err := s.store.DeleteMark(ctx, m.Seq); err != nil {
				return sent, err
			}
			sent = true
			if err == nil {
				s.noteResult(nil)
			}
		case errors.As(err, &rejected):
			// Sending the same record again would be refused again, and waiting on it
			// would hold up everything after it.
			slog.Warn("the sort viewer refused this; dropped", "kind", m.Kind, "ref", m.Ref, "error", err)
			if err := s.store.DeleteMark(ctx, m.Seq); err != nil {
				return sent, err
			}
			s.noteResult(err)
			sent = true
		default:
			_ = s.store.FailMark(ctx, m.Seq, err.Error())
			s.noteResult(err)
			return sent, err
		}
	}
	return sent, nil
}

func (s *Sharing) noteResult(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.connected, s.lastError = false, err.Error()
		return
	}
	now := time.Now().UTC()
	s.connected, s.lastError, s.lastSent = true, "", &now
}

func (s *Sharing) sendMark(ctx context.Context, m ShareMark) error {
	switch m.Kind {
	case ShareCollection:
		return s.sendCollection(ctx, m.Ref)
	case ShareRun:
		return s.sendRun(ctx, m.Ref)
	case SharePiece:
		return s.sendPiece(ctx, m.Ref)
	case SharePhoto:
		return s.sendPhoto(ctx, m.Ref)
	case ShareClearUnknowns:
		return s.send(ctx, http.MethodPost, "/api/push/collections/"+m.Ref+"/clear-unknowns", "", nil)
	}
	return ErrNotShared // a kind this version doesn't know
}

// sharePlan is the plan as the site takes it.
type sharePlan struct {
	Profile       *shareProfile  `json:"profile,omitempty"`
	Bins          []sharePlanBin `json:"bins"`
	BinsStartedAt *time.Time     `json:"bins_started_at,omitempty"`
}

type shareProfile struct {
	Name string `json:"name"`
}

type sharePlanBin struct {
	Layer      int           `json:"layer"`
	Bin        int           `json:"bin"`
	Categories []CategoryRef `json:"categories"`
}

func (s *Sharing) sendCollection(ctx context.Context, ref string) error {
	id, err := strconv.ParseInt(ref, 10, 64)
	if err != nil {
		return ErrNotShared
	}
	plan, err := s.collections.Plan(ctx, id)
	if errors.Is(err, ErrCollectionNotFound) {
		return ErrNotShared
	}
	if err != nil {
		return err
	}
	body := struct {
		Name string    `json:"name"`
		Plan sharePlan `json:"plan"`
	}{Name: plan.Collection.Name, Plan: sharePlan{Bins: []sharePlanBin{}, BinsStartedAt: plan.Collection.BinsStartedAt}}
	if plan.Profile != nil {
		body.Plan.Profile = &shareProfile{Name: plan.Profile.Name}
	}
	for _, b := range plan.Bins {
		if len(b.Categories) > 0 {
			body.Plan.Bins = append(body.Plan.Bins, sharePlanBin{Layer: b.Layer, Bin: b.Bin, Categories: b.Categories})
		}
	}
	return s.sendJSON(ctx, "/api/push/collections/"+ref, body)
}

func (s *Sharing) sendRun(ctx context.Context, ref string) error {
	id, err := strconv.ParseInt(ref, 10, 64)
	if err != nil {
		return ErrNotShared
	}
	run, err := s.store.GetRun(ctx, id)
	if err != nil {
		return err
	}
	return s.sendJSON(ctx, "/api/push/runs/"+ref, map[string]any{
		"collection_id": run.CollectionID, "profile_name": run.ProfileName,
		"started_at": run.StartedAt, "ended_at": run.EndedAt,
	})
}

// SharedPiece is a piece as the site takes it, with any review applied.
type SharedPiece struct {
	CollectionID int64      `json:"collection_id"`
	RunID        int64      `json:"run_id"`
	Status       string     `json:"status"`
	SeenAt       time.Time  `json:"seen_at"`
	SortedAt     *time.Time `json:"sorted_at"`
	PartID       string     `json:"part_id"`
	PartName     string     `json:"part_name"`
	PartCategory string     `json:"part_category"`
	PartScore    float64    `json:"part_score"`
	PartImgURL   string     `json:"part_img_url"`
	ColorID      string     `json:"color_id"`
	ColorName    string     `json:"color_name"`
	Category     string     `json:"category"`
	CategoryName string     `json:"category_name"`
	BinLayer     int        `json:"bin_layer"`
	BinNumber    int        `json:"bin_number"`
	VirtualBin   bool       `json:"virtual_bin"`
}

// notAPart: review flags that mean there's no right part to show.
var notAPart = []string{"not_lego", "no_piece", "multiple_pieces", "assembly"}

// SharedPieceOf is what the site is told about a piece. A review wins over the
// identification: a wrong colour with the right one given shows the right one; a wrong
// part (or something that isn't one LEGO piece) shows as unknown.
func SharedPieceOf(p Piece, collectionID int64, colorName func(string) string) SharedPiece {
	out := SharedPiece{
		CollectionID: collectionID, RunID: p.RunID, Status: string(p.Status), SeenAt: p.SeenAt,
		SortedAt: p.DistributedAt, PartID: p.PartID, PartName: p.PartName, PartCategory: p.PartCategory,
		PartScore: p.PartScore, PartImgURL: p.PartImgURL, ColorID: p.ColorID, ColorName: p.ColorName,
		Category: p.Category, CategoryName: p.CategoryName, BinLayer: p.BinLayer, BinNumber: p.BinNumber,
		VirtualBin: p.VirtualBin,
	}
	r := p.Review
	if r == nil {
		return out
	}
	if r.ColorCorrect != nil && !*r.ColorCorrect && r.ColorID != "" {
		out.ColorID, out.ColorName = r.ColorID, colorName(r.ColorID)
	}
	wrongPart := r.PartCorrect != nil && !*r.PartCorrect
	if wrongPart || slices.ContainsFunc(r.Issues, func(i string) bool { return slices.Contains(notAPart, i) }) {
		out.PartID, out.PartName, out.PartCategory, out.PartScore, out.PartImgURL = "", "", "", 0, ""
		out.ColorID, out.ColorName = "", ""
	}
	return out
}

func (s *Sharing) sendPiece(ctx context.Context, id string) error {
	p, err := s.records.GetPiece(ctx, id)
	if errors.Is(err, ErrPieceNotFound) {
		return ErrNotShared
	}
	if err != nil {
		return err
	}
	if p.RunID == 0 {
		return ErrNotShared
	}
	run, err := s.store.GetRun(ctx, p.RunID)
	if err != nil {
		return err
	}
	names := map[string]string{}
	if s.colorNames != nil && p.Review != nil {
		if names, err = s.colorNames(ctx); err != nil {
			return err
		}
	}
	name := func(id string) string {
		if n := names[id]; n != "" {
			return n
		}
		return "Colour " + id
	}
	return s.sendJSON(ctx, "/api/push/pieces/"+url.PathEscape(id), SharedPieceOf(p, run.CollectionID, name))
}

// sendPhoto sends a piece's last photo (the newest from C4), shrunk.
func (s *Sharing) sendPhoto(ctx context.Context, id string) error {
	p, err := s.records.GetPiece(ctx, id)
	if errors.Is(err, ErrPieceNotFound) {
		return ErrNotShared
	}
	if err != nil {
		return err
	}
	if len(p.Images) == 0 {
		return ErrNotShared // photos cleared
	}
	last := p.Images[0]
	for _, img := range p.Images {
		if img.TakenAt.After(last.TakenAt) || img.TakenAt.Equal(last.TakenAt) && img.Index > last.Index {
			last = img
		}
	}
	path, err := s.images(last.Path)
	if err != nil {
		return ErrNotShared
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return ErrNotShared
	}
	if err != nil {
		return err
	}
	small, err := shrinkJPEG(data, sharePhotoSide)
	if err != nil {
		return ErrNotShared // not a readable picture; nothing to send
	}
	return s.send(ctx, http.MethodPut, "/api/push/pieces/"+url.PathEscape(id)+"/photo", "image/jpeg", small)
}

// shrinkJPEG scales a photo down so its longest side is at most side, as JPEG.
func shrinkJPEG(data []byte, side int) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= side && h <= side {
		return data, nil
	}
	if w >= h {
		h, w = max(1, h*side/w), side
	} else {
		w, h = max(1, w*side/h), side
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)
	var out bytes.Buffer
	if err := jpeg.Encode(&out, dst, &jpeg.Options{Quality: 82}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// shareRejected is the site refusing a record as bad (a 4xx), as opposed to being
// unreachable or broken.
type shareRejected struct {
	status int
	msg    string
}

func (e *shareRejected) Error() string {
	return fmt.Sprintf("the site refused it (%d): %s", e.status, e.msg)
}

func (s *Sharing) sendJSON(ctx context.Context, path string, body any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	return s.send(ctx, http.MethodPut, path, "application/json", data)
}

func (s *Sharing) send(ctx context.Context, method, path, contentType string, body []byte) error {
	set, err := s.store.SharingSettings(ctx)
	if err != nil {
		return err
	}
	if set.URL == "" || set.APIKey == "" {
		return ErrSharingNotSetUp
	}
	req, err := http.NewRequestWithContext(ctx, method, set.URL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+set.APIKey)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("can't reach the site: %w", err)
	}
	defer resp.Body.Close()
	msg, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	switch {
	case resp.StatusCode < 300:
		return nil
	case resp.StatusCode == http.StatusUnauthorized:
		return errors.New("the site refused the key: make a new one under its Admin → Sorter keys")
	case resp.StatusCode >= 400 && resp.StatusCode < 500 &&
		resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusRequestTimeout &&
		resp.StatusCode != http.StatusTooManyRequests:
		var e struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(msg, &e)
		return &shareRejected{status: resp.StatusCode, msg: e.Error}
	default:
		return fmt.Errorf("the site answered %s", resp.Status)
	}
}
