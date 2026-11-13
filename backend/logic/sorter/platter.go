package sorter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"log/slog"
	"time"

	"goklippy/backend/logic/brickognize"
	"goklippy/backend/logic/sorter/profile"
	"goklippy/backend/logic/vision"
)

// The classification platter (C4): equal sectors that only turn forward, one sector
// per index. C3 drops a piece into the sector under it (the drop sector); the camera
// sees it land, photographs it, and Brickognize names it while the platter carries
// it round. ExitSector indexes after landing it falls off into the chute, so before
// that index the chute is aimed and the layer chosen for it.

// VisionSource is what the platter and feeder read from the cameras.
type VisionSource interface {
	Observation(ch Channel) vision.Observation
	// Frame is the camera's newest frame and when it was taken.
	Frame(ch Channel) (image.Image, time.Time, bool)
}

// PlatterDeps is what the platter works with.
type PlatterDeps struct {
	HW         Hardware
	Vision     VisionSource
	Identifier Identifier
	Sorting    *SortingService
	Records    *Records
	Settings   func() MachineSettings
	// RunID is the current sort run, for the piece records.
	RunID func() int64
	// OnStall is told when pieces aboard have made no progress for StallMs. The
	// machine tries to clear the channel, then asks the operator.
	OnStall func(aboard int)
	// OnDidntFall is told when a piece is still at the exit after the index that
	// should have dropped it.
	OnDidntFall func()
	// OnLanded is told when a piece lands and gets its record, with the frame's time.
	OnLanded func(pieceID string, at time.Time)
	// Upstream gives up to max pictures of the piece from C2 and C3 (its journey), to
	// send Brickognize alongside C4's. Nil: C4's alone.
	Upstream func(pieceID string, max int) [][]byte
}

type slotState int

const (
	slotSettling    slotState = iota // landed; waiting for it to stop bouncing
	slotCapturing                    // collecting photos
	slotIdentifying                  // photos sent to Brickognize
	slotResolved                     // knows its category (or that it has none)
)

type platterSlot struct {
	piece     Piece
	state     slotState
	landedAt  time.Time
	box       vision.Box // where it lay in the drop zone
	shots     []platterShot
	lastFrame time.Time
	sentAt    time.Time
	// Settling: the newest observation looked at, and how many frames in a row the
	// piece's box has stayed put. Photos start once it's still.
	lastObs     time.Time
	stillFrames int
	// The bin it was routed to, once it is next to fall.
	routed bool
	choice BinChoice
}

type identResult struct {
	pieceID  string
	result   brickognize.Result
	err      error
	strategy string // which request's answer it is (platter_identify.go)
}

// Platter runs C4. Step it from the control loop; it never blocks.
type Platter struct {
	deps PlatterDeps

	slots       []*platterSlot // slots[0] is under the drop; [i] is i indexes on
	indexDone   <-chan error   // set while an index is turning
	settleUntil time.Time
	freeSince   time.Time // when the drop sector last became free
	// The chute and doors for the piece about to fall.
	aimDone      <-chan error
	doorsSettled time.Time
	aimedFor     string // piece id
	idents       chan identResult
	lastProgress time.Time
	stallRaised  bool
	recent       []Piece // last pieces delivered, newest first
	delivered    int     // pieces off C4 (into a bin or the bucket) since it was made
	// checkFall: after a drop, a settled frame must show the exit clear by fallBy
	// (Platter.FallWaitMs after it settled): a piece may still be sliding off.
	checkFall bool
	fallBy    time.Time
	// severalUntil: C3 just dropped more than one piece at once; a landing before
	// this is a double (ExpectSeveral).
	severalUntil time.Time
}

const (
	recentPieces = 8
	cropMargin   = 0.35
)

func NewPlatter(deps PlatterDeps) *Platter {
	p := &Platter{deps: deps, idents: make(chan identResult, 16)}
	p.Reset(time.Now())
	return p
}

// Reset empties the platter's memory, as after a stop. Pieces it was carrying are
// recorded as lost.
func (p *Platter) Reset(now time.Time) {
	for _, s := range p.slots {
		if s != nil && p.deps.Records != nil {
			s.piece.Status = PieceLost
			s.piece.AddProblem(ProblemLost, "The sort stopped while it was on the classification channel.", now)
			p.deps.Records.SavePiece(s.piece)
		}
	}
	n := max(2, p.deps.Settings().Platter.Sectors)
	p.slots = make([]*platterSlot, n)
	p.indexDone, p.aimDone, p.aimedFor = nil, nil, ""
	p.settleUntil, p.freeSince, p.lastProgress = now, now, now
	p.stallRaised = false
}

// Ready is the gate C3 reads: the drop sector is free and still, and (with
// Platter.ClearExitFirst) the platter has room. When C3 waits, the feeder's
// back-pressure pauses the channels before it.
func (p *Platter) Ready(now time.Time) bool {
	return p.indexDone == nil && p.slots[0] == nil && !now.Before(p.settleUntil) &&
		p.roomForAnother(p.deps.Settings())
}

// roomForAnother: at most two pieces on the platter, the other one in the exit slot
// (it falls on the next index) and still being identified. Once it's identified it's
// about to drop, and a piece landing then would hold the platter still while it's
// photographed, keeping the finished one from dropping; so nothing lands until it has.
func (p *Platter) roomForAnother(settings MachineSettings) bool {
	if !settings.Platter.ClearExitFirst {
		return true
	}
	exit := p.exitIndex(settings) - 1
	for i, s := range p.slots {
		if s == nil {
			continue
		}
		if i != exit || s.state >= slotResolved {
			return false
		}
	}
	return true
}

// Busy reports the platter turning, settling or with a piece in its drop sector:
// C3 keeps its lead back from the exit (FeederInput.PlatterBusy).
func (p *Platter) Busy(now time.Time) bool {
	return p.indexDone != nil || p.slots[0] != nil || now.Before(p.settleUntil)
}

// TimeState is what C4 is doing now, for the time spent (time_spent.go): turning;
// busy photographing, waiting on Brickognize or for the chute; waiting to turn with
// pieces aboard; or empty.
func (p *Platter) TimeState(now time.Time) string {
	if p.indexDone != nil || p.aimDone != nil {
		if p.indexDone != nil {
			return TimeMoving
		}
		return TimeBusy // the chute is turning for the next piece
	}
	aboard := false
	for _, s := range p.slots {
		if s == nil {
			continue
		}
		aboard = true
		if s.state < slotResolved {
			return TimeBusy // landing, photographed or being identified
		}
	}
	switch {
	case !aboard:
		return TimeEmpty
	case now.Before(p.settleUntil) || now.Before(p.doorsSettled):
		return TimeBusy
	default:
		return TimeWaiting
	}
}

// Doing is what C4 is doing now in a few words, for the status on Run › Sort.
func (p *Platter) Doing(now time.Time) string {
	if p.indexDone != nil {
		return "turning to the next sector"
	}
	if p.checkFall && now.Before(p.fallBy) {
		return "watching a piece fall off"
	}
	if p.aimDone != nil {
		return "waiting for the chute to aim"
	}
	if s := p.slots[0]; s != nil {
		switch s.state {
		case slotSettling:
			return "a piece landed: waiting for it to lie still"
		case slotCapturing:
			return "photographing a piece"
		}
	}
	identifying, aboard := 0, 0
	for _, s := range p.slots {
		if s == nil {
			continue
		}
		aboard++
		if s.state == slotIdentifying {
			identifying++
		}
	}
	switch {
	case identifying > 0:
		return "asking Brickognize what it is"
	case aboard == 0:
		return "empty: waiting for C3 to drop a piece"
	}
	if s := p.slots[p.exitIndex(p.deps.Settings())-1]; s != nil && s.routed {
		return "about to drop a piece"
	}
	return "waiting for the next piece"
}

// Aboard lists the pieces on the platter, nearest the exit first.
func (p *Platter) Aboard() []Piece {
	var out []Piece
	for i := len(p.slots) - 1; i >= 0; i-- {
		if s := p.slots[i]; s != nil {
			out = append(out, s.piece.Clone())
		}
	}
	return out
}

// Delivered counts the pieces C4 has sent off (into a bin or the bucket), ever.
func (p *Platter) Delivered() int { return p.delivered }

// Recent is the last pieces delivered, newest first.
func (p *Platter) Recent() []Piece {
	return append([]Piece(nil), p.recent...)
}

// Step runs one control-loop tick.
func (p *Platter) Step(now time.Time) {
	settings := p.deps.Settings()
	ps := settings.Platter
	p.drainIdents(now)

	if p.indexDone != nil {
		select {
		case err := <-p.indexDone:
			p.indexDone = nil
			if err != nil {
				slog.Warn("platter index failed", "error", err)
			}
			p.finishIndex(now, ps)
		default:
			return
		}
	}
	if now.Before(p.settleUntil) {
		return
	}

	obs := p.deps.Vision.Observation(ChannelC4)
	p.verifyFall(now, obs)
	p.watchDrop(now, obs, ps)
	p.capture(now, obs, ps)
	p.prepareExit(now, settings)
	p.maybeIndex(now, settings)
	p.watchdog(now, ps)
}

// watchDrop notices a piece landing in the free drop sector.
func (p *Platter) watchDrop(now time.Time, obs vision.Observation, ps PlatterSettings) {
	if p.slots[0] != nil || !obs.Valid || obs.At.Before(p.settleUntil) || !obs.InDrop {
		return
	}
	var inDrop []vision.PieceObs
	for _, o := range obs.Pieces {
		if o.Zone == vision.ZoneDrop {
			inDrop = append(inDrop, o)
		}
	}
	if len(inDrop) == 0 {
		return
	}
	s := &platterSlot{
		piece:    Piece{ID: NewPieceID(), Status: PieceTracking, SeenAt: now, Problems: []Problem{}},
		state:    slotSettling,
		landedAt: now,
		box:      inDrop[0].Box,
	}
	if p.deps.RunID != nil {
		s.piece.RunID = p.deps.RunID()
	}
	several := now.Before(p.severalUntil)
	p.severalUntil = time.Time{}
	switch {
	case len(inDrop) > 1 || touchingPieces(obs, inDrop[0].Box):
		s.piece.Status = PieceMultiDrop
		s.piece.AddProblem(ProblemDoubleFeed, "It landed with another piece, so it couldn't be photographed alone.", now)
		s.state = slotResolved
	case several:
		s.piece.Status = PieceMultiDrop
		s.piece.AddProblem(ProblemDoubleFeed, "More than one piece fell off C3 together, so it couldn't be photographed alone.", now)
		s.state = slotResolved
	}
	p.slots[0] = s
	p.progress(now)
	p.deps.Records.SavePiece(s.piece)
	slog.Info("piece landed on the platter", "piece", s.piece.ID, "multi_drop", s.piece.Status == PieceMultiDrop)
	if p.deps.OnLanded != nil && s.piece.Status != PieceMultiDrop {
		p.deps.OnLanded(s.piece.ID, obs.At)
	}
}

// touchingPieces reports a merged box made of parts that don't look like one piece:
// side by side, not stacked along one axis.
func touchingPieces(obs vision.Observation, box vision.Box) bool {
	for _, m := range obs.Merged {
		if m.Box != box || len(m.Parts) < 2 {
			continue
		}
		for i, a := range m.Parts {
			for _, b := range m.Parts[i+1:] {
				if !linedUp(a, b) {
					return true
				}
			}
		}
	}
	return false
}

// linedUp: the parts of one object stacked, as a minifigure's head over its torso: one
// box's span across x or y lies mostly inside the other's, and one is much the
// smaller. Two pieces of about the same size end to end (a plate against a brick)
// line up too, but are two pieces.
func linedUp(a, b vision.Box) bool {
	overlap := func(a1, a2, b1, b2 int) float64 {
		span := min(a2-a1, b2-b1)
		shared := min(a2, b2) - max(a1, b1)
		if span <= 0 {
			return 0
		}
		return float64(shared) / float64(span)
	}
	small, big := min(a.Area(), b.Area()), max(a.Area(), b.Area())
	if big <= 0 || float64(small) > partOfOneRatio*float64(big) {
		return false
	}
	return overlap(a.X1, a.X2, b.X1, b.X2) >= 0.7 || overlap(a.Y1, a.Y2, b.Y1, b.Y2) >= 0.7
}

// partOfOneRatio: a lined-up part at most this share of the other's area can be part of
// the same object.
const partOfOneRatio = 0.5

const (
	// stillFramesNeeded: frames in a row the landed piece's box must stay put before
	// it's photographed, so no photo is of it still sliding or bouncing.
	stillFramesNeeded = 2
	// settleMax: a piece still rocking after this long is photographed anyway.
	settleMax = 2 * time.Second
	// photoSkew: a frame and an observation (or a full-size frame) this far apart are
	// different moments, and a box from one would miss the piece in the other.
	photoSkew = 120 * time.Millisecond
)

// stillTol is how far a box may shift between frames and still count as not moving.
func stillTol(b vision.Box) float64 {
	return max(4, 0.08*boxSize(b))
}

// capture waits for the piece in the drop sector to lie still, photographs it, then
// sends the burst off. A piece that moves while being photographed (bumped) starts
// over; a second piece in the drop sector makes it a double.
func (p *Platter) capture(now time.Time, obs vision.Observation, ps PlatterSettings) {
	s := p.slots[0]
	if s == nil || s.state > slotCapturing || !obs.Valid || !obs.At.After(s.lastObs) {
		return
	}
	s.lastObs = obs.At
	if inDrop := dropPieces(obs); len(inDrop) > 1 {
		p.markDouble(s, now, "Another piece landed with it, so it couldn't be photographed alone.")
		return
	}
	lead, ok := dropPieceNear(obs, s.box)
	if !ok {
		// Not in the drop sector this frame: slid just past its edge, or briefly lost.
		// Look for it nearby anywhere; failing that, once settleMax has passed take the
		// photos where it was last seen rather than wait for it forever.
		lead, ok = pieceNear(obs, s.box, 2*boxSize(s.box))
		if !ok {
			if s.state == slotSettling && now.Sub(s.landedAt) >= settleMax {
				s.state = slotCapturing
			}
			if s.state != slotCapturing {
				return
			}
			lead = vision.PieceObs{Box: s.box}
		}
	}
	moved := centerDist(lead.Box, s.box) > stillTol(s.box) || !similarArea(lead.Box.Area(), s.box.Area())
	s.box = lead.Box
	switch s.state {
	case slotSettling:
		if moved {
			s.stillFrames = 0
		} else {
			s.stillFrames++
		}
		settled := now.Sub(s.landedAt) >= time.Duration(ps.SettleMs)*time.Millisecond
		if !settled || (s.stillFrames < stillFramesNeeded && now.Sub(s.landedAt) < settleMax) {
			return
		}
		s.state = slotCapturing
	case slotCapturing:
		if moved {
			// Bumped: these photos show it in different places; start again.
			s.shots, s.stillFrames, s.state = nil, 0, slotSettling
			return
		}
	}
	frame, at, ok := p.deps.Vision.Frame(ChannelC4)
	if !ok || !at.After(s.lastFrame) || at.Before(s.landedAt) || absDuration(at.Sub(obs.At)) > photoSkew {
		return
	}
	s.lastFrame = at
	shot := platterShot{frame: frame, box: s.box, at: at}
	if hr, ok := p.deps.Vision.(hiResSource); ok {
		// The camera's full-size frame from the same moment, when it has one.
		if jpg, fullAt, scale, ok := hr.HiRes(ChannelC4); ok && scale > 1 && absDuration(fullAt.Sub(at)) <= photoSkew {
			shot.full, shot.scale = jpg, scale
		}
	}
	s.shots = append(s.shots, shot)
	want := min(max(1, ps.BurstFrames), brickognize.MaxImages)
	if len(s.shots) < want {
		return
	}
	s.state = slotIdentifying
	s.sentAt = now
	s.piece.Status = PieceIdentifying
	p.deps.Records.SavePiece(s.piece)
	p.progress(now)
	shots, id := s.shots, s.piece.ID
	go func() {
		// Off the control loop: a full-size frame takes a while to decode, and
		// Brickognize a while to answer (platter_identify.go).
		p.idents <- p.identify(id, shots, ps)
	}()
}

// hiResSource is vision with a full-size frame next to the tracked one (Perception
// for a camera capturing larger than it's tracked at).
type hiResSource interface {
	HiRes(ch Channel) (jpg []byte, at time.Time, scale float64, ok bool)
}

// platterShot is one photo of a piece on C4: the tracked frame and the piece's box in
// it, and the full-size frame (scale full pixels per tracked one) when there is one.
type platterShot struct {
	frame image.Image
	box   vision.Box
	full  []byte
	scale float64
	at    time.Time
}

// crop is the piece cut out of the full-size frame, or else the tracked one.
func (sh platterShot) crop() ([]byte, error) {
	if len(sh.full) > 0 && sh.scale > 1 {
		img, err := jpeg.Decode(bytes.NewReader(sh.full))
		if err == nil {
			s := sh.scale
			box := vision.Box{
				X1: int(float64(sh.box.X1) * s), Y1: int(float64(sh.box.Y1) * s),
				X2: int(float64(sh.box.X2)*s + 0.5), Y2: int(float64(sh.box.Y2)*s + 0.5),
			}
			if jpg, err := cropJPEG(img, box); err == nil {
				return jpg, nil
			}
		}
		slog.Warn("the full-size photo couldn't be used; cropping the tracked one", "error", err)
	}
	return cropJPEG(sh.frame, sh.box)
}

// dropPieces are the pieces seen in the drop sector.
func dropPieces(obs vision.Observation) []vision.PieceObs {
	var out []vision.PieceObs
	for _, o := range obs.Pieces {
		if o.Zone == vision.ZoneDrop {
			out = append(out, o)
		}
	}
	return out
}

// dropPieceNear is the piece in the drop sector nearest box: the one that landed.
func dropPieceNear(obs vision.Observation, box vision.Box) (vision.PieceObs, bool) {
	var best vision.PieceObs
	found := false
	for _, o := range dropPieces(obs) {
		if !found || centerDist(o.Box, box) < centerDist(best.Box, box) {
			best, found = o, true
		}
	}
	return best, found
}

// pieceNear is the piece anywhere on C4 nearest box, within max pixels of it.
func pieceNear(obs vision.Observation, box vision.Box, max float64) (vision.PieceObs, bool) {
	var best vision.PieceObs
	found := false
	for _, o := range obs.Pieces {
		if d := centerDist(o.Box, box); d <= max && (!found || d < centerDist(best.Box, box)) {
			best, found = o, true
		}
	}
	return best, found
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

// markDouble records the piece in the drop sector as having landed with another: it
// isn't photographed, and goes to the bucket.
func (p *Platter) markDouble(s *platterSlot, now time.Time, why string) {
	if s.piece.Status == PieceMultiDrop {
		return
	}
	s.piece.Status = PieceMultiDrop
	s.piece.AddProblem(ProblemDoubleFeed, why, now)
	s.state, s.shots = slotResolved, nil
	p.deps.Records.SavePiece(s.piece)
	p.progress(now)
	slog.Info("piece landed with another", "piece", s.piece.ID)
}

// ExpectSeveral tells the platter that more than one piece just fell off C3 together:
// the next landing is a double, however it looks.
func (p *Platter) ExpectSeveral(now time.Time) {
	// C3 sees them gone a frame or so after C4 sees them land, so a piece that landed
	// just now is them.
	if s := p.slots[0]; s != nil && s.state <= slotCapturing && now.Sub(s.landedAt) < expectSeveralFor {
		p.markDouble(s, now, "More than one piece fell off C3 together, so it couldn't be photographed alone.")
		return
	}
	p.severalUntil = now.Add(expectSeveralFor)
}

// expectSeveralFor: how long after C3 drops several at once a landing counts as them.
const expectSeveralFor = 3 * time.Second

// cropJPEG cuts the box (with a margin) out of the frame.
func cropJPEG(frame image.Image, box vision.Box) ([]byte, error) {
	w, h := box.X2-box.X1, box.Y2-box.Y1
	mx, my := int(float64(w)*cropMargin)+4, int(float64(h)*cropMargin)+4
	r := image.Rect(box.X1-mx, box.Y1-my, box.X2+mx, box.Y2+my).Intersect(frame.Bounds())
	if r.Empty() {
		return nil, fmt.Errorf("the box is outside the frame")
	}
	sub, ok := frame.(interface {
		SubImage(image.Rectangle) image.Image
	})
	if !ok {
		return nil, fmt.Errorf("can't crop %T", frame)
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, sub.SubImage(r), &jpeg.Options{Quality: 90}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// drainIdents takes Brickognize's answers for pieces still aboard.
func (p *Platter) drainIdents(now time.Time) {
	for {
		select {
		case r := <-p.idents:
			for _, s := range p.slots {
				if s != nil && s.piece.ID == r.pieceID && s.state == slotIdentifying {
					applyIdentification(&s.piece, r, now)
					s.state = slotResolved
					p.deps.Records.SavePiece(s.piece)
					p.progress(now)
				}
			}
		default:
			return
		}
	}
}

func applyIdentification(piece *Piece, r identResult, now time.Time) {
	piece.IdentifiedAt = &now
	if r.err != nil {
		piece.Status = PieceIDFailed
		piece.AddProblem(ProblemIDFailed, "Identification failed: "+r.err.Error(), now)
		return
	}
	if raw, err := json.Marshal(r.result); err == nil {
		piece.Prediction = raw
	}
	item, color := r.result.Best()
	if item == nil {
		piece.Status = PieceUnidentified
		return
	}
	piece.Status = PieceIdentified
	piece.PartID, piece.PartName, piece.PartCategory, piece.PartScore = item.ID, item.Name, item.Category, item.Score
	if color != nil {
		piece.ColorID, piece.ColorName, piece.ColorScore = color.ID, color.Name, color.Score
	}
}

// prepareExit routes the piece that falls on the next index and aims the chute.
func (p *Platter) prepareExit(now time.Time, settings MachineSettings) {
	exit := p.exitIndex(settings) - 1
	s := p.slots[exit]
	if s == nil || s.routed {
		return
	}
	ps := settings.Platter
	if s.state == slotIdentifying && now.Sub(s.sentAt) >= time.Duration(ps.IdentifyTimeoutMs)*time.Millisecond {
		s.piece.Status = PieceIDFailed
		s.piece.AddProblem(ProblemIDTimeout, "Brickognize didn't answer before it reached the exit, so it went to misc.", now)
		s.state = slotResolved
	}
	if s.state < slotCapturing {
		s.piece.AddProblem(ProblemNeverPhotographed, "It reached the exit before it was photographed.", now)
		s.state = slotResolved
	}
	if s.state != slotResolved {
		return
	}

	leftOut := settings.Distributor.LeftOut
	if leftOut && (p.deps.Sorting == nil || p.deps.Sorting.Active() == nil) {
		// No chute and no profile: nothing is aimed, and nothing says where it would go.
		s.choice = BinChoice{Reason: RouteNoChute}
		s.piece.RouteReason = RouteNoChute
		s.routed = true
		p.aim(now, s, settings)
		return
	}

	ctx := context.Background()
	var res RouteResult
	var err error
	if s.piece.Status == PieceIdentified {
		piece := profile.Piece{Part: s.piece.PartID, Color: s.piece.ColorID, PartCategory: s.piece.PartCategory, ColorName: s.piece.ColorName}
		res, err = p.deps.Sorting.Route(ctx, piece, s.piece.PieceMm, true)
	} else {
		res, err = p.deps.Sorting.RouteCategory(ctx, profile.Misc, s.piece.PieceMm, true)
	}
	if err != nil {
		slog.Warn("routing a piece failed; it goes to the bucket", "piece", s.piece.ID, "error", err)
		res = RouteResult{Category: profile.Misc, CategoryName: "Misc", Choice: BinChoice{Reason: RouteMisc}}
	}
	s.choice = res.Choice
	s.piece.Category, s.piece.CategoryName = res.Category, res.CategoryName
	if s.piece.CategoryName == "" {
		s.piece.CategoryName = "Misc"
	}
	s.piece.RouteReason = s.choice.Reason
	if s.choice.Reason == RouteNoBin {
		s.piece.AddProblem(ProblemNoBin, "Every bin was taken, so it went to the bucket.", now)
	}
	if s.choice.Bin != nil {
		s.piece.BinLayer, s.piece.BinNumber = s.choice.Bin.Layer, s.choice.Bin.Bin
		// With the chute left out the bin is only where it would have gone; it still
		// counts, so bin layouts, full bins and plans can be tried without the chute.
		s.piece.VirtualBin = leftOut
	}
	s.routed = true
	p.aim(now, s, settings)
}

// aim sets the doors and turns the chute for a routed piece.
func (p *Platter) aim(now time.Time, s *platterSlot, settings MachineSettings) {
	if settings.Distributor.LeftOut {
		p.doorsSettled, p.aimedFor, p.aimDone = now, s.piece.ID, nil
		return
	}
	layer := -1
	if s.choice.Bin != nil {
		layer = s.choice.Bin.Layer - 1
	}
	if err := p.deps.HW.SelectLayer(layer); err != nil {
		slog.Warn("setting the layer servos failed", "error", err)
	}
	p.doorsSettled = now.Add(time.Duration(settings.Distributor.DoorSettleMs) * time.Millisecond)
	p.aimedFor = s.piece.ID
	p.aimDone = nil
	if s.choice.Bin == nil {
		return // the bucket: every layer closed, wherever the chute points
	}
	done, err := p.deps.HW.AimChute(s.choice.Bin.Angle)
	if err != nil {
		slog.Warn("aiming the chute failed; the piece goes to the bucket", "piece", s.piece.ID, "error", err)
		_ = p.deps.HW.SelectLayer(-1)
		s.choice = BinChoice{Reason: RouteMisc}
		s.piece.BinLayer, s.piece.BinNumber = 0, 0
		return
	}
	p.aimDone = done
}

// distributionReady reports whether the chute and doors are set for the piece that
// falls next.
func (p *Platter) distributionReady(now time.Time, s *platterSlot) bool {
	if s == nil {
		return true
	}
	if !s.routed || p.aimedFor != s.piece.ID || now.Before(p.doorsSettled) {
		return false
	}
	if p.aimDone != nil {
		select {
		case <-p.aimDone:
			p.aimDone = nil
		default:
			return false
		}
	}
	return true
}

// maybeIndex turns one sector when the drop sector is done with and the piece about
// to fall is ready.
func (p *Platter) maybeIndex(now time.Time, settings MachineSettings) {
	aboard := false
	for _, s := range p.slots {
		if s != nil {
			aboard = true
		}
	}
	if !aboard {
		return
	}
	drop := p.slots[0]
	exit := p.slots[p.exitIndex(settings)-1]
	// A finished piece at the exit drops as soon as it's routed and aimed, without
	// waiting out the idle time: the sooner it's off, the sooner C3 may feed again.
	exitReady := settings.Platter.ClearExitFirst && exit != nil && exit.routed
	if drop == nil {
		if !exitReady && now.Sub(p.freeSince) < time.Duration(settings.Platter.IdleIndexMs)*time.Millisecond {
			return
		}
	} else if drop.state < slotIdentifying && drop.piece.Status != PieceMultiDrop {
		return // still landing or being photographed
	}
	if (p.checkFall && now.Before(p.fallBy)) || !p.distributionReady(now, exit) {
		return // still watching the last piece fall off; or the chute isn't set
	}
	done, err := p.deps.HW.MoveChannel(ChannelC4, 360/float64(len(p.slots)), settings.IndexRpm)
	if err != nil {
		slog.Warn("platter index refused", "error", err)
		return
	}
	p.indexDone = done
	p.progress(now)
}

// finishIndex moves every slot on one sector; the piece reaching the exit has fallen.
func (p *Platter) finishIndex(now time.Time, ps PlatterSettings) {
	exit := min(max(1, ps.ExitSector), len(p.slots)-1)
	fallen := p.slots[exit-1]
	copy(p.slots[1:], p.slots[:len(p.slots)-1])
	p.slots[0] = nil
	p.slots[exit] = nil
	if fallen != nil {
		p.delivered++
		fallen.piece.Status = PieceDistributed
		fallen.piece.DistributedAt = &now
		p.deps.Records.SavePiece(fallen.piece)
		p.deps.Records.NoteCatalog(fallen.piece)
		if fallen.piece.BinLayer > 0 {
			p.deps.Sorting.NoteDelivered(BinKey{fallen.piece.BinLayer, fallen.piece.BinNumber})
		}
		p.recent = append([]Piece{fallen.piece.Clone()}, p.recent...)
		if len(p.recent) > recentPieces {
			p.recent = p.recent[:recentPieces]
		}
		slog.Info("piece distributed", "piece", fallen.piece.ID, "part", fallen.piece.PartID,
			"category", fallen.piece.Category, "layer", fallen.piece.BinLayer, "bin", fallen.piece.BinNumber)
	}
	p.settleUntil = now.Add(time.Duration(ps.SettleMs) * time.Millisecond)
	p.freeSince = p.settleUntil
	p.checkFall = fallen != nil
	p.fallBy = p.settleUntil.Add(time.Duration(ps.FallWaitMs) * time.Millisecond)
	p.progress(now)
}

// verifyFall looks at the first settled frame after a drop: a piece still in the exit
// didn't fall. It was counted as delivered already; the problem is noted and the
// machine shakes it off.
func (p *Platter) verifyFall(now time.Time, obs vision.Observation) {
	if !p.checkFall || !obs.Valid || obs.At.Before(p.settleUntil) {
		return
	}
	if obs.InExit && now.Before(p.fallBy) {
		return // still sliding off the edge: give it a moment before calling it stuck
	}
	p.checkFall = false
	if !obs.InExit || len(p.recent) == 0 {
		return
	}
	last := &p.recent[0]
	last.AddProblem(ProblemDidntFall, "It was still at the exit after the turn meant to drop it.", now)
	p.deps.Records.SavePiece(*last)
	if p.deps.OnDidntFall != nil {
		p.deps.OnDidntFall()
	}
}

// Sweep records every piece aboard as gone to the bucket, after the machine turned the
// channel round with every layer closed to clear it.
func (p *Platter) Sweep(now time.Time) int {
	n := 0
	for i, s := range p.slots {
		if s == nil {
			continue
		}
		s.piece.Status = PieceDistributed
		s.piece.DistributedAt = &now
		s.piece.BinLayer, s.piece.BinNumber, s.piece.RouteReason = 0, 0, RouteMisc
		s.piece.AddProblem(ProblemSweptOff, "The classification channel stopped making progress, so it was turned with every layer closed; the piece went to the bucket.", now)
		p.deps.Records.SavePiece(s.piece)
		p.deps.Records.NoteCatalog(s.piece)
		p.recent = append([]Piece{s.piece.Clone()}, p.recent...)
		p.slots[i] = nil
		p.delivered++
		n++
	}
	if len(p.recent) > recentPieces {
		p.recent = p.recent[:recentPieces]
	}
	p.aimDone, p.aimedFor, p.indexDone = nil, "", nil
	p.settleUntil, p.freeSince = now, now
	p.progress(now)
	return n
}

func (p *Platter) exitIndex(settings MachineSettings) int {
	return min(max(1, settings.Platter.ExitSector), len(p.slots)-1)
}

func (p *Platter) progress(now time.Time) {
	p.lastProgress = now
	p.stallRaised = false
}

// watchdog raises an incident when pieces are aboard and nothing has happened for
// too long.
func (p *Platter) watchdog(now time.Time, ps PlatterSettings) {
	if p.stallRaised || ps.StallMs <= 0 || p.deps.OnStall == nil {
		return
	}
	aboard := len(p.Aboard())
	if aboard == 0 || now.Sub(p.lastProgress) < time.Duration(ps.StallMs)*time.Millisecond {
		return
	}
	p.stallRaised = true
	p.deps.OnStall(aboard)
}
