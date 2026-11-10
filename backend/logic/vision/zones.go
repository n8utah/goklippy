// Package vision turns camera frames of a rotating channel into what the sorter acts
// on: which zones hold a piece, and where each piece is. It has no camera code; frames
// come in as image.Image. After Sorter V2's zones/ and perception/.
//
// Angles are image angles in degrees around the ring's center: 0° points right and
// angles grow clockwise on screen (y points down). Pieces travel forward, which is
// increasing angle.
package vision

import "math"

// Point is a pixel position.
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Box is a detection in pixels: X1,Y1 top-left, X2,Y2 bottom-right (exclusive).
type Box struct {
	X1 int `json:"x1"`
	Y1 int `json:"y1"`
	X2 int `json:"x2"`
	Y2 int `json:"y2"`
}

func (b Box) Center() Point {
	return Point{X: float64(b.X1+b.X2) / 2, Y: float64(b.Y1+b.Y2) / 2}
}

func (b Box) Area() int {
	return max(0, b.X2-b.X1) * max(0, b.Y2-b.Y1)
}

// samples are the box's corners, edge middles and center: enough to tell when a box
// straddles a zone edge without testing every pixel.
func (b Box) samples() [9]Point {
	x1, y1, x2, y2 := float64(b.X1), float64(b.Y1), float64(b.X2), float64(b.Y2)
	mx, my := (x1+x2)/2, (y1+y2)/2
	return [9]Point{{x1, y1}, {x2, y1}, {x1, y2}, {x2, y2}, {mx, y1}, {mx, y2}, {x1, my}, {x2, my}, {mx, my}}
}

// NormalizeAngle puts an angle in [0, 360).
func NormalizeAngle(a float64) float64 {
	a = math.Mod(a, 360)
	if a < 0 {
		a += 360
	}
	return a
}

// ForwardDeg is how far forward from one angle to another, in [0, 360).
func ForwardDeg(from, to float64) float64 {
	return NormalizeAngle(to - from)
}

// Span is an arc taken forward from Start to End: a piece meets Start first.
type Span struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

// Width is the span's angle; a span whose ends meet is the whole circle.
func (s Span) Width() float64 {
	if w := ForwardDeg(s.Start, s.End); w > 0 {
		return w
	}
	return 360
}

func (s Span) Contains(angle float64) bool {
	return ForwardDeg(s.Start, angle) < s.Width()
}

// Ring is the channel as the camera sees it: the band from InnerRadius to
// OuterRadius around Center. Over the exit's angles the channel ends at ExitRadius;
// past it the camera sees the next channel.
type Ring struct {
	Center      Point   `json:"center"`
	InnerRadius float64 `json:"inner_radius"`
	OuterRadius float64 `json:"outer_radius"`
	ExitRadius  float64 `json:"exit_radius"`
}

// Zone names a part of a channel.
type Zone string

const (
	ZoneNone    Zone = ""
	ZoneDrop    Zone = "drop"    // where pieces from the channel before land
	ZonePrecise Zone = "precise" // a sliver just before the exit, for metering
	ZoneExit    Zone = "exit"    // where pieces leave through the wall
)

// ChannelZones is one channel's ring and zones, drawn on a frame of FrameW×FrameH.
type ChannelZones struct {
	FrameW int  `json:"frame_w"`
	FrameH int  `json:"frame_h"`
	Ring   Ring `json:"ring"`
	Drop   Span `json:"drop"`
	Exit   Span `json:"exit"`
	// PreciseWidth is how many degrees before the exit the precise zone starts
	// (0: no precise zone).
	PreciseWidth float64 `json:"precise_width"`
}

// Precise is the precise zone, if it has one.
func (z ChannelZones) Precise() (Span, bool) {
	if z.PreciseWidth <= 0 {
		return Span{}, false
	}
	w := math.Min(z.PreciseWidth, 120)
	return Span{Start: NormalizeAngle(z.Exit.Start - w), End: z.Exit.Start}, true
}

// Travel is the arc pieces ride: from the drop's start to the exit's end.
func (z ChannelZones) Travel() Span {
	return Span{Start: z.Drop.Start, End: z.Exit.End}
}

// AngleOf is a point's angle around the ring's center.
func (z ChannelZones) AngleOf(p Point) float64 {
	return NormalizeAngle(math.Atan2(p.Y-z.Ring.Center.Y, p.X-z.Ring.Center.X) * 180 / math.Pi)
}

func (z ChannelZones) radiusOf(p Point) float64 {
	return math.Hypot(p.X-z.Ring.Center.X, p.Y-z.Ring.Center.Y)
}

// OnChannel reports whether a point is on the band pieces ride.
func (z ChannelZones) OnChannel(p Point) bool {
	r := z.radiusOf(p)
	if r < z.Ring.InnerRadius || r > z.Ring.OuterRadius {
		return false
	}
	angle := z.AngleOf(p)
	if !z.Travel().Contains(angle) {
		return false
	}
	return !z.Exit.Contains(angle) || r <= z.Ring.ExitRadius
}

// ZoneAt is the zone an angle on the channel is in. The precise zone sits just
// before the exit, so it's checked first if the drop overlaps it.
func (z ChannelZones) ZoneAt(angle float64) Zone {
	if z.Exit.Contains(angle) {
		return ZoneExit
	}
	if p, ok := z.Precise(); ok && p.Contains(angle) {
		return ZonePrecise
	}
	if z.Drop.Contains(angle) {
		return ZoneDrop
	}
	return ZoneNone
}

// ForwardToExit is how far forward a point is from the exit's start: positive while
// short of it, zero or negative once in it.
func (z ChannelZones) ForwardToExit(angle float64) float64 {
	if z.Exit.Contains(angle) {
		return -ForwardDeg(z.Exit.Start, angle)
	}
	return ForwardDeg(angle, z.Exit.Start)
}

// ScaledTo is these zones for a frame of another size (a camera mode change).
func (z ChannelZones) ScaledTo(w, h int) ChannelZones {
	if z.FrameW <= 0 || z.FrameH <= 0 || (w == z.FrameW && h == z.FrameH) {
		return z
	}
	sx, sy := float64(w)/float64(z.FrameW), float64(h)/float64(z.FrameH)
	s := (sx + sy) / 2
	out := z
	out.FrameW, out.FrameH = w, h
	out.Ring = Ring{
		Center:      Point{X: z.Ring.Center.X * sx, Y: z.Ring.Center.Y * sy},
		InnerRadius: z.Ring.InnerRadius * s,
		OuterRadius: z.Ring.OuterRadius * s,
		ExitRadius:  z.Ring.ExitRadius * s,
	}
	return out
}

// Valid reports whether the zones can describe a channel.
func (z ChannelZones) Valid() bool {
	return z.FrameW > 0 && z.FrameH > 0 && z.Ring.OuterRadius > z.Ring.InnerRadius && z.Ring.InnerRadius >= 0 &&
		z.Drop.Width() < 360 && z.Exit.Width() < 360
}
