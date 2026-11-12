package sorter

import (
	"context"
	"image"
	"image/color"
	"math"
	"math/rand/v2"
	"sync"
	"time"

	"goklippy/backend/logic/brickognize"
	"goklippy/backend/logic/vision"
)

// SimWorld is pretend LEGO for SimHardware: pieces ride the channels as the motors
// turn them and fall from one channel to the next. It is the cameras (Observation,
// Frame) and Brickognize (Identify) for the machine, so the whole sort runs with no
// hardware, for tests and for trying the UI.
//
// Every channel uses the same zones (SimZones). C1 is a hopper: each bulk pulse drops
// the next part of Supply into C2's drop zone. A piece leaves C2 or C3 when it passes
// the middle of the exit and lands in the next channel's drop zone; it leaves C4 the
// same way, into the chute, and the world notes where the chute pointed then.
type SimWorld struct {
	hw       *SimHardware
	settings func() MachineSettings

	mu     sync.Mutex
	supply []SimPart
	// Random, when set, makes a part for each bulk pulse once Supply runs out.
	Random   bool
	rng      *rand.Rand
	onRotor  map[Channel][]*simPiece // pieces riding each channel
	landedC4 []SimPart               // parts in the order they landed on C4
	asked    int                     // how many of them Brickognize has named
	falls    []SimFall
	frameAt  time.Time
	ignores  spotIgnores // stuck pieces worked around, as Perception does
}

// SimPart is a part the world can hold.
type SimPart struct {
	ID       string
	Name     string
	Category string
	ColorID  string
	Color    string
	RGB      color.RGBA
	// Unknown parts get no match from the pretend Brickognize.
	Unknown bool
	// HeldOn makes the piece stick on that channel: turning it doesn't carry the
	// piece until the channel has been shaken HeldShakes times. A huge HeldShakes
	// never lets go.
	HeldOn     Channel
	HeldShakes int
	// NoFall: on C4 it stops in the exit instead of falling, until C4 is shaken.
	NoFall bool
}

// SimFall is a piece dropping off C4, with where it went.
type SimFall struct {
	Part       SimPart
	Layer      int     // the open layer servo (from 1), 0 for none: the bucket
	ChuteAngle float64 // where the chute pointed
	ChuteHomed bool
	At         time.Time
}

type simPiece struct {
	part  SimPart
	angle float64
	held  int // shakes still needed to free it
}

func newSimPiece(part SimPart, ch Channel) *simPiece {
	p := &simPiece{part: part, angle: simLandAngle}
	if part.HeldOn == ch {
		p.held = part.HeldShakes
	}
	return p
}

// SimZones are the zones every simulated channel uses, in a 640×480 frame.
func SimZones() vision.ChannelZones {
	return vision.ChannelZones{
		FrameW: 640, FrameH: 480,
		Ring:         vision.Ring{Center: vision.Point{X: 320, Y: 240}, InnerRadius: 100, OuterRadius: 200, ExitRadius: 195},
		Drop:         vision.Span{Start: 200, End: 260},
		Exit:         vision.Span{Start: 330, End: 350},
		PreciseWidth: 15,
	}
}

const (
	simLandAngle   = 215.0
	simExitAngle   = 340.0
	simPieceSize   = 26
	simPieceRadius = 150.0
)

// SimCatalog is the parts the random supply draws from.
var SimCatalog = []SimPart{
	{ID: "3001", Name: "Brick 2 x 4", Category: "Brick", ColorID: "5", Color: "Red", RGB: color.RGBA{200, 30, 30, 255}},
	{ID: "3003", Name: "Brick 2 x 2", Category: "Brick", ColorID: "3", Color: "Yellow", RGB: color.RGBA{230, 200, 30, 255}},
	{ID: "3020", Name: "Plate 2 x 4", Category: "Plate", ColorID: "7", Color: "Blue", RGB: color.RGBA{30, 60, 200, 255}},
	{ID: "3023", Name: "Plate 1 x 2", Category: "Plate", ColorID: "1", Color: "White", RGB: color.RGBA{240, 240, 240, 255}},
	{ID: "3068b", Name: "Tile 2 x 2", Category: "Tile", ColorID: "11", Color: "Black", RGB: color.RGBA{20, 20, 20, 255}},
	{ID: "x9999", Name: "Mystery", RGB: color.RGBA{120, 200, 120, 255}, Unknown: true},
}

// NewSimWorld watches the simulated hardware's moves.
func NewSimWorld(hw *SimHardware, settings func() MachineSettings, supply []SimPart) *SimWorld {
	w := &SimWorld{
		hw: hw, settings: settings, supply: supply,
		rng:     rand.New(rand.NewPCG(1, 2)),
		onRotor: map[Channel][]*simPiece{},
		ignores: spotIgnores{},
	}
	hw.OnMove(w.moved)
	hw.OnShake(w.shaken)
	return w
}

// Falls lists the pieces that have dropped off C4 so far.
func (w *SimWorld) Falls() []SimFall {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]SimFall(nil), w.falls...)
}

// OnChannels counts the pieces still riding C2–C4.
func (w *SimWorld) OnChannels() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	n := 0
	for _, ps := range w.onRotor {
		n += len(ps)
	}
	return n
}

// moved advances the pieces on a channel that turned.
func (w *SimWorld) moved(ch Channel, from, to float64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	deg := to - from
	switch ch {
	case ChannelC1:
		if deg > 0 {
			w.feedBulk()
		}
	case ChannelC2, ChannelC3, ChannelC4:
		w.turn(ch, deg)
	}
}

func (w *SimWorld) feedBulk() {
	var part SimPart
	switch {
	case len(w.supply) > 0:
		part, w.supply = w.supply[0], w.supply[1:]
	case w.Random:
		part = SimCatalog[w.rng.IntN(len(SimCatalog))]
	default:
		return
	}
	w.onRotor[ChannelC2] = append(w.onRotor[ChannelC2], newSimPiece(part, ChannelC2))
}

// shaken frees a little the pieces held on a shaken channel.
func (w *SimWorld) shaken(ch Channel) {
	w.mu.Lock()
	defer w.mu.Unlock()
	var stay []*simPiece
	for _, p := range w.onRotor[ch] {
		if p.held > 0 {
			p.held--
		}
		if ch == ChannelC4 && p.part.NoFall && p.angle == simExitAngle {
			w.fall(ch, p) // shaken off the exit
			continue
		}
		stay = append(stay, p)
	}
	w.onRotor[ch] = stay
}

// turn moves a channel's pieces; those passing the exit's middle fall to the next.
func (w *SimWorld) turn(ch Channel, deg float64) {
	var stay []*simPiece
	for _, p := range w.onRotor[ch] {
		if p.held > 0 || (ch == ChannelC4 && p.part.NoFall && p.angle == simExitAngle) {
			stay = append(stay, p) // held, or hanging in the exit: the channel turns under it
			continue
		}
		before := p.angle
		p.angle += deg
		if before < simExitAngle && p.angle >= simExitAngle {
			if ch == ChannelC4 && p.part.NoFall {
				p.angle = simExitAngle // hangs in the exit
				stay = append(stay, p)
				continue
			}
			w.fall(ch, p)
			continue
		}
		p.angle = math.Mod(p.angle, 360)
		stay = append(stay, p)
	}
	w.onRotor[ch] = stay
}

func (w *SimWorld) fall(ch Channel, p *simPiece) {
	switch ch {
	case ChannelC2:
		w.onRotor[ChannelC3] = append(w.onRotor[ChannelC3], newSimPiece(p.part, ChannelC3))
	case ChannelC3:
		w.onRotor[ChannelC4] = append(w.onRotor[ChannelC4], newSimPiece(p.part, ChannelC4))
		w.landedC4 = append(w.landedC4, p.part)
	case ChannelC4:
		status := w.hw.ChuteStatus()
		f := SimFall{Part: p.part, Layer: w.hw.Layer() + 1, ChuteHomed: status.Homed, At: time.Now()}
		if status.Angle != nil {
			f.ChuteAngle = *status.Angle
		}
		w.falls = append(w.falls, f)
	}
}

func simBox(angle float64) vision.Box {
	z := SimZones()
	rad := angle * math.Pi / 180
	cx := z.Ring.Center.X + simPieceRadius*math.Cos(rad)
	cy := z.Ring.Center.Y + simPieceRadius*math.Sin(rad)
	h := simPieceSize / 2
	return vision.Box{X1: int(cx) - h, Y1: int(cy) - h, X2: int(cx) + h, Y2: int(cy) + h}
}

// Observation is what a camera over the channel would report now.
func (w *SimWorld) Observation(ch Channel) vision.Observation {
	if ch != ChannelC2 && ch != ChannelC3 && ch != ChannelC4 {
		return vision.Observation{}
	}
	now := time.Now()
	w.mu.Lock()
	var boxes []vision.Box
	for _, p := range w.onRotor[ch] {
		boxes = append(boxes, simBox(p.angle))
	}
	boxes, ignored := w.ignores.filter(ch, boxes, simMergeGap, now)
	w.mu.Unlock()
	obs := vision.Observe(SimZones(), boxes, simMergeGap, now)
	obs.Ignored = ignored
	return obs
}

// simMergeGap is how close the simulated camera's boxes join into one piece.
const simMergeGap = 2

// IgnoreSpot leaves a stuck piece out of ch's observations.
func (w *SimWorld) IgnoreSpot(ch Channel, box vision.Box, area int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.ignores.add(ch, box, area, time.Now())
}

// ForgetIgnored watches all of ch again.
func (w *SimWorld) ForgetIgnored(ch Channel) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.ignores, ch)
}

// Stuck counts the pieces on ch that don't ride with it (HeldOn), for tests.
func (w *SimWorld) Stuck(ch Channel) int {
	w.mu.Lock()
	defer w.mu.Unlock()
	n := 0
	for _, p := range w.onRotor[ch] {
		if p.held > 0 {
			n++
		}
	}
	return n
}

// Snapshot is a channel's frame and its pieces' boxes at the same moment.
func (w *SimWorld) Snapshot(ch Channel) (image.Image, []vision.Box, time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()
	var boxes []vision.Box
	for _, p := range w.onRotor[ch] {
		boxes = append(boxes, simBox(p.angle))
	}
	img, at := w.frameLocked(ch)
	return img, boxes, at
}

// Frame draws C4 (or any channel) with its pieces as colored squares.
func (w *SimWorld) Frame(ch Channel) (image.Image, time.Time, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	img, at := w.frameLocked(ch)
	return img, at, true
}

func (w *SimWorld) frameLocked(ch Channel) (image.Image, time.Time) {
	img := image.NewRGBA(image.Rect(0, 0, 640, 480))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = 128, 128, 128, 255
	}
	for _, p := range w.onRotor[ch] {
		b := simBox(p.angle)
		for y := max(0, b.Y1); y < min(480, b.Y2); y++ {
			for x := max(0, b.X1); x < min(640, b.X2); x++ {
				img.SetRGBA(x, y, p.part.RGB)
			}
		}
	}
	// Each frame is a new moment, even when called twice in one tick.
	now := time.Now()
	if !now.After(w.frameAt) {
		now = w.frameAt.Add(time.Microsecond)
	}
	w.frameAt = now
	return img, now
}

// Identify names the next piece that landed on C4, as Brickognize would.
func (w *SimWorld) Identify(_ context.Context, jpegs [][]byte) (brickognize.Result, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(jpegs) == 0 {
		return brickognize.Result{}, brickognize.ErrNoImages
	}
	if w.asked >= len(w.landedC4) {
		return brickognize.Result{Items: []brickognize.Item{}, Colors: []brickognize.Color{}}, nil
	}
	part := w.landedC4[w.asked]
	w.asked++
	res := brickognize.Result{ListingID: "sim", Items: []brickognize.Item{}, Colors: []brickognize.Color{}}
	if part.Unknown {
		return res, nil
	}
	res.Items = append(res.Items, brickognize.Item{ID: part.ID, Name: part.Name, Category: part.Category, Score: 0.95})
	res.Colors = append(res.Colors, brickognize.Color{ID: part.ColorID, Name: part.Color, Score: 0.9})
	return res, nil
}
