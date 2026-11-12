package sorter

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"goklippy/backend/logic/vision"
)

// Journeys: the pictures of each tracked piece as it rides a channel, which were kept
// as good pictures of it and why the rest weren't, kept for the Piece journeys page.
// This is phase 9 step 1 (docs/lego-sorter-server-plan.md): one journey per track on
// one channel. Linking a piece's tracks across channels comes next.

// Journey is one piece's pictures and the decisions about them.
type Journey struct {
	ID      string          `json:"id"`
	Started time.Time       `json:"started"`
	Updated time.Time       `json:"updated"`
	Done    bool            `json:"done"`
	Tracks  []*JourneyTrack `json:"tracks"`
	// Seen counts every frame of the piece, Kept the good pictures, Reasons the rest
	// by why they weren't kept.
	Seen    int            `json:"seen"`
	Kept    int            `json:"kept"`
	Reasons map[string]int `json:"reasons"`
	Thumb   string         `json:"thumb,omitempty"` // the sharpest kept picture
	// Handoffs are its falls from one camera channel to the next, linked or not.
	Handoffs []JourneyHandoff `json:"handoffs,omitempty"`
	// PieceID is the piece record it became on C4, once known.
	PieceID string `json:"piece_id,omitempty"`
}

// JourneyHandoff is a piece landing on a channel from the one above, and whether its
// journey could carry on: only when exactly one piece left the channel above just
// before, so a picture of one piece can never join another's journey.
type JourneyHandoff struct {
	From       Channel   `json:"from"`
	To         Channel   `json:"to"`
	At         time.Time `json:"at"`
	Result     string    `json:"result"`     // HandoffLinked, HandoffNone or HandoffSeveral
	Candidates int       `json:"candidates"` // pieces that had just left the channel above
	GapMs      int64     `json:"gap_ms,omitempty"`
}

// What became of a handoff.
const (
	HandoffLinked  = "linked"  // one piece left the channel above just before: this is it
	HandoffNone    = "none"    // nothing left the channel above just before
	HandoffSeveral = "several" // more than one piece could be it, so none is assumed
)

// upstream is the camera channel each channel's pieces fall in from.
var upstream = map[Channel]Channel{ChannelC3: ChannelC2, ChannelC4: ChannelC3}

// A channel with no frame for this long has no camera to hand off from.
const upstreamCameraFresh = 5 * time.Second

// JourneyTrack is the piece's track on one channel.
type JourneyTrack struct {
	Channel   Channel         `json:"channel"`
	TrackID   int             `json:"track_id"`
	Started   time.Time       `json:"started"`
	Ended     time.Time       `json:"ended,omitempty"`
	EndReason string          `json:"end_reason,omitempty"`
	FrameW    int             `json:"frame_w"`
	FrameH    int             `json:"frame_h"`
	Ring      vision.Ring     `json:"ring"`
	Frames    []JourneyFrame  `json:"frames"`
	Moments   []JourneyMoment `json:"moments"`
	// Unlisted counts frames past the per-track limit, judged but not listed.
	Unlisted int `json:"unlisted,omitempty"`
}

// JourneyFrame is one frame of the piece and the decision about it.
type JourneyFrame struct {
	At        time.Time   `json:"at"`
	Angle     float64     `json:"angle"`
	Zone      vision.Zone `json:"zone"`
	Box       vision.Box  `json:"box"`
	Area      int         `json:"area"`
	Sharpness float64     `json:"sharpness"`
	Kept      bool        `json:"kept"`
	Reason    string      `json:"reason,omitempty"` // why not kept
	Thumb     string      `json:"thumb,omitempty"`  // small picture of the crop
	Crop      string      `json:"crop,omitempty"`   // the full crop, for kept ones
}

// JourneyMoment is the whole frame at a moment worth seeing: landing, last seen.
type JourneyMoment struct {
	Label string       `json:"label"`
	At    time.Time    `json:"at"`
	Image string       `json:"image"`
	Boxes []vision.Box `json:"boxes"` // every piece in the frame
	Piece int          `json:"piece"` // which box is this piece
}

// JourneySummary is a journey in the list.
type JourneySummary struct {
	ID        string         `json:"id"`
	Started   time.Time      `json:"started"`
	Updated   time.Time      `json:"updated"`
	Done      bool           `json:"done"`
	Channels  []Channel      `json:"channels"`
	EndReason string         `json:"end_reason,omitempty"`
	Seen      int            `json:"seen"`
	Kept      int            `json:"kept"`
	Reasons   map[string]int `json:"reasons"`
	Thumb     string         `json:"thumb,omitempty"`
	PieceID   string         `json:"piece_id,omitempty"`
}

// Why a frame isn't kept.
const (
	ReasonCutOff   = "cut off"
	ReasonNotAlone = "not alone"
	ReasonSettling = "settling"
	ReasonMoving   = "moving"
	ReasonSizeJump = "size jump"
	ReasonBlurry   = "blurry"
	ReasonSharper  = "a sharper one nearby"
	ReasonOverMax  = "over the limit"
)

const (
	// A track unseen this long has left; longer than the tracker coasts (2.5 s).
	journeyTimeout   = 3 * time.Second
	journeyMaxFrames = 400 // frames listed per track
	journeyMaxThumbs = 120 // thumbnails per track
	journeyThumbPx   = 96
	journeyMomentPx  = 640
	frameEdgePx      = 2 // a box this close to the frame's edge is cut off
)

var (
	ErrNoJourney  = errors.New("no such journey")
	validJourney  = regexp.MustCompile(`^[0-9a-z-]+$`)
	validJourneyF = regexp.MustCompile(`^[a-z]-[0-9a-z]+\.jpg$`)
)

// JourneyRecorder follows every tracked piece's frames on the camera channels.
type JourneyRecorder struct {
	dir      string
	settings func() MachineSettings

	mu     sync.Mutex
	open   map[trackKey]*journeyState
	recent []*Journey            // finished, oldest first
	seenAt map[Channel]time.Time // each channel's newest frame
	angles map[Channel][]float64 // where the pieces are in it
	pieces []pieceNote           // landed pieces waiting for their journey
}

type trackKey struct {
	ch Channel
	id int
}

type journeyState struct {
	j         *Journey
	t         *JourneyTrack
	lastSeen  time.Time
	lastZone  vision.Zone
	lastAngle float64
	travel    float64 // degrees forward since it was first seen
	frames    int     // frames seen
	areas     []int   // of frames whole and alone, for the usual size
	bins      map[int]int
	thumbs    int
	last      image.Image // the newest frame, for the last-seen moment
	lastBoxes []vision.Box
	lastPiece int
	// used: this track fell and was taken by a landing below, or was one of several
	// that could have been, so it can't be claimed again.
	used bool
	// pending is this track's landing, still waiting to learn whether a piece left the
	// channel above: that channel's camera may not have sent the frame showing it gone.
	pending *JourneyHandoff
}

// dirMove is a pending journey's pictures moving into the journey it turned out to
// carry on, done once the lock is released.
type dirMove struct{ from, to string }

// NewJourneyRecorder keeps journeys under dir and loads the ones saved there.
func NewJourneyRecorder(dir string, settings func() MachineSettings) *JourneyRecorder {
	r := &JourneyRecorder{dir: dir, settings: settings, open: map[trackKey]*journeyState{}, seenAt: map[Channel]time.Time{}, angles: map[Channel][]float64{}}
	r.load()
	return r
}

// save is a picture to write once the lock is released.
type save struct {
	path string
	img  image.Image
	rect image.Rectangle
	max  int
}

// Observe takes one frame of a channel with what perception made of it (pieces with
// track ids, in the frame's zones).
func (r *JourneyRecorder) Observe(ch Channel, img image.Image, z vision.ChannelZones, obs vision.Observation) {
	if !obs.Valid || img == nil {
		return
	}
	ps := r.settings().Pictures.withDefaults()
	now := obs.At
	var saves []save
	var finished []*Journey

	r.mu.Lock()
	boxes := make([]vision.Box, len(obs.Pieces))
	for i, p := range obs.Pieces {
		boxes[i] = p.Box
	}
	// New tracks landing in the drop zone this frame: each may carry on a journey from
	// the channel above, but not when several land at once.
	landing := 0
	for _, p := range obs.Pieces {
		if p.TrackID != 0 && r.open[trackKey{ch, p.TrackID}] == nil && p.Zone == vision.ZoneDrop {
			landing++
		}
	}
	window := time.Duration(ps.HandoffMs) * time.Millisecond
	seen := map[int]bool{}
	for i, p := range obs.Pieces {
		if p.TrackID == 0 {
			continue
		}
		seen[p.TrackID] = true
		key := trackKey{ch, p.TrackID}
		st := r.open[key]
		if st == nil {
			var linked []save
			st, linked = r.begin(ch, p, z, img.Bounds(), now, landing, window)
			saves = append(saves, linked...)
			r.open[key] = st
			saves = append(saves, r.moment(st, "Landed", now, img, boxes, i))
		}
		saves = append(saves, r.judge(st, obs, i, img, z, ps)...)
		st.lastSeen, st.last, st.lastBoxes, st.lastPiece, st.lastZone = now, img, boxes, i, p.Zone
	}
	r.seenAt[ch] = now
	r.angles[ch] = r.angles[ch][:0]
	for _, p := range obs.Pieces {
		r.angles[ch] = append(r.angles[ch], p.Angle)
	}
	// This frame may show a piece gone that a landing below is waiting on.
	linkSaves, moves := r.resolve(now, window)
	saves = append(saves, linkSaves...)
	r.claimWaiting(now, window)
	for key, st := range r.open {
		if key.ch == ch && !seen[key.id] && now.Sub(st.lastSeen) > journeyTimeout {
			saves = append(saves, r.finish(st)...)
			finished = append(finished, st.j)
			delete(r.open, key)
		}
	}
	pruned := r.prune(ps.KeepJourneys)
	var jsons []save
	for _, j := range finished {
		jsons = append(jsons, save{path: filepath.Join(r.dir, j.ID, "journey.json")})
	}
	data := make([][]byte, len(finished))
	for i, j := range finished {
		data[i], _ = json.MarshalIndent(j, "", " ")
	}
	r.mu.Unlock()

	for _, s := range saves {
		if err := writeCrop(s); err != nil {
			slog.Warn("journey picture not saved", "path", s.path, "error", err)
		}
	}
	for _, m := range moves {
		moveJourneyFiles(filepath.Join(r.dir, m.from), filepath.Join(r.dir, m.to))
	}
	for i, s := range jsons {
		if err := os.WriteFile(s.path, data[i], 0o644); err != nil {
			slog.Warn("journey not saved", "path", s.path, "error", err)
		}
	}
	for _, id := range pruned {
		_ = os.RemoveAll(filepath.Join(r.dir, id))
	}
}

func (r *JourneyRecorder) start(ch Channel, p vision.PieceObs, z vision.ChannelZones, bounds image.Rectangle, now time.Time) *journeyState {
	id := fmt.Sprintf("%s-%s-%d", strings.ReplaceAll(now.UTC().Format("20060102-150405.000"), ".", "-"), ch, p.TrackID)
	t := &JourneyTrack{Channel: ch, TrackID: p.TrackID, Started: now, FrameW: bounds.Dx(), FrameH: bounds.Dy(), Ring: z.Ring, Frames: []JourneyFrame{}, Moments: []JourneyMoment{}}
	j := &Journey{ID: id, Started: now, Updated: now, Tracks: []*JourneyTrack{t}, Reasons: map[string]int{}}
	return &journeyState{j: j, t: t, lastAngle: p.Angle, bins: map[int]int{}}
}

// begin starts following a new track. One that lands in the drop zone may be the piece
// that just fell from the channel above: when that's already certain its track joins
// that piece's journey; when the camera above hasn't yet shown the piece gone, it waits
// (pending, decided by resolve). It returns the pictures to save.
func (r *JourneyRecorder) begin(ch Channel, p vision.PieceObs, z vision.ChannelZones, bounds image.Rectangle, now time.Time, landing int, window time.Duration) (*journeyState, []save) {
	st := r.start(ch, p, z, bounds, now)
	up, ok := upstream[ch]
	if !ok || p.Zone != vision.ZoneDrop || now.Sub(r.seenAt[up]) > upstreamCameraFresh {
		return st, nil // no camera above to come from, or it didn't land
	}
	st.pending = &JourneyHandoff{From: up, To: ch, At: now}
	if landing > 1 {
		r.settle(st, HandoffSeveral, nil)
		return st, nil
	}
	saves, _ := r.tryLink(st, now, window)
	return st, saves
}

// tryLink decides a pending landing if it can: linked to the one piece that left the
// channel above, several when more than one could be it, none once the window has
// passed. It returns the pictures to save, and a folder to move when a journey already
// on disk joined another.
func (r *JourneyRecorder) tryLink(st *journeyState, now time.Time, window time.Duration) ([]save, *dirMove) {
	h := st.pending
	cands := r.fallen(h.From, h.At, window)
	switch {
	case len(cands) > 1:
		r.settle(st, HandoffSeveral, cands)
	case len(cands) == 1:
		return r.link(st, cands[0])
	case now.Sub(h.At) > window:
		r.settle(st, HandoffNone, nil)
	}
	return nil, nil
}

// settle records an unlinked handoff on the track's own journey; the candidates, if
// any, are used up: none of them can be told apart from the others.
func (r *JourneyRecorder) settle(st *journeyState, result string, cands []*journeyState) {
	h := *st.pending
	h.Result, h.Candidates = result, len(cands)
	for _, c := range cands {
		c.used = true
	}
	st.j.Handoffs = append(st.j.Handoffs, h)
	st.pending = nil
}

// link carries the fallen piece's journey on with the landed track.
func (r *JourneyRecorder) link(st, from *journeyState) ([]save, *dirMove) {
	h := *st.pending
	st.pending = nil
	h.Result, h.Candidates, h.GapMs = HandoffLinked, 1, h.At.Sub(from.lastSeen).Milliseconds()
	ft := from.t
	ft.Ended, ft.EndReason = from.lastSeen, "fell to "+strings.ToUpper(string(h.To))
	from.used = true
	var saves []save
	if from.last != nil {
		saves = append(saves, r.moment(from, "Fell", from.lastSeen, from.last, from.lastBoxes, from.lastPiece))
	}
	delete(r.open, trackKey{h.From, ft.TrackID})

	j, own := from.j, st.j
	j.Tracks = append(j.Tracks, st.t)
	j.Handoffs = append(j.Handoffs, h)
	j.Seen += own.Seen
	if own.Updated.After(j.Updated) {
		j.Updated = own.Updated
	}
	if j.PieceID == "" {
		j.PieceID = own.PieceID
	}
	st.j = j
	r.tally(st)
	if own.Seen == 0 {
		return saves, nil // nothing of its own on disk yet
	}
	return saves, &dirMove{from: own.ID, to: j.ID}
}

// resolve decides the landings still waiting, now that a channel sent a frame. It
// returns the folders to move.
func (r *JourneyRecorder) resolve(now time.Time, window time.Duration) ([]save, []dirMove) {
	var saves []save
	var moves []dirMove
	for _, st := range r.open {
		if st.pending == nil {
			continue
		}
		s, m := r.tryLink(st, now, window)
		saves = append(saves, s...)
		if m != nil {
			moves = append(moves, *m)
		}
	}
	return saves, moves
}

// fallen are the open tracks on a channel that were last seen at its exit (or just
// short of it) within window before landedAt, and whose spot is empty in its newest
// frame: pieces that just fell off. (A track the tracker lost while its piece sat
// still isn't one: the piece is still there.)
func (r *JourneyRecorder) fallen(ch Channel, landedAt time.Time, window time.Duration) []*journeyState {
	var out []*journeyState
	for key, st := range r.open {
		if key.ch != ch || st.used || (st.lastZone != vision.ZoneExit && st.lastZone != vision.ZonePrecise) {
			continue
		}
		if !st.lastSeen.Before(r.seenAt[ch]) || landedAt.Sub(st.lastSeen) > window ||
			st.lastSeen.After(landedAt.Add(frameSkew)) {
			continue
		}
		gone := true
		for _, a := range r.angles[ch] {
			if math.Abs(vision.ForwardDeg(st.lastAngle, a)) < fallenGapDeg || math.Abs(vision.ForwardDeg(a, st.lastAngle)) < fallenGapDeg {
				gone = false
				break
			}
		}
		if gone {
			out = append(out, st)
		}
	}
	return out
}

// fallenGapDeg: a piece in the newest frame this near a vanished track's last angle is
// that piece, still there under a new track id.
const fallenGapDeg = 8.0

// frameSkew: different cameras' frames aren't taken together, so a piece last seen up
// to this long after another landed can still be the one that fell.
const frameSkew = 100 * time.Millisecond

// pieceNote is a piece record waiting to be tied to its journey.
type pieceNote struct {
	id string
	at time.Time
}

// NotePiece ties the piece that just landed on C4 to its journey: the journey whose C4
// track started as it landed, or with no camera on C4, the piece that just fell off C3.
// The cameras may not have shown that yet, so it waits (checked on every frame) for up
// to the handoff window. It returns the journey's id if it could tell at once.
func (r *JourneyRecorder) NotePiece(pieceID string, at time.Time) string {
	window := time.Duration(r.settings().Pictures.withDefaults().HandoffMs) * time.Millisecond
	r.mu.Lock()
	defer r.mu.Unlock()
	n := pieceNote{id: pieceID, at: at}
	if id, decided := r.claim(n, window); decided {
		return id
	}
	r.pieces = append(r.pieces, n)
	return ""
}

// claim finds a piece's journey. decided is false while it might yet be told (nothing
// fits yet); id is "" when it can't be (several fit).
func (r *JourneyRecorder) claim(n pieceNote, window time.Duration) (id string, decided bool) {
	var cands []*journeyState
	for key, st := range r.open {
		d := st.t.Started.Sub(n.at)
		// Still in the drop zone: pieces carried on by an index aren't the new one.
		if key.ch == ChannelC4 && st.j.PieceID == "" && st.lastZone == vision.ZoneDrop && d <= window && d >= -window {
			cands = append(cands, st)
		}
	}
	if len(cands) == 0 {
		for _, st := range r.fallen(ChannelC3, n.at, window) {
			if st.j.PieceID == "" {
				cands = append(cands, st)
			}
		}
	}
	switch len(cands) {
	case 0:
		return "", false
	case 1:
		cands[0].j.PieceID = n.id
		return cands[0].j.ID, true
	default:
		return "", true
	}
}

// claimWaiting retries the pieces waiting for their journey, giving up on each once the
// window has passed.
func (r *JourneyRecorder) claimWaiting(now time.Time, window time.Duration) {
	kept := r.pieces[:0]
	for _, n := range r.pieces {
		id, decided := r.claim(n, window)
		switch {
		case decided && id != "":
			slog.Info("piece linked to its journey", "piece", n.id, "journey", id)
		case !decided && now.Sub(n.at) <= window:
			kept = append(kept, n)
		}
	}
	r.pieces = kept
}

// ForPiece is the journey that became this piece, as JSON.
func (r *JourneyRecorder) ForPiece(pieceID string) ([]byte, error) {
	if pieceID == "" {
		return nil, ErrNoJourney
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, st := range r.open {
		if st.j.PieceID == pieceID {
			return json.Marshal(st.j)
		}
	}
	for i := len(r.recent) - 1; i >= 0; i-- {
		if r.recent[i].PieceID == pieceID {
			return json.Marshal(r.recent[i])
		}
	}
	return nil, ErrNoJourney
}

// judge decides about one frame of a piece, and returns the pictures to save.
func (r *JourneyRecorder) judge(st *journeyState, obs vision.Observation, i int, img image.Image, z vision.ChannelZones, ps PicturesSettings) []save {
	p := obs.Pieces[i]
	bounds := img.Bounds()
	f := JourneyFrame{At: obs.At, Angle: p.Angle, Zone: p.Zone, Box: p.Box, Area: p.Box.Area()}
	crop := cropRect(p.Box, ps.CropMarginPct, bounds)
	f.Sharpness = math.Round(sharpness(img, crop)*10) / 10

	step := 0.0
	if st.frames > 0 {
		step = vision.ForwardDeg(st.lastAngle, p.Angle)
		if step > 180 {
			step -= 360 // a little backwards
		}
		st.travel += step
	}
	st.lastAngle = p.Angle
	st.frames++

	switch {
	case cutOff(p.Box, z, bounds):
		f.Reason = ReasonCutOff
	case notAlone(obs, i, ps.IsolationPx):
		f.Reason = ReasonNotAlone
	case st.frames <= ps.SettleFrames:
		f.Reason = ReasonSettling
	case math.Abs(step) > ps.MaxDegPerFrame:
		f.Reason = ReasonMoving
	case sizeJump(st.areas, f.Area, ps.SizeFactor):
		f.Reason = ReasonSizeJump
	case f.Sharpness < ps.MinSharpness:
		f.Reason = ReasonBlurry
	}
	if f.Reason != ReasonCutOff && f.Reason != ReasonNotAlone {
		st.areas = append(st.areas, f.Area)
	}

	j, t := st.j, st.t
	j.Seen++
	j.Updated = obs.At
	if len(t.Frames) >= journeyMaxFrames {
		t.Unlisted++
		if f.Reason != "" {
			j.Reasons[f.Reason]++
		}
		return nil
	}
	n := len(t.Frames)
	var saves []save
	if st.thumbs < journeyMaxThumbs {
		// Named by channel too: a journey's tracks share its folder.
		f.Thumb = fmt.Sprintf("t-%s%04d.jpg", t.Channel, n)
		st.thumbs++
		saves = append(saves, save{path: filepath.Join(r.dir, j.ID, f.Thumb), img: img, rect: crop, max: journeyThumbPx})
	}
	if f.Reason == "" {
		// Keep the sharpest in each stretch of travel.
		bin := int(math.Floor(st.travel / ps.KeepEveryDeg))
		if k, ok := st.bins[bin]; ok {
			if f.Sharpness > t.Frames[k].Sharpness {
				t.Frames[k].Kept, t.Frames[k].Reason = false, ReasonSharper
				st.bins[bin] = n
				f.Kept = true
			} else {
				f.Reason = ReasonSharper
			}
		} else {
			st.bins[bin] = n
			f.Kept = true
		}
	}
	if f.Kept {
		f.Crop = fmt.Sprintf("k-%s%04d.jpg", t.Channel, n)
		saves = append(saves, save{path: filepath.Join(r.dir, j.ID, f.Crop), img: img, rect: crop, max: ps.CropMaxPx})
	}
	t.Frames = append(t.Frames, f)
	if f.Kept && len(st.bins) > ps.MaxKept {
		// Too many: let go of the least sharp.
		worstBin, worst := 0, -1
		for b, k := range st.bins {
			if worst < 0 || t.Frames[k].Sharpness < t.Frames[worst].Sharpness {
				worstBin, worst = b, k
			}
		}
		t.Frames[worst].Kept, t.Frames[worst].Reason = false, ReasonOverMax
		delete(st.bins, worstBin)
	}
	r.tally(st)
	return saves
}

// tally recounts the journey's kept pictures and reasons from its frames.
func (r *JourneyRecorder) tally(st *journeyState) {
	j := st.j
	kept, reasons := 0, map[string]int{}
	best := -1.0
	for _, t := range j.Tracks {
		for _, f := range t.Frames {
			if f.Kept {
				kept++
				if f.Sharpness > best {
					best, j.Thumb = f.Sharpness, f.Crop
				}
			} else if f.Reason != "" {
				reasons[f.Reason]++
			}
		}
	}
	for reason, n := range j.Reasons { // the unlisted frames' reasons
		if _, listed := reasons[reason]; !listed && n > 0 {
			reasons[reason] = n
		}
	}
	j.Kept, j.Reasons = kept, reasons
}

func (r *JourneyRecorder) moment(st *journeyState, label string, at time.Time, img image.Image, boxes []vision.Box, piece int) save {
	name := "m-" + strings.ToLower(strings.ReplaceAll(label, " ", ""))
	name += fmt.Sprintf("%s%d.jpg", st.t.Channel, len(st.t.Moments))
	st.t.Moments = append(st.t.Moments, JourneyMoment{Label: label, At: at, Image: name, Boxes: append([]vision.Box(nil), boxes...), Piece: piece})
	return save{path: filepath.Join(r.dir, st.j.ID, name), img: img, rect: img.Bounds(), max: journeyMomentPx}
}

// finish closes a track that's gone.
func (r *JourneyRecorder) finish(st *journeyState) []save {
	t := st.t
	t.Ended = st.lastSeen
	t.EndReason = "lost from view"
	if n := len(t.Frames); n > 0 && t.Frames[n-1].Zone == vision.ZoneExit {
		t.EndReason = "left by the exit"
	}
	st.j.Done = true
	var saves []save
	if st.last != nil {
		saves = append(saves, r.moment(st, "Last seen", st.lastSeen, st.last, st.lastBoxes, st.lastPiece))
	}
	r.recent = append(r.recent, st.j)
	return saves
}

// prune drops the oldest finished journeys past keep, returning their ids.
func (r *JourneyRecorder) prune(keep int) []string {
	var ids []string
	for len(r.recent) > keep {
		ids = append(ids, r.recent[0].ID)
		r.recent = r.recent[1:]
	}
	return ids
}

// List is the newest journeys first, those still riding included.
func (r *JourneyRecorder) List(limit int) []JourneySummary {
	r.mu.Lock()
	defer r.mu.Unlock()
	all := make([]*Journey, 0, len(r.open)+len(r.recent))
	for _, st := range r.open {
		all = append(all, st.j)
	}
	all = append(all, r.recent...)
	sort.Slice(all, func(a, b int) bool { return all[a].Started.After(all[b].Started) })
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	out := make([]JourneySummary, len(all))
	for i, j := range all {
		s := JourneySummary{ID: j.ID, Started: j.Started, Updated: j.Updated, Done: j.Done, Seen: j.Seen, Kept: j.Kept, Reasons: copyCounts(j.Reasons), Thumb: j.Thumb, PieceID: j.PieceID}
		for _, t := range j.Tracks {
			s.Channels = append(s.Channels, t.Channel)
			s.EndReason = t.EndReason
		}
		out[i] = s
	}
	return out
}

// Get is one journey as JSON.
func (r *JourneyRecorder) Get(id string) ([]byte, error) {
	if !validJourney.MatchString(id) {
		return nil, ErrNoJourney
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, st := range r.open {
		if st.j.ID == id {
			return json.Marshal(st.j)
		}
	}
	for _, j := range r.recent {
		if j.ID == id {
			return json.Marshal(j)
		}
	}
	return nil, ErrNoJourney
}

// File is a journey's picture's path on disk, checked so a request can't reach
// outside the journeys folder.
func (r *JourneyRecorder) File(id, name string) (string, error) {
	if !validJourney.MatchString(id) || !validJourneyF.MatchString(name) {
		return "", ErrNoJourney
	}
	return filepath.Join(r.dir, id, name), nil
}

// load reads the saved journeys, oldest first.
func (r *JourneyRecorder) load() {
	paths, _ := filepath.Glob(filepath.Join(r.dir, "*", "journey.json"))
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var j Journey
		if json.Unmarshal(raw, &j) == nil && j.ID != "" {
			r.recent = append(r.recent, &j)
		}
	}
	sort.Slice(r.recent, func(a, b int) bool { return r.recent[a].Started.Before(r.recent[b].Started) })
}

// moveJourneyFiles moves a journey's pictures into the journey it turned out to carry
// on; their names already say which channel they're from, so none collide.
func moveJourneyFiles(from, to string) {
	entries, err := os.ReadDir(from)
	if err != nil {
		return
	}
	if err := os.MkdirAll(to, 0o755); err != nil {
		slog.Warn("journey pictures not moved", "from", from, "error", err)
		return
	}
	for _, e := range entries {
		if err := os.Rename(filepath.Join(from, e.Name()), filepath.Join(to, e.Name())); err != nil {
			slog.Warn("journey picture not moved", "name", e.Name(), "error", err)
		}
	}
	_ = os.Remove(from) // only if empty
}

func copyCounts(m map[string]int) map[string]int {
	out := make(map[string]int, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// cutOff: the box touches the frame's edge or reaches past the ring, so part of the
// piece is out of the picture or out of the channel.
func cutOff(b vision.Box, z vision.ChannelZones, bounds image.Rectangle) bool {
	if b.X1 <= bounds.Min.X+frameEdgePx || b.Y1 <= bounds.Min.Y+frameEdgePx ||
		b.X2 >= bounds.Max.X-frameEdgePx || b.Y2 >= bounds.Max.Y-frameEdgePx {
		return true
	}
	c := z.Ring.Center
	for _, p := range [4][2]int{{b.X1, b.Y1}, {b.X2, b.Y1}, {b.X1, b.Y2}, {b.X2, b.Y2}} {
		if math.Hypot(float64(p[0])-c.X, float64(p[1])-c.Y) > z.Ring.OuterRadius {
			return true
		}
	}
	// The box's nearest point to the center against the inner edge.
	nx := math.Max(float64(b.X1), math.Min(c.X, float64(b.X2)))
	ny := math.Max(float64(b.Y1), math.Min(c.Y, float64(b.Y2)))
	return math.Hypot(nx-c.X, ny-c.Y) < z.Ring.InnerRadius
}

// notAlone: another piece's box comes within margin pixels.
func notAlone(obs vision.Observation, i, margin int) bool {
	b := obs.Pieces[i].Box
	grown := image.Rect(b.X1-margin, b.Y1-margin, b.X2+margin, b.Y2+margin)
	for k, o := range obs.Pieces {
		if k != i && grown.Overlaps(image.Rect(o.Box.X1, o.Box.Y1, o.Box.X2, o.Box.Y2)) {
			return true
		}
	}
	return false
}

// sizeJump: the area is more than factor off the track's usual (median) area.
func sizeJump(areas []int, area int, factor float64) bool {
	if len(areas) < 3 {
		return false
	}
	sorted := append([]int(nil), areas...)
	sort.Ints(sorted)
	median := float64(sorted[len(sorted)/2])
	a := float64(area)
	return a > median*factor || a < median/factor
}
