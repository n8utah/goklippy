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
	frames    [][]byte
	lastFrame time.Time
	sentAt    time.Time
	// The bin it was routed to, once it is next to fall.
	routed bool
	choice BinChoice
}

type identResult struct {
	pieceID string
	result  brickognize.Result
	err     error
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
	// checkFall: after a drop, the next settled frame must show the exit clear.
	checkFall bool
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

// Ready is the gate C3 reads: the drop sector is free and still.
func (p *Platter) Ready(now time.Time) bool {
	return p.indexDone == nil && p.slots[0] == nil && !now.Before(p.settleUntil)
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
	if len(inDrop) > 1 || touchingPieces(obs, inDrop[0].Box) {
		s.piece.Status = PieceMultiDrop
		s.piece.AddProblem(ProblemDoubleFeed, "It landed with another piece, so it couldn't be photographed alone.", now)
		s.state = slotResolved
	}
	p.slots[0] = s
	p.progress(now)
	p.deps.Records.SavePiece(s.piece)
	slog.Info("piece landed on the platter", "piece", s.piece.ID, "multi_drop", s.piece.Status == PieceMultiDrop)
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

// linedUp: one box's span across x or y lies mostly inside the other's, as the parts
// of one object stack (a minifigure's head over its torso).
func linedUp(a, b vision.Box) bool {
	overlap := func(a1, a2, b1, b2 int) float64 {
		span := min(a2-a1, b2-b1)
		shared := min(a2, b2) - max(a1, b1)
		if span <= 0 {
			return 0
		}
		return float64(shared) / float64(span)
	}
	return overlap(a.X1, a.X2, b.X1, b.X2) >= 0.7 || overlap(a.Y1, a.Y2, b.Y1, b.Y2) >= 0.7
}

// capture photographs the piece in the drop sector, then sends the burst off.
func (p *Platter) capture(now time.Time, obs vision.Observation, ps PlatterSettings) {
	s := p.slots[0]
	if s == nil {
		return
	}
	if s.state == slotSettling {
		if now.Sub(s.landedAt) < time.Duration(ps.SettleMs)*time.Millisecond {
			return
		}
		s.state = slotCapturing
	}
	if s.state != slotCapturing {
		return
	}
	if lead, ok := dropPiece(obs); ok {
		s.box = lead.Box
	}
	frame, at, ok := p.deps.Vision.Frame(ChannelC4)
	if !ok || !at.After(s.lastFrame) || at.Before(s.landedAt) {
		return
	}
	s.lastFrame = at
	if jpg, err := cropJPEG(frame, s.box); err == nil {
		p.deps.Records.SaveImage(s.piece.ID, len(s.frames), jpg, true, at)
		s.frames = append(s.frames, jpg)
	}
	want := min(max(1, ps.BurstFrames), brickognize.MaxImages)
	if len(s.frames) < want {
		return
	}
	s.state = slotIdentifying
	s.sentAt = now
	s.piece.Status = PieceIdentifying
	p.deps.Records.SavePiece(s.piece)
	p.progress(now)
	frames, id := s.frames, s.piece.ID
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(ps.IdentifyTimeoutMs)*time.Millisecond)
		defer cancel()
		res, err := p.deps.Identifier.Identify(ctx, frames)
		p.idents <- identResult{pieceID: id, result: res, err: err}
	}()
}

func dropPiece(obs vision.Observation) (vision.PieceObs, bool) {
	for _, o := range obs.Pieces {
		if o.Zone == vision.ZoneDrop {
			return o, true
		}
	}
	return vision.PieceObs{}, false
}

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

	ctx := context.Background()
	var res RouteResult
	var err error
	if s.piece.Status == PieceIdentified {
		res, err = p.deps.Sorting.Route(ctx, s.piece.PartID, s.piece.ColorID, s.piece.PieceMm, true)
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
	}
	s.routed = true
	p.aim(now, s, settings)
}

// aim sets the doors and turns the chute for a routed piece.
func (p *Platter) aim(now time.Time, s *platterSlot, settings MachineSettings) {
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
	if drop == nil {
		if now.Sub(p.freeSince) < time.Duration(settings.Platter.IdleIndexMs)*time.Millisecond {
			return
		}
	} else if drop.state < slotIdentifying && drop.piece.Status != PieceMultiDrop {
		return // still landing or being photographed
	}
	if !p.distributionReady(now, p.slots[p.exitIndex(settings)-1]) {
		return
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
		fallen.piece.Status = PieceDistributed
		fallen.piece.DistributedAt = &now
		p.deps.Records.SavePiece(fallen.piece)
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
	p.progress(now)
}

// verifyFall looks at the first settled frame after a drop: a piece still in the exit
// didn't fall. It was counted as delivered already; the problem is noted and the
// machine shakes it off.
func (p *Platter) verifyFall(now time.Time, obs vision.Observation) {
	if !p.checkFall || !obs.Valid || obs.At.Before(p.settleUntil) {
		return
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
		p.recent = append([]Piece{s.piece.Clone()}, p.recent...)
		p.slots[i] = nil
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
