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
}

type sortStations struct {
	deps    SortDeps
	feeder  *Feeder
	platter *Platter
	runID   int64
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
}

// AttachSorting wires in the sorting stations. Call it before Run.
func (m *Machine) AttachSorting(deps SortDeps) {
	st := &sortStations{deps: deps, feeder: NewFeeder()}
	st.platter = NewPlatter(PlatterDeps{
		HW: m.hw, Vision: deps.Vision, Identifier: deps.Identifier, Sorting: deps.Sorting, Records: deps.Records,
		Settings: m.settings,
		RunID:    func() int64 { return st.runID },
		// The platter steps on the loop, so it can raise straight onto it.
		RaiseIncident: m.raiseIncident,
	})
	m.sort = st
}

// startRun opens a sort run, on Start.
func (m *Machine) startRun(now time.Time) error {
	if m.sort == nil {
		return nil
	}
	st := m.sort
	active := st.deps.Sorting.Active()
	if active == nil {
		return ErrNoActiveProfile
	}
	id := active.ID
	profileID, name := &id, active.Name
	runID, err := st.deps.Records.Store().StartRun(context.Background(), profileID, name, now)
	if err != nil {
		return fmt.Errorf("start a sort run: %w", err)
	}
	st.runID = runID
	st.feeder.Reset()
	st.platter.Reset(now)
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
	if m.sort == nil || m.incident != nil {
		return
	}
	st := m.sort
	settings := m.settings()
	st.platter.Step(now)
	if m.incident != nil {
		return // the platter just raised one
	}
	moves := st.feeder.Step(FeederInput{
		Now:          now,
		C2:           st.deps.Vision.Observation(ChannelC2),
		C3:           st.deps.Vision.Observation(ChannelC3),
		PlatterReady: st.platter.Ready(now),
	}, settings)
	for _, mv := range moves {
		if _, err := m.hw.MoveChannel(mv.Channel, mv.Degrees, mv.Rpm); err != nil {
			slog.Warn("feeder move refused", "channel", mv.Channel, "error", err)
		}
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
		}
		m.changed = true
		return nil
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
	snap.Aboard = append(snap.Aboard, st.platter.Aboard()...)
	snap.Recent = append(snap.Recent, st.platter.Recent()...)
	now := time.Now()
	snap.Channels = map[Channel]ChannelSummary{}
	for _, ch := range []Channel{ChannelC2, ChannelC3, ChannelC4} {
		obs := st.deps.Vision.Observation(ch)
		sum := ChannelSummary{Valid: obs.Valid, Pieces: len(obs.Pieces), InDrop: obs.InDrop, InPrecise: obs.InPrecise, InExit: obs.InExit, Falls: st.feeder.Falls(ch)}
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
