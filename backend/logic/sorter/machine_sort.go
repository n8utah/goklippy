package sorter

import (
	"context"
	"fmt"
	"image"
	"log/slog"
	"slices"
	"time"

	"goklippy/backend/logic/vision"
)

// SortDeps connects the machine to the cameras, Brickognize, the profile and the
// records. Without them the machine still homes and aims, but sorting does nothing.
type SortDeps struct {
	Vision     VisionSource
	Identifier Identifier
	Sorting    *SortingService
	Records    *Records
	// Journeys, when set, learns which journey each landed piece is, to show the
	// piece's pictures from C2 and C3 with its record.
	Journeys *JourneyRecorder
}

type sortStations struct {
	deps    SortDeps
	feeder  *Feeder
	platter *Platter
	runID   int64
	spent   *timeSpent // this run's time per channel per state
}

// ChannelSummary is one camera channel's state for the dashboard.
type ChannelSummary struct {
	Valid     bool  `json:"valid"`
	AgeMs     int64 `json:"age_ms"`
	Pieces    int   `json:"pieces"`
	InDrop    bool  `json:"in_drop"`
	InPrecise bool  `json:"in_precise"`
	InExit    bool  `json:"in_exit"`
	Falls     int   `json:"falls,omitempty"` // pieces pushed off its exit this run
	// BackedUp: its leading piece is waiting at the exit, so the channel before it has
	// stopped feeding it (feeder back-pressure).
	BackedUp bool `json:"backed_up,omitempty"`
	// Ignored is how many stuck pieces its camera is leaving out (worked around).
	Ignored int `json:"ignored,omitempty"`
}

// AttachSorting wires in the sorting stations. Call it before Run.
func (m *Machine) AttachSorting(deps SortDeps) {
	st := &sortStations{deps: deps, feeder: NewFeeder()}
	st.platter = NewPlatter(PlatterDeps{
		HW: m.hw, Vision: deps.Vision, Identifier: deps.Identifier, Sorting: deps.Sorting, Records: deps.Records,
		Settings: m.settings,
		RunID:    func() int64 { return st.runID },
		// The platter steps on the loop, so these run on it too.
		OnStall:     m.onPlatterStall,
		OnDidntFall: m.onDidntFall,
	})
	if deps.Journeys != nil {
		st.platter.deps.OnLanded = func(pieceID string, at time.Time) {
			if id := deps.Journeys.NotePiece(pieceID, at); id != "" {
				slog.Info("piece linked to its journey", "piece", pieceID, "journey", id)
			}
		}
	}
	m.sort = st
}

// startRun opens a sort run, on Start.
func (m *Machine) startRun(now time.Time) error {
	if m.sort == nil {
		return nil
	}
	st := m.sort
	// With the chute left out pieces go nowhere in particular, so no profile is used
	// (or needed): the run has none.
	var profileID *int64
	name := ""
	if !m.settings().Distributor.LeftOut {
		active := st.deps.Sorting.Active()
		if active == nil {
			return ErrNoActiveProfile
		}
		id := active.ID
		profileID, name = &id, active.Name
	}
	runID, err := st.deps.Records.Store().StartRun(context.Background(), profileID, name, now)
	if err != nil {
		return fmt.Errorf("start a sort run: %w", err)
	}
	st.runID = runID
	st.feeder.Reset()
	st.platter.Reset(now)
	st.spent = newTimeSpent()
	st.spent.last = now
	slog.Info("sort run started", "run", runID, "profile", name)
	return nil
}

// endRun closes the run, on Stop or an error. Pieces still aboard are recorded lost.
func (m *Machine) endRun(now time.Time) {
	if m.sort == nil || m.sort.runID == 0 {
		return
	}
	st := m.sort
	st.platter.Reset(now)
	st.feeder.Reset()
	runID := st.runID
	st.runID = 0
	if err := st.deps.Records.Store().EndRun(context.Background(), runID, now); err != nil {
		slog.Warn("ending the sort run failed", "run", runID, "error", err)
	}
	slog.Info("sort run ended", "run", runID)
}

// stepSorting runs the stations for one tick: the platter, then the feeder feeding it.
func (m *Machine) stepSorting(now time.Time) {
	if m.sort == nil {
		return
	}
	st := m.sort
	var dt time.Duration
	if st.spent != nil {
		dt = st.spent.tick(now)
	}
	held := func() bool {
		if m.incident == nil && m.job == nil {
			return false
		}
		if st.spent != nil {
			st.spent.addAll(TimeStuck, dt) // a recovery, or stopped for an incident
		}
		return true
	}
	if held() {
		return
	}
	settings := m.settings()
	st.platter.Step(now)
	if held() {
		return // the platter just raised one or started a recovery
	}
	if stuck := st.feeder.Stuck(); len(stuck) > 0 {
		m.recoverLanding(stuck[0])
		held()
		return
	}
	moves := st.feeder.Step(FeederInput{
		Now:          now,
		C2:           st.deps.Vision.Observation(ChannelC2),
		C3:           st.deps.Vision.Observation(ChannelC3),
		PlatterReady: st.platter.Ready(now),
	}, settings)
	if st.feeder.TakeSeveralFell(ChannelC3) {
		st.platter.ExpectSeveral(now) // the next landing on C4 is a double
	}
	for _, mv := range moves {
		if _, err := m.hw.RampChannel(mv.Channel, mv.Degrees, mv.Rpm, mv.Accel); err != nil {
			slog.Warn("feeder move refused", "channel", mv.Channel, "error", err)
		}
	}
	if st.spent != nil {
		for _, ch := range []Channel{ChannelC1, ChannelC2, ChannelC3} {
			st.spent.add(ch, st.feeder.State(ch), dt)
		}
		st.spent.add(ChannelC4, st.platter.TimeState(now), dt)
	}
}

// raiseIncident stops the sort until the operator resolves it.
func (m *Machine) raiseIncident(inc Incident) {
	if m.incident != nil {
		return
	}
	if inc.OpenedAt.IsZero() {
		inc.OpenedAt = time.Now().UTC()
	}
	if m.sort != nil && m.sort.deps.Records != nil {
		id, err := m.sort.deps.Records.Store().OpenIncident(context.Background(), inc)
		if err != nil {
			slog.Warn("recording the incident failed", "kind", inc.Kind, "error", err)
		}
		inc.ID = id
	}
	_ = m.hw.StopAll()
	m.incident = &IncidentHold{Incident: inc, Actions: []string{ResolveDone}}
	m.changed = true
	slog.Warn("sorter incident", "kind", inc.Kind, "title", inc.Title, "message", inc.Message)
}

// ResolveIncident records how the operator dealt with the open incident, and lets
// the sort carry on.
func (m *Machine) ResolveIncident(ctx context.Context, action string) error {
	return m.send(ctx, request{do: func() error {
		if m.incident == nil {
			return ErrNoIncident
		}
		if !slices.Contains(m.incident.Actions, action) {
			return fmt.Errorf("%w: %q isn't a way to resolve this incident", ErrInvalidTransition, action)
		}
		now := time.Now().UTC()
		if m.incident.ID != 0 && m.sort != nil {
			if err := m.sort.deps.Records.Store().ResolveIncident(context.Background(), m.incident.ID, action, now); err != nil {
				slog.Warn("recording the incident's resolution failed", "error", err)
			}
		}
		m.incident = nil
		if m.sort != nil {
			m.sort.platter.progress(now)
			m.sort.feeder.Unstick(ChannelC2)
			m.sort.feeder.Unstick(ChannelC3)
		}
		m.changed = true
		return nil
	}})
}

func (m *Machine) recovery() recoveryRunner {
	return recoveryRunner{hw: m.hw, vision: m.sort.deps.Vision, settings: m.settings}
}

// workAroundStuck leaves a stuck piece out of the channel's camera, notes it, and lets
// the sort carry on (Feeder.WorkAroundStuck). It reports false when it can't: the
// setting is off, the vision can't ignore a spot, or the piece isn't in view.
func (m *Machine) workAroundStuck(ch Channel) bool {
	st := m.sort
	ign, ok := st.deps.Vision.(SpotIgnorer)
	if !ok || !m.settings().Feeder.WorkAroundStuck {
		return false
	}
	obs := st.deps.Vision.Observation(ch)
	lead, ok := obs.Leading()
	if !ok {
		return false
	}
	ign.IgnoreSpot(ch, lead.Box, pieceArea(obs, lead.Box))
	st.feeder.Unstick(ch)
	now := time.Now().UTC()
	inc := Incident{
		Kind:     IncidentLandingWorkedAround,
		Title:    fmt.Sprintf("Working around a piece stuck on %s", ch),
		Message:  "It doesn't move with the channel and shaking and pushing didn't free it, so its camera leaves it out and sorting carries on. Take it off when you're next at the machine; it's watched again once it's gone.",
		OpenedAt: now,
	}
	if st.deps.Records != nil {
		store := st.deps.Records.Store()
		if id, err := store.OpenIncident(context.Background(), inc); err != nil {
			slog.Warn("recording the worked-around piece failed", "error", err)
		} else if err := store.ResolveIncident(context.Background(), id, ResolveWorkedAround, now); err != nil {
			slog.Warn("recording the worked-around piece failed", "error", err)
		}
	}
	m.changed = true
	slog.Warn("working around a stuck piece", "channel", ch, "box", lead.Box)
	return true
}

// ForgetIgnored watches all of a channel again, after a stuck piece was taken off.
func (m *Machine) ForgetIgnored(ctx context.Context, ch Channel) error {
	return m.send(ctx, request{do: func() error {
		if m.sort == nil {
			return nil
		}
		if ign, ok := m.sort.deps.Vision.(SpotIgnorer); ok {
			ign.ForgetIgnored(ch)
		}
		m.changed = true
		return nil
	}})
}

// clearTurns is how far the platter turns, every layer closed, to clear a stall.
const clearTurns = 2

// onPlatterStall turns C4 round with every layer closed so what's aboard drops into
// the bucket, then asks the operator if the camera still sees a piece.
func (m *Machine) onPlatterStall(aboard int) {
	st := m.sort
	settings := m.settings()
	err := m.startJob("Recovery: clear the classification channel", func(ctx context.Context, report reportFunc) error {
		if !settings.Distributor.LeftOut {
			report(1, 2, "Closing every layer, so pieces drop into the bucket")
			if err := m.hw.SelectLayer(-1); err != nil {
				return err
			}
			if err := waitFor(ctx, time.Duration(settings.Distributor.DoorSettleMs)*time.Millisecond); err != nil {
				return err
			}
		}
		report(2, 2, fmt.Sprintf("Turning the channel %d times round", clearTurns))
		done, err := m.hw.MoveChannel(ChannelC4, 360*clearTurns, settings.IndexRpm)
		if err != nil {
			return err
		}
		if err := awaitDone(ctx, done); err != nil {
			return err
		}
		return waitFor(ctx, time.Duration(settings.Platter.SettleMs)*time.Millisecond+recoverySettle)
	}, func(err error) {
		now := time.Now().UTC()
		st.platter.Sweep(now)
		obs := st.deps.Vision.Observation(ChannelC4)
		if err == nil && obs.Valid && obs.Empty() {
			slog.Info("classification channel cleared", "pieces", aboard)
			return
		}
		m.raiseIncident(Incident{
			Kind:    IncidentPlatterStuck,
			Title:   "Stuck on the classification channel",
			Message: fmt.Sprintf("%d piece(s) made no progress, and turning the channel round didn't clear it. Clear the channel, then press Done.", aboard),
		})
	})
	if err != nil {
		slog.Warn("couldn't start clearing the classification channel", "error", err)
	}
}

// onDidntFall shakes a piece off C4's exit.
func (m *Machine) onDidntFall() {
	runner := m.recovery()
	err := m.startJob("Recovery: shake a piece off the classification channel", func(ctx context.Context, report reportFunc) error {
		report(1, 1, "Shake C4: Heavy stuck")
		return runner.shake(ctx, ChannelC4, "heavy")
	}, nil)
	if err != nil {
		slog.Warn("couldn't start shaking C4", "error", err)
	}
}

// ShakeChannel runs a shake by hand, as a job.
func (m *Machine) ShakeChannel(ctx context.Context, ch Channel, key string) error {
	p, err := shakeProfile(key)
	if err != nil {
		return err
	}
	return m.send(ctx, request{do: func() error {
		if err := m.handsOff(); err != nil {
			return err
		}
		sh := rotorShake(p, channelGear(m.settings(), ch))
		return m.startJob(fmt.Sprintf("Shake %s: %s", ch, p.Label), func(ctx context.Context, report reportFunc) error {
			done, err := m.hw.Shake(ch, sh)
			if err != nil {
				return err
			}
			return awaitDone(ctx, done)
		}, nil)
	}})
}

// RaiseIncident is for tests and the hardware layer to stop the sort.
func (m *Machine) RaiseIncident(ctx context.Context, inc Incident) error {
	return m.send(ctx, request{do: func() error {
		m.raiseIncident(inc)
		return nil
	}})
}

func (m *Machine) sortSnapshot(snap *Snapshot) {
	snap.Incident = m.incident
	snap.Aboard, snap.Recent = []Piece{}, []Piece{}
	if m.sort == nil {
		return
	}
	st := m.sort
	snap.RunID = st.runID
	if st.spent != nil {
		snap.TimeMs = st.spent.snapshot()
	}
	snap.Aboard = append(snap.Aboard, st.platter.Aboard()...)
	snap.Recent = append(snap.Recent, st.platter.Recent()...)
	now := time.Now()
	snap.Channels = map[Channel]ChannelSummary{}
	for _, ch := range []Channel{ChannelC2, ChannelC3, ChannelC4} {
		obs := st.deps.Vision.Observation(ch)
		sum := ChannelSummary{Valid: obs.Valid, Pieces: len(obs.Pieces), InDrop: obs.InDrop, InPrecise: obs.InPrecise, InExit: obs.InExit, Falls: st.feeder.Falls(ch), BackedUp: st.feeder.BackedUp(ch), Ignored: len(obs.Ignored)}
		if obs.Valid {
			sum.AgeMs = now.Sub(obs.At).Milliseconds()
		}
		snap.Channels[ch] = sum
	}
}

// NoVision is the vision source when no cameras are set up: every channel reports
// no frame, so the feeder and platter wait.
type NoVision struct{}

func (NoVision) Observation(Channel) vision.Observation { return vision.Observation{} }
func (NoVision) Frame(Channel) (image.Image, time.Time, bool) {
	return nil, time.Time{}, false
}
