package logic

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"sync"
	"time"
)

// Live watching (README, "Live"): while a shared collection is sorting, the people who
// can see it can watch the sort as it happens: the cameras, the pieces going through,
// and the run's numbers. The sorter can't be reached from here, so it keeps asking (a
// long poll, LiveReport), saying which collection it's sorting, and is told how many are
// watching; only while someone is does it send frames and its state. Nothing is
// stored: the newest of each is kept in memory and handed to the pages watching.
//
// One sorter: the newest report says what's sorting.

const (
	// LiveWait is the longest a report is held waiting for the watchers to change.
	LiveWait = 20 * time.Second
	// liveStale: no report for this long and the sorter is taken to be gone.
	liveStale = 45 * time.Second
	// liveViewerQueue: events a slow page may fall behind by before it misses some (a
	// frame it misses is only a notice; it fetches the newest anyway).
	liveViewerQueue = 32
)

// LiveChannels are the cameras that can be watched.
var LiveChannels = []string{"c2", "c3", "c4"}

var (
	ErrNotLive     = errors.New("that collection isn't being sorted right now")
	ErrLiveChannel = errors.New("no such camera")
)

// LiveReport is the sorter's word on what it's doing: the collection sorting now (0 for
// none, or one that isn't shared) and how many watchers it last heard of.
type LiveReport struct {
	CollectionID int64 `json:"collection_id"`
	Watchers     int   `json:"watchers"`
}

// LiveEvent is what a watching page is sent: "state" (the sorter's state, as it sent
// it), "frame" (a newer picture from a camera: fetch it), or "ended" (sorting stopped,
// or the sorter went quiet).
type LiveEvent struct {
	Type    string          `json:"type"`
	State   json.RawMessage `json:"state,omitempty"`
	Channel string          `json:"channel,omitempty"`
	Seq     int64           `json:"seq,omitempty"`
}

type liveFrame struct {
	jpeg []byte
	seq  int64
}

// Live relays the sorter's live view to the pages watching.
type Live struct {
	now func() time.Time
	// onChange is told when a collection starts or stops being live.
	onChange func(collectionID int64)

	mu         sync.Mutex
	collection int64 // live now, 0 for none
	heardAt    time.Time
	state      json.RawMessage
	frames     map[string]liveFrame
	seq        int64
	viewers    map[chan LiveEvent]struct{}
	// watchers is closed (and replaced) when the number watching changes, waking the
	// sorter's report.
	watchers chan struct{}
}

func NewLive(now func() time.Time, onChange func(collectionID int64)) *Live {
	return &Live{now: now, onChange: onChange, frames: map[string]liveFrame{},
		viewers: map[chan LiveEvent]struct{}{}, watchers: make(chan struct{})}
}

// IsLive reports whether a collection is being sorted now, with the sorter in touch.
func (l *Live) IsLive(collectionID int64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.liveLocked(collectionID)
}

func (l *Live) liveLocked(collectionID int64) bool {
	return collectionID != 0 && l.collection == collectionID && l.now().Sub(l.heardAt) < liveStale
}

// Report takes the sorter's word and returns how many are watching, holding on (up to
// LiveWait, or until ctx ends) while that's still the number it knows.
func (l *Live) Report(ctx context.Context, r LiveReport) int {
	l.mu.Lock()
	if l.collection != r.CollectionID || l.now().Sub(l.heardAt) >= liveStale {
		l.switchLocked(r.CollectionID)
	}
	l.heardAt = l.now()
	n, wake := len(l.viewers), l.watchers
	l.mu.Unlock()
	if n != r.Watchers {
		return n
	}
	t := time.NewTimer(LiveWait)
	defer t.Stop()
	select {
	case <-wake:
	case <-t.C:
	case <-ctx.Done():
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.viewers)
}

// switchLocked makes another collection (or none) the live one: what was kept for the
// last is dropped and its watchers told it ended.
func (l *Live) switchLocked(collectionID int64) {
	old := l.collection
	l.collection, l.state, l.frames = collectionID, nil, map[string]liveFrame{}
	l.endViewersLocked()
	if l.onChange != nil {
		for _, id := range []int64{old, collectionID} {
			if id != 0 {
				go l.onChange(id)
			}
		}
	}
}

func (l *Live) endViewersLocked() {
	if len(l.viewers) == 0 {
		return
	}
	for ch := range l.viewers {
		select {
		case ch <- LiveEvent{Type: "ended"}:
		default:
		}
		delete(l.viewers, ch)
	}
	l.watchersChangedLocked()
}

func (l *Live) watchersChangedLocked() {
	close(l.watchers)
	l.watchers = make(chan struct{})
}

// PutState keeps the sorter's newest state and sends it to the pages watching.
func (l *Live) PutState(collectionID int64, state json.RawMessage) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.liveLocked(collectionID) {
		return ErrNotLive
	}
	l.state = slices.Clone(state)
	l.sendLocked(LiveEvent{Type: "state", State: l.state})
	return nil
}

// PutFrame keeps a camera's newest picture and tells the pages watching.
func (l *Live) PutFrame(collectionID int64, channel string, jpeg []byte) error {
	if !slices.Contains(LiveChannels, channel) {
		return ErrLiveChannel
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.liveLocked(collectionID) {
		return ErrNotLive
	}
	l.seq++
	l.frames[channel] = liveFrame{jpeg: slices.Clone(jpeg), seq: l.seq}
	l.sendLocked(LiveEvent{Type: "frame", Channel: channel, Seq: l.seq})
	return nil
}

// sendLocked hands an event to every page watching. One too slow to keep up misses it
// rather than holding up the sorter.
func (l *Live) sendLocked(e LiveEvent) {
	for ch := range l.viewers {
		select {
		case ch <- e:
		default:
		}
	}
}

// Watch starts a page watching a live collection: it's sent the newest state and
// pictures at once, then each as it comes. Call stop when it goes.
func (l *Live) Watch(collectionID int64) (<-chan LiveEvent, func(), error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.liveLocked(collectionID) {
		return nil, nil, ErrNotLive
	}
	ch := make(chan LiveEvent, liveViewerQueue)
	if l.state != nil {
		ch <- LiveEvent{Type: "state", State: l.state}
	}
	for _, name := range LiveChannels {
		if f, ok := l.frames[name]; ok {
			ch <- LiveEvent{Type: "frame", Channel: name, Seq: f.seq}
		}
	}
	l.viewers[ch] = struct{}{}
	l.watchersChangedLocked()
	return ch, func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		if _, ok := l.viewers[ch]; ok {
			delete(l.viewers, ch)
			l.watchersChangedLocked()
		}
	}, nil
}

// Frame is a camera's newest picture, while its collection is live.
func (l *Live) Frame(collectionID int64, channel string) ([]byte, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.liveLocked(collectionID) {
		return nil, ErrNotLive
	}
	f, ok := l.frames[channel]
	if !ok {
		return nil, ErrLiveChannel
	}
	return f.jpeg, nil
}

// Watchers is how many pages are watching now.
func (l *Live) Watchers() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.viewers)
}
