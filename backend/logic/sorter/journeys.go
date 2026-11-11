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
}

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
	recent []*Journey // finished, oldest first
}

type trackKey struct {
	ch Channel
	id int
}

type journeyState struct {
	j         *Journey
	t         *JourneyTrack
	lastSeen  time.Time
	lastAngle float64
	travel    float64 // degrees forward since it was first seen
	frames    int     // frames seen
	areas     []int   // of frames whole and alone, for the usual size
	bins      map[int]int
	thumbs    int
	last      image.Image // the newest frame, for the last-seen moment
	lastBoxes []vision.Box
	lastPiece int
}

// NewJourneyRecorder keeps journeys under dir and loads the ones saved there.
func NewJourneyRecorder(dir string, settings func() MachineSettings) *JourneyRecorder {
	r := &JourneyRecorder{dir: dir, settings: settings, open: map[trackKey]*journeyState{}}
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
	seen := map[int]bool{}
	for i, p := range obs.Pieces {
		if p.TrackID == 0 {
			continue
		}
		seen[p.TrackID] = true
		key := trackKey{ch, p.TrackID}
		st := r.open[key]
		if st == nil {
			st = r.start(ch, p, z, img.Bounds(), now)
			r.open[key] = st
			saves = append(saves, r.moment(st, "Landed", now, img, boxes, i))
		}
		saves = append(saves, r.judge(st, obs, i, img, z, ps)...)
		st.lastSeen, st.last, st.lastBoxes, st.lastPiece = now, img, boxes, i
	}
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
		f.Thumb = fmt.Sprintf("t-%04d.jpg", n)
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
		f.Crop = fmt.Sprintf("k-%04d.jpg", n)
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
	name += fmt.Sprintf("%d.jpg", len(st.t.Moments))
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
		s := JourneySummary{ID: j.ID, Started: j.Started, Updated: j.Updated, Done: j.Done, Seen: j.Seen, Kept: j.Kept, Reasons: copyCounts(j.Reasons), Thumb: j.Thumb}
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
