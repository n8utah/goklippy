package sorter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// Watching live on the sort viewer site (its README, "Live"). While a shared collection
// is sorting, the people who can see it can watch the sort as it happens there: the
// cameras, the pieces going through, the run's numbers. The site can't reach the
// sorter, so the sorter keeps asking it (a long poll, saying which shared collection is
// sorting) and hears how many are watching. Only while someone is does it send
// anything more: each camera's picture a few times a second, and its state every
// second. Nobody watching costs one small request every LiveWait or so.

// LiveSource is what's shown live: the machine's state and the cameras' newest pictures.
type LiveSource struct {
	Snapshot func() Snapshot
	Frame    func(ch Channel) ([]byte, time.Time, bool)
}

const (
	liveFrameSide  = 640                    // pictures are shrunk to this on their longest side
	liveFrameEvery = 250 * time.Millisecond // at most 4 a second per camera
	liveStateEvery = time.Second
	liveRecent     = 12 // pieces sorted lately sent with the state
	// The report is held by the site up to 20 s; a status change cuts it short.
	liveReportTimeout = 40 * time.Second
	livePushTimeout   = 10 * time.Second
	liveRetryMax      = time.Minute
	liveIdleCheck     = 30 * time.Second // not set up: look again this often
)

// liveChannels are the cameras sent, in order.
var liveChannels = []Channel{ChannelC2, ChannelC3, ChannelC4}

// liveStatus is what the sorter tells the site: the shared collection sorting now.
type liveStatus struct {
	collection int64 // 0: none (or not a shared one)
	runID      int64
}

// liveView is the live side of sharing.
type liveView struct {
	src    LiveSource
	client *http.Client

	mu       sync.Mutex
	watchers int
	watching int64 // the collection they're watching
	lastErr  string
	// shared remembers whether each collection is shared, looked at again now and then.
	shared map[int64]sharedCheck
}

type sharedCheck struct {
	shared bool
	at     time.Time
}

const sharedRecheck = 15 * time.Second

// AttachLive gives sharing what to show live. Call before RunLive.
func (s *Sharing) AttachLive(src LiveSource) {
	s.live = &liveView{src: src, client: &http.Client{}, shared: map[int64]sharedCheck{}}
}

// LiveWatchers is how many are watching the sort live now, and which collection.
func (s *Sharing) LiveWatchers() (int, int64) {
	if s.live == nil {
		return 0, 0
	}
	s.live.mu.Lock()
	defer s.live.mu.Unlock()
	return s.live.watchers, s.live.watching
}

// RunLive keeps the site told what's sorting, and streams to it while anyone watches,
// until ctx ends.
func (s *Sharing) RunLive(ctx context.Context) {
	if s.live == nil {
		return
	}
	go s.streamLive(ctx)
	backoff := time.Duration(0)
	known := -1
	var last liveStatus
	for ctx.Err() == nil {
		set, err := s.store.SharingSettings(ctx)
		if err != nil || set.URL == "" || set.APIKey == "" {
			s.setWatchers(0, 0, "")
			sleepCtx(ctx, liveIdleCheck)
			continue
		}
		st := s.liveStatus(ctx)
		if st != last {
			known, last = -1, st
		}
		n, err := s.reportLive(ctx, st, known)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			s.setWatchers(0, 0, err.Error())
			known = -1
			backoff = min(max(backoff*2, shareRetryMin), liveRetryMax)
			sleepCtx(ctx, backoff)
			continue
		}
		if n != known && (n > 0) != (known > 0) {
			slog.Info("sort viewer: watching live", "collection", st.collection, "watchers", n)
		}
		backoff, known = 0, n
		s.setWatchers(n, st.collection, "")
	}
}

func (s *Sharing) setWatchers(n int, collection int64, errText string) {
	s.live.mu.Lock()
	defer s.live.mu.Unlock()
	if n == 0 {
		collection = 0
	}
	s.live.watchers, s.live.watching, s.live.lastErr = n, collection, errText
}

// liveStatus is the shared collection a run is sorting into now, if any.
func (s *Sharing) liveStatus(ctx context.Context) liveStatus {
	snap := s.live.src.Snapshot()
	if snap.RunID == 0 || snap.Collection == nil {
		return liveStatus{}
	}
	if !s.isShared(ctx, snap.Collection.ID) {
		return liveStatus{}
	}
	return liveStatus{collection: snap.Collection.ID, runID: snap.RunID}
}

func (s *Sharing) isShared(ctx context.Context, id int64) bool {
	l := s.live
	l.mu.Lock()
	c, ok := l.shared[id]
	l.mu.Unlock()
	if ok && time.Since(c.at) < sharedRecheck {
		return c.shared
	}
	col, err := s.collections.Get(ctx, id)
	shared := err == nil && col.Shared
	l.mu.Lock()
	l.shared[id] = sharedCheck{shared: shared, at: time.Now()}
	l.mu.Unlock()
	return shared
}

// reportLive tells the site what's sorting and returns how many are watching. The site
// holds it while that's still known; it's cut short when what's sorting changes.
func (s *Sharing) reportLive(ctx context.Context, st liveStatus, known int) (int, error) {
	rctx, cancel := context.WithTimeout(ctx, liveReportTimeout)
	defer cancel()
	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-rctx.Done():
				return
			case <-t.C:
				if s.liveStatus(rctx) != st {
					cancel()
					return
				}
			}
		}
	}()
	body, _ := json.Marshal(map[string]any{"collection_id": st.collection, "watchers": known})
	out, err := s.liveRequest(rctx, http.MethodPost, "/api/push/live", "application/json", body)
	if err != nil {
		if ctx.Err() == nil && rctx.Err() != nil && !errors.Is(rctx.Err(), context.DeadlineExceeded) {
			return known, nil // what's sorting changed: report again at once
		}
		return 0, err
	}
	var resp struct {
		Watchers int `json:"watchers"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return 0, fmt.Errorf("the site's answer: %w", err)
	}
	return resp.Watchers, nil
}

// streamLive sends the cameras and the state while anyone is watching.
func (s *Sharing) streamLive(ctx context.Context) {
	t := time.NewTicker(liveFrameEvery)
	defer t.Stop()
	sent := map[Channel]time.Time{}
	var stateAt time.Time
	failing := false
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		n, collection := s.LiveWatchers()
		if n == 0 || collection == 0 {
			clear(sent)
			stateAt = time.Time{}
			continue
		}
		if st := s.liveStatus(ctx); st.collection != collection {
			continue // stopped, or another collection: the next report says so
		}
		err := s.sendLiveFrames(ctx, collection, sent)
		if err == nil && time.Since(stateAt) >= liveStateEvery {
			stateAt = time.Now()
			err = s.sendLiveState(ctx, collection)
		}
		switch {
		case err != nil && !failing:
			slog.Warn("sort viewer: live view not sent", "error", err)
			failing = true
		case err == nil && failing:
			slog.Info("sort viewer: live view sending again")
			failing = false
		}
		if err != nil {
			sleepCtx(ctx, 2*time.Second)
		}
	}
}

func (s *Sharing) sendLiveFrames(ctx context.Context, collection int64, sent map[Channel]time.Time) error {
	for _, ch := range liveChannels {
		jpg, at, ok := s.live.src.Frame(ch)
		if !ok || !at.After(sent[ch]) {
			continue
		}
		sent[ch] = at
		small, err := shrinkJPEG(jpg, liveFrameSide)
		if err != nil {
			continue
		}
		path := fmt.Sprintf("/api/push/live/%d/frames/%s", collection, ch)
		if _, err := s.liveRequest(ctx, http.MethodPut, path, "image/jpeg", small); err != nil {
			return err
		}
	}
	return nil
}

// LivePiece is a piece as the live view shows it.
type LivePiece struct {
	ID string `json:"id"`
	SharedPiece
}

// LiveRun is the run's numbers so far.
type LiveRun struct {
	ID          int64      `json:"id"`
	StartedAt   time.Time  `json:"started_at"`
	Pieces      int        `json:"pieces"`
	Sorted      int        `json:"sorted"`
	SortingMs   int64      `json:"sorting_ms"` // since it started, less the time blocked
	BlockedMs   int64      `json:"blocked_ms"` // paused, or waiting on an incident
	PerMinute   float64    `json:"per_minute"` // sorted per minute of sorting
	BlockedNow  bool       `json:"blocked_now"`
	LastPieceAt *time.Time `json:"last_piece_at,omitempty"`
}

// LiveState is the sort as it is now, for the live view.
type LiveState struct {
	At       time.Time          `json:"at"`
	State    State              `json:"state"`
	Incident string             `json:"incident,omitempty"` // stopped for this until it's fixed
	Doing    map[Channel]string `json:"doing"`
	// OnChannel is how many pieces each camera sees on its channel.
	OnChannel map[Channel]int `json:"on_channel"`
	Aboard    []LivePiece     `json:"aboard"` // on C4 now, being photographed and named
	Recent    []LivePiece     `json:"recent"` // sorted lately, newest first
	Run       *LiveRun        `json:"run,omitempty"`
}

func (s *Sharing) sendLiveState(ctx context.Context, collection int64) error {
	st := s.liveState(ctx, collection)
	body, err := json.Marshal(st)
	if err != nil {
		return err
	}
	_, err = s.liveRequest(ctx, http.MethodPut, fmt.Sprintf("/api/push/live/%d/state", collection), "application/json", body)
	return err
}

func (s *Sharing) liveState(ctx context.Context, collection int64) LiveState {
	snap := s.live.src.Snapshot()
	now := time.Now().UTC()
	out := LiveState{At: now, State: snap.State, Doing: snap.Doing, OnChannel: map[Channel]int{},
		Aboard: []LivePiece{}, Recent: []LivePiece{}}
	if snap.Incident != nil {
		out.Incident = snap.Incident.Title
	}
	for ch, sum := range snap.Channels {
		out.OnChannel[ch] = sum.Pieces
	}
	colour := func(id string) string { return "Colour " + id }
	piece := func(p Piece) LivePiece {
		return LivePiece{ID: p.ID, SharedPiece: SharedPieceOf(p, collection, colour)}
	}
	for _, p := range snap.Aboard {
		out.Aboard = append(out.Aboard, piece(p))
	}
	for i, p := range snap.Recent {
		if i >= liveRecent {
			break
		}
		out.Recent = append(out.Recent, piece(p))
	}
	if s.records != nil {
		if runs, err := s.records.ListRuns(ctx, collection, 1); err == nil && len(runs) > 0 && runs[0].ID == snap.RunID {
			out.Run = liveRun(runs[0], now)
		}
	}
	return out
}

// liveRun works out the run's numbers as Run › Sort does (helpers/sorterRuns.js): its
// sorting time leaves out the time it stood blocked.
func liveRun(r Run, now time.Time) *LiveRun {
	blocked := r.BlockedMs
	if r.BlockedSince != nil && r.EndedAt == nil {
		blocked += max(0, now.Sub(*r.BlockedSince).Milliseconds())
	}
	sorting := max(0, now.Sub(r.StartedAt).Milliseconds()-blocked)
	out := &LiveRun{ID: r.ID, StartedAt: r.StartedAt, Pieces: r.Pieces, Sorted: r.Distributed,
		SortingMs: sorting, BlockedMs: blocked, BlockedNow: r.BlockedSince != nil, LastPieceAt: r.LastPieceAt}
	if sorting >= 1000 {
		out.PerMinute = float64(r.Distributed) / (float64(sorting) / 60000)
	}
	return out
}

// liveRequest is one call to the site's live API, returning its answer.
func (s *Sharing) liveRequest(ctx context.Context, method, path, contentType string, body []byte) ([]byte, error) {
	set, err := s.store.SharingSettings(ctx)
	if err != nil {
		return nil, err
	}
	if set.URL == "" || set.APIKey == "" {
		return nil, ErrSharingNotSetUp
	}
	if method != http.MethodPost {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, livePushTimeout)
		defer cancel()
	}
	req, err := http.NewRequestWithContext(ctx, method, set.URL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+set.APIKey)
	req.Header.Set("Content-Type", contentType)
	resp, err := s.live.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("can't reach the site: %w", err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	switch {
	case resp.StatusCode < 300:
		return out, nil
	case resp.StatusCode == http.StatusUnauthorized:
		return nil, errors.New("the site refused the key: make a new one under its Admin → Sorter keys")
	case resp.StatusCode == http.StatusNotFound && method == http.MethodPost:
		return nil, errors.New("the site doesn't take live views yet: deploy the newer sort viewer")
	}
	return nil, fmt.Errorf("the site answered %d: %s", resp.StatusCode, bytes.TrimSpace(out))
}

func sleepCtx(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
