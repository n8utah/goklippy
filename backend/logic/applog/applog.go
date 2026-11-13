// Package applog keeps the backend's logs for the Diagnose › Logs page: every record
// at Info or above, as JSON lines in hourly files (YYYYMMDD-HH.jsonl) under a folder,
// kept for a rolling window (24 h) and read back newest first with filters.
//
// A message logged over and over (a camera restarting every few seconds) is kept at
// most repeatBurst times a minute; the rest are counted and noted in one line when the
// minute is up, so a loop can't bury everything else.
package applog

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
)

// Entry is one log record.
type Entry struct {
	Time  time.Time         `json:"time"`
	Level string            `json:"level"` // DEBUG, INFO, WARN, ERROR
	Msg   string            `json:"msg"`
	Attrs map[string]string `json:"attrs,omitempty"`
}

const (
	fileLayout  = "20060102-15"
	fileSuffix  = ".jsonl"
	repeatBurst = 5           // the same message kept this many times a window …
	repeatEvery = time.Minute // … per window
)

// Store writes entries to hourly files under dir and reads them back.
type Store struct {
	dir  string
	keep time.Duration

	mu      sync.Mutex
	file    *os.File
	hour    string
	repeats map[string]*repeat
	now     func() time.Time
}

type repeat struct {
	since   time.Time
	kept    int
	dropped int
	level   slog.Level
	msg     string
}

// Open keeps logs in dir for keep (24 h), removing older files now and as it goes.
func Open(dir string, keep time.Duration) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s := &Store{dir: dir, keep: keep, repeats: map[string]*repeat{}, now: time.Now}
	s.prune(s.now())
	return s, nil
}

// Close flushes and closes the current file.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file == nil {
		return nil
	}
	err := s.file.Close()
	s.file = nil
	return err
}

// Add writes one entry, unless it's a repeat over the limit.
func (s *Store) Add(level slog.Level, e Entry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.flushRepeats(e.Time, false)
	// The same message for different requests isn't a repeat.
	key := e.Msg
	if p := e.Attrs["path"]; p != "" {
		key += "\x00" + e.Attrs["method"] + " " + p
	}
	r := s.repeats[key]
	if r == nil {
		r = &repeat{since: e.Time, level: level, msg: e.Msg}
		s.repeats[key] = r
	}
	if r.kept >= repeatBurst {
		r.dropped++
		return
	}
	r.kept++
	s.write(e)
}

// flushRepeats notes messages dropped as repeats once their window is over (all of
// them when force), and forgets finished windows.
func (s *Store) flushRepeats(now time.Time, force bool) {
	for key, r := range s.repeats {
		if !force && now.Sub(r.since) < repeatEvery {
			continue
		}
		if r.dropped > 0 {
			s.write(Entry{
				Time:  now,
				Level: r.level.String(),
				Msg:   r.msg,
				Attrs: map[string]string{"repeated": fmt.Sprintf("%d more times in %s, not each kept", r.dropped, now.Sub(r.since).Round(time.Second))},
			})
		}
		delete(s.repeats, key)
	}
}

func (s *Store) write(e Entry) {
	hour := e.Time.UTC().Format(fileLayout)
	if s.file == nil || hour != s.hour {
		if s.file != nil {
			_ = s.file.Close()
		}
		f, err := os.OpenFile(filepath.Join(s.dir, hour+fileSuffix), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			s.file = nil
			return
		}
		s.file, s.hour = f, hour
		s.prune(e.Time)
	}
	b, err := json.Marshal(e)
	if err != nil {
		return
	}
	_, _ = s.file.Write(append(b, '\n'))
}

// prune removes hourly files wholly older than the window.
func (s *Store) prune(now time.Time) {
	for _, name := range s.files() {
		start, err := time.Parse(fileLayout, strings.TrimSuffix(name, fileSuffix))
		if err == nil && now.Sub(start.Add(time.Hour)) > s.keep {
			_ = os.Remove(filepath.Join(s.dir, name))
		}
	}
}

// files are the log files, oldest first.
func (s *Store) files() []string {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), fileSuffix) {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

// Filter narrows a query. Before pages back (entries older than it); Since keeps only
// newer ones (for following live). MinLevel is DEBUG, INFO, WARN or ERROR.
type Filter struct {
	MinLevel string
	Search   string // in the message or any attribute, any case
	Since    time.Time
	Before   time.Time
	Limit    int
}

// Query returns matching entries newest first, and whether there are older ones.
func (s *Store) Query(f Filter) ([]Entry, bool) {
	limit := f.Limit
	if limit <= 0 || limit > 2000 {
		limit = 500
	}
	min := levelOf(f.MinLevel)
	q := strings.ToLower(strings.TrimSpace(f.Search))
	files := s.files()
	slices.Reverse(files)
	var out []Entry
	for _, name := range files {
		start, err := time.Parse(fileLayout, strings.TrimSuffix(name, fileSuffix))
		if err == nil && !f.Since.IsZero() && start.Add(time.Hour).Before(f.Since) {
			break // this file and every older one end before Since
		}
		entries := s.read(name)
		for i := len(entries) - 1; i >= 0; i-- {
			e := entries[i]
			if !f.Before.IsZero() && !e.Time.Before(f.Before) {
				continue
			}
			if !f.Since.IsZero() && !e.Time.After(f.Since) {
				continue
			}
			if levelOf(e.Level) < min || (q != "" && !matches(e, q)) {
				continue
			}
			if len(out) == limit {
				return out, true
			}
			out = append(out, e)
		}
	}
	return out, false
}

// Clear removes every log file.
func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file != nil {
		_ = s.file.Close()
		s.file = nil
	}
	for _, name := range s.files() {
		if err := os.Remove(filepath.Join(s.dir, name)); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) read(name string) []Entry {
	s.mu.Lock() // the open file may be mid-write
	defer s.mu.Unlock()
	f, err := os.Open(filepath.Join(s.dir, name))
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []Entry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		var e Entry
		if json.Unmarshal(sc.Bytes(), &e) == nil {
			out = append(out, e)
		}
	}
	return out
}

func matches(e Entry, q string) bool {
	if strings.Contains(strings.ToLower(e.Msg), q) {
		return true
	}
	for k, v := range e.Attrs {
		if strings.Contains(strings.ToLower(k+"="+v), q) {
			return true
		}
	}
	return false
}

func levelOf(name string) slog.Level {
	var l slog.Level
	if l.UnmarshalText([]byte(strings.ToUpper(name))) != nil {
		return slog.LevelInfo
	}
	return l
}

// Handler is a slog.Handler that keeps records at min or above in the store.
type Handler struct {
	store  *Store
	min    slog.Level
	attrs  []slog.Attr
	groups []string
}

// Handler keeps records at min or above.
func (s *Store) Handler(min slog.Level) *Handler { return &Handler{store: s, min: min} }

func (h *Handler) Enabled(_ context.Context, l slog.Level) bool { return l >= h.min }

func (h *Handler) Handle(_ context.Context, r slog.Record) error {
	attrs := map[string]string{}
	prefix := strings.Join(h.groups, ".")
	if prefix != "" {
		prefix += "."
	}
	for _, a := range h.attrs {
		addAttr(attrs, "", a)
	}
	r.Attrs(func(a slog.Attr) bool {
		addAttr(attrs, prefix, a)
		return true
	})
	if len(attrs) == 0 {
		attrs = nil
	}
	t := r.Time
	if t.IsZero() {
		t = time.Now()
	}
	h.store.Add(r.Level, Entry{Time: t.UTC(), Level: r.Level.String(), Msg: r.Message, Attrs: attrs})
	return nil
}

func addAttr(out map[string]string, prefix string, a slog.Attr) {
	v := a.Value.Resolve()
	if v.Kind() == slog.KindGroup {
		for _, g := range v.Group() {
			addAttr(out, prefix+a.Key+".", g)
		}
		return
	}
	if a.Key == "" {
		return
	}
	out[prefix+a.Key] = v.String()
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h2 := *h
	h2.attrs = append(slices.Clone(h.attrs), attrs...)
	return &h2
}

func (h *Handler) WithGroup(name string) slog.Handler {
	h2 := *h
	h2.groups = append(slices.Clone(h.groups), name)
	return &h2
}

// Tee sends each record to every handler that wants it.
type Tee []slog.Handler

func (t Tee) Enabled(ctx context.Context, l slog.Level) bool {
	for _, h := range t {
		if h.Enabled(ctx, l) {
			return true
		}
	}
	return false
}

func (t Tee) Handle(ctx context.Context, r slog.Record) error {
	for _, h := range t {
		if h.Enabled(ctx, r.Level) {
			_ = h.Handle(ctx, r.Clone())
		}
	}
	return nil
}

func (t Tee) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := make(Tee, len(t))
	for i, h := range t {
		out[i] = h.WithAttrs(attrs)
	}
	return out
}

func (t Tee) WithGroup(name string) slog.Handler {
	out := make(Tee, len(t))
	for i, h := range t {
		out[i] = h.WithGroup(name)
	}
	return out
}
