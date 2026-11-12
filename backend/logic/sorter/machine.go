package sorter

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// State is where the sorter is in its lifecycle.
type State string

const (
	StateStandby State = "standby" // hardware not started
	StateHoming  State = "homing"
	StateReady   State = "ready" // homed, not sorting
	StateRunning State = "running"
	StatePaused  State = "paused"
	StateError   State = "error"
)

// Command is an operator action on the whole machine.
type Command string

const (
	CommandHome   Command = "home"
	CommandStart  Command = "start"
	CommandPause  Command = "pause"
	CommandResume Command = "resume"
	CommandStop   Command = "stop"
)

var ErrBusy = errors.New("the sorter is busy with another task; wait for it or cancel it")

// Snapshot is the sorter's state as the UI sees it.
type Snapshot struct {
	State    State          `json:"state"`
	Error    string         `json:"error,omitempty"`
	Since    time.Time      `json:"since"`
	Hardware HardwareHealth `json:"hardware"`
	Chute    ChuteStatus    `json:"chute"`
	Lights   LightsStatus   `json:"lights"`
	Supply   SupplyStatus   `json:"supply"`
	Job      *JobStatus     `json:"job,omitempty"`      // the task running now
	LastJob  *JobStatus     `json:"last_job,omitempty"` // the last one to finish
	Ticks    uint64         `json:"ticks"`              // control-loop ticks spent running

	// Sorting, once AttachSorting is called.
	Incident *IncidentHold              `json:"incident,omitempty"`
	RunID    int64                      `json:"run_id,omitempty"`
	Aboard   []Piece                    `json:"aboard"`
	Recent   []Piece                    `json:"recent"`
	Channels map[Channel]ChannelSummary `json:"channels,omitempty"`
	// TimeMs: the current (or last) run's time per channel per state, in ms
	// (time_spent.go): where the time goes.
	TimeMs map[Channel]map[string]int64 `json:"time_ms,omitempty"`
}

// JobStatus is a task the machine runs off the loop: homing, aiming, sending to a bin.
type JobStatus struct {
	Name   string    `json:"name"`
	Step   int       `json:"step"`
	Total  int       `json:"total"`
	Detail string    `json:"detail,omitempty"`
	Error  string    `json:"error,omitempty"` // only on LastJob
	At     time.Time `json:"at"`
}

const (
	defaultTick        = 50 * time.Millisecond
	healthCheckEvery   = time.Second
	publishTicksEvery  = time.Second
	subscriberCapacity = 1
	defaultTestDwell   = time.Second
)

// request is something handed to the loop, with its reply.
type request struct {
	command Command
	do      func() error // runs on the loop
	reply   chan error
}

type jobProgress struct {
	job         *job
	step, total int
	detail      string
}

type jobResult struct {
	job *job
	err error
}

type job struct {
	status JobStatus
	cancel context.CancelFunc
	// onDone runs on the loop when the job ends.
	onDone func(err error)
}

// Machine runs the sorter. One goroutine (Run) owns all state; everything else talks
// to it through requests, so nothing about the machine changes from two places.
// Slow hardware sequences run as jobs, one at a time, in their own goroutines.
type Machine struct {
	hw       Hardware
	settings func() MachineSettings
	tick     time.Duration

	requests chan request
	progress chan jobProgress
	results  chan jobResult
	done     chan struct{} // closed when Run returns

	snapMu sync.RWMutex
	snap   Snapshot

	subsMu sync.Mutex
	subs   map[chan Snapshot]struct{}

	// Owned by the Run goroutine.
	state    State
	since    time.Time
	err      string
	ticks    uint64
	health   HardwareHealth
	chute    ChuteStatus
	lights   LightsStatus
	supply   SupplyStatus
	job      *job
	lastJob  *JobStatus
	sort     *sortStations
	incident *IncidentHold
	// freed: where each channel's last stuck piece was shaken or pushed free, and when,
	// so one that sticks again there skips straight past shaking (machine_stuck.go).
	freed    map[Channel]freedSpot
	changed  bool
	lastSent time.Time
}

// NewMachine creates a machine in standby. tick 0 uses the default 50 ms.
func NewMachine(hw Hardware, settings func() MachineSettings, tick time.Duration) *Machine {
	if tick <= 0 {
		tick = defaultTick
	}
	m := &Machine{
		hw:       hw,
		settings: settings,
		tick:     tick,
		requests: make(chan request),
		progress: make(chan jobProgress, 16),
		results:  make(chan jobResult, 1),
		done:     make(chan struct{}),
		subs:     make(map[chan Snapshot]struct{}),
		state:    StateStandby,
		since:    time.Now().UTC(),
		health:   hw.Health(),
		chute:    hw.ChuteStatus(),
		lights:   hw.LightsStatus(),
		supply:   hw.SupplyVoltage(),
	}
	m.snap = m.snapshot()
	return m
}

// Done is closed once Run has returned, after its stop-time cleanup.
func (m *Machine) Done() <-chan struct{} { return m.done }

// Run is the control loop. It returns when ctx ends, stopping the motors if busy and
// turning the light off. Call it once.
func (m *Machine) Run(ctx context.Context) {
	defer close(m.done)
	ticker := time.NewTicker(m.tick)
	defer ticker.Stop()
	health := time.NewTicker(healthCheckEvery)
	defer health.Stop()

	for {
		select {
		case <-ctx.Done():
			if m.job != nil {
				m.job.cancel()
			}
			if m.job != nil || m.state == StateRunning || m.state == StatePaused {
				_ = m.hw.StopAll()
			}
			m.lightsOff()
			return
		case req := <-m.requests:
			// Publish before replying, so a caller sees what its request changed.
			err := m.handle(req)
			m.publish()
			req.reply <- err
			continue
		case p := <-m.progress:
			if p.job == m.job {
				m.job.status.Step, m.job.status.Total, m.job.status.Detail = p.step, p.total, p.detail
				m.changed = true
			}
		case r := <-m.results:
			m.finishJob(r)
		case <-health.C:
			m.checkHealth()
		case <-ticker.C:
			m.step()
		}
		m.publish()
	}
}

// Do runs a command on the loop and returns the resulting snapshot.
func (m *Machine) Do(ctx context.Context, cmd Command) (Snapshot, error) {
	err := m.send(ctx, request{command: cmd})
	return m.Snapshot(), err
}

// MoveChannel turns one channel by hand. It is refused while the machine is sorting,
// homing, or running another task, since the motors are spoken for then.
func (m *Machine) MoveChannel(ctx context.Context, ch Channel, degrees, rpm float64) error {
	return m.send(ctx, request{do: func() error {
		if err := m.handsOff(); err != nil {
			return err
		}
		_, err := m.hw.MoveChannel(ch, degrees, rpm)
		return err
	}})
}

// SetLights turns the machine's light on or off. It's allowed at any time, sorting
// included: it moves nothing.
func (m *Machine) SetLights(ctx context.Context, on bool) error {
	return m.send(ctx, request{do: func() error {
		err := m.hw.SetLights(on)
		m.refreshLights()
		return err
	}})
}

// BlinkLights switches the light on and off every period for a while, then leaves it
// off: one person can check the wiring at the machine with a meter or by eye. It's a
// job, so Cancel stops it; the light is left off either way.
func (m *Machine) BlinkLights(ctx context.Context, d, period time.Duration) error {
	if d <= 0 || d > 5*time.Minute || period < 200*time.Millisecond {
		return fmt.Errorf("%w: blink for up to 5 minutes, at most every 200 ms", ErrInvalidSettings)
	}
	return m.send(ctx, request{do: func() error {
		if err := m.handsOff(); err != nil {
			return err
		}
		if err := m.hw.SetLights(true); err != nil {
			return err // nothing to blink: say why now
		}
		n := int(d / period)
		return m.startJob("Blink the light", func(ctx context.Context, report reportFunc) error {
			defer func() { _ = m.hw.SetLights(false) }()
			on := true
			for i := 1; i <= n; i++ {
				state := "on"
				if !on {
					state = "off"
				}
				report(i, n, "Light "+state)
				if err := waitFor(ctx, period); err != nil {
					return err
				}
				on = !on
				if err := m.hw.SetLights(on); err != nil {
					return err
				}
			}
			return nil
		}, func(error) { m.refreshLights() })
	}})
}

// lightsForSorting turns the light on as sorting starts, if set to. A light that
// won't switch doesn't stop the run; it's logged and shown on the page.
func (m *Machine) lightsForSorting() {
	if !m.settings().Lights.OnWhenSorting || m.lights.On {
		return
	}
	if err := m.hw.SetLights(true); err != nil {
		slog.Warn("sorter: light not turned on for sorting", "error", err)
	}
	m.refreshLights()
}

// lightsOff is part of every stop, for safety: the light is never left on by a
// machine that isn't sorting. A failure is logged; it doesn't stop the stop.
func (m *Machine) lightsOff() {
	if err := m.hw.SetLights(false); err != nil {
		slog.Warn("sorter: light not turned off", "error", err)
	}
	m.refreshLights()
}

func (m *Machine) refreshLights() {
	if lights := m.hw.LightsStatus(); lights != m.lights {
		m.lights = lights
		m.changed = true
	}
}

// ZeroChute makes the chute's current position 0° without its switch.
func (m *Machine) ZeroChute(ctx context.Context) error {
	return m.send(ctx, request{do: func() error {
		if err := m.handsOff(); err != nil {
			return err
		}
		if err := m.hw.ZeroChute(); err != nil {
			return err
		}
		m.refreshChute()
		return nil
	}})
}

// AimChute turns the chute to an angle as a job.
func (m *Machine) AimChute(ctx context.Context, angle float64) error {
	return m.send(ctx, request{do: func() error {
		if err := m.needsHomedChute(); err != nil {
			return err
		}
		return m.startJob(fmt.Sprintf("Aim the chute at %.1f°", angle), func(ctx context.Context, report reportFunc) error {
			return m.aim(ctx, angle)
		}, nil)
	}})
}

// SendToBin selects a bin's layer and aims the chute at it, as a job.
func (m *Machine) SendToBin(ctx context.Context, addr BinAddress) error {
	return m.send(ctx, request{do: func() error {
		if err := m.needsHomedChute(); err != nil {
			return err
		}
		pos, err := m.settings().Distributor.BinPosition(addr)
		if err != nil {
			return err
		}
		if !pos.Reachable {
			return fmt.Errorf("%w: layer %d bin %d is at %.1f°, in the wedge by home", ErrBinUnreachable, addr.Layer, addr.Bin, pos.Angle)
		}
		name := fmt.Sprintf("Send to layer %d, bin %d", addr.Layer, addr.Bin)
		return m.startJob(name, func(ctx context.Context, report reportFunc) error {
			if err := m.selectLayer(ctx, addr.Layer-1); err != nil {
				return err
			}
			return m.aim(ctx, pos.Angle)
		}, nil)
	}})
}

// TestLayer selects a layer and points the chute at each of its reachable bins in
// turn, pausing dwell at each, then closes every layer. dwell 0 uses one second.
func (m *Machine) TestLayer(ctx context.Context, layer int, dwell time.Duration) error {
	if dwell <= 0 {
		dwell = defaultTestDwell
	}
	return m.send(ctx, request{do: func() error {
		if err := m.needsHomedChute(); err != nil {
			return err
		}
		d := m.settings().Distributor
		if layer < 1 || layer > len(d.Layers) {
			return fmt.Errorf("%w: layer %d does not exist", ErrNoSuchBin, layer)
		}
		var bins []BinPosition
		for _, pos := range d.LayerBins()[layer-1] {
			if pos.Reachable {
				bins = append(bins, pos)
			}
		}
		if len(bins) == 0 {
			return fmt.Errorf("layer %d has no bins the chute can reach", layer)
		}
		return m.startJob(fmt.Sprintf("Test every bin on layer %d", layer), func(ctx context.Context, report reportFunc) error {
			if err := m.selectLayer(ctx, layer-1); err != nil {
				return err
			}
			for i, pos := range bins {
				report(i+1, len(bins), fmt.Sprintf("Bin %d (%.1f°)", pos.Bin, pos.Angle))
				if err := m.aim(ctx, pos.Angle); err != nil {
					return err
				}
				if err := waitFor(ctx, dwell); err != nil {
					return err
				}
			}
			return m.hw.SelectLayer(-1)
		}, nil)
	}})
}

// CancelJob stops the running job and the motors.
func (m *Machine) CancelJob(ctx context.Context) error {
	return m.send(ctx, request{do: func() error {
		if m.job == nil {
			return nil
		}
		m.job.cancel()
		return m.hw.StopAll()
	}})
}

func (m *Machine) send(ctx context.Context, req request) error {
	req.reply = make(chan error, 1)
	select {
	case m.requests <- req:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-req.reply:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ChuteNow reads the chute from the hardware rather than the last snapshot, for when
// the angle must be current (calibration captures). It only reads.
func (m *Machine) ChuteNow() ChuteStatus {
	return m.hw.ChuteStatus()
}

// Snapshot is the latest published state.
func (m *Machine) Snapshot() Snapshot {
	m.snapMu.RLock()
	defer m.snapMu.RUnlock()
	return m.snap
}

// Subscribe returns a channel of snapshots, sent on every change and once a second
// while running. Call the returned func to stop.
func (m *Machine) Subscribe() (<-chan Snapshot, func()) {
	ch := make(chan Snapshot, subscriberCapacity)
	m.subsMu.Lock()
	m.subs[ch] = struct{}{}
	m.subsMu.Unlock()
	return ch, func() {
		m.subsMu.Lock()
		delete(m.subs, ch)
		m.subsMu.Unlock()
	}
}

func (m *Machine) handle(req request) error {
	if req.do != nil {
		return req.do()
	}
	switch req.command {
	case CommandHome:
		return m.startHoming()
	case CommandStart:
		if m.job != nil {
			return ErrBusy
		}
		if m.state != StateReady {
			return fmt.Errorf("%w: can't start while %s", ErrInvalidTransition, m.state)
		}
		if err := m.needsSupply(); err != nil {
			return err
		}
		if err := m.startRun(time.Now().UTC()); err != nil {
			return err
		}
		m.lightsForSorting()
		return m.transition(req.command, StateRunning, StateReady)
	case CommandPause:
		return m.transition(req.command, StatePaused, StateRunning)
	case CommandResume:
		if m.job != nil {
			return ErrBusy
		}
		if err := m.needsSupply(); err != nil {
			return err
		}
		if err := m.transition(req.command, StateRunning, StatePaused); err != nil {
			return err
		}
		m.lightsForSorting()
		return nil
	case CommandStop:
		if err := m.transition(req.command, StateReady, StateRunning, StatePaused); err != nil {
			return err
		}
		if m.job != nil {
			m.job.cancel() // a recovery mid-way
		}
		m.endRun(time.Now().UTC())
		m.lightsOff()
		if err := m.hw.StopAll(); err != nil {
			slog.Warn("sorter stop: motors did not all stop", "error", err)
			return fmt.Errorf("stopped, but the motors did not all stop: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unknown command %q", req.command)
	}
}

// handsOff refuses manual hardware use while the machine is sorting or busy.
func (m *Machine) handsOff() error {
	switch {
	case m.job != nil:
		return ErrBusy
	case m.state == StateRunning || m.state == StateHoming:
		return fmt.Errorf("%w: pause or stop the sorter first", ErrInvalidTransition)
	}
	return nil
}

// needsHomedChute is handsOff plus a homed chute, for anything that aims it.
func (m *Machine) needsHomedChute() error {
	if err := m.handsOff(); err != nil {
		return err
	}
	if !m.chute.Homed {
		return ErrChuteNotHomed
	}
	return nil
}

// transition moves to next if the machine is in one of from.
func (m *Machine) transition(cmd Command, next State, from ...State) error {
	for _, s := range from {
		if m.state == s {
			m.setState(next, "")
			return nil
		}
	}
	return fmt.Errorf("%w: can't %s while %s", ErrInvalidTransition, cmd, m.state)
}

func (m *Machine) startHoming() error {
	if m.job != nil {
		return ErrBusy
	}
	if m.state == StateRunning {
		return fmt.Errorf("%w: can't home while running", ErrInvalidTransition)
	}
	if err := m.needsSupply(); err != nil {
		return err
	}
	err := m.startJob("Home", func(ctx context.Context, report reportFunc) error {
		return m.home(ctx, report)
	}, func(err error) {
		if err != nil {
			slog.Warn("sorter homing failed", "error", err)
			m.setState(StateError, err.Error())
			return
		}
		slog.Info("sorter homed")
		m.setState(StateReady, "")
	})
	if err != nil {
		return err
	}
	m.setState(StateHoming, "")
	return nil
}

// home brings the hardware to a known state: the chute homed on its switch, every
// layer closed, and the chute parked at bin 1.
func (m *Machine) home(ctx context.Context, report reportFunc) error {
	if health := m.hw.Health(); !health.Connected {
		return errors.New(health.Message)
	}
	if m.settings().Distributor.LeftOut {
		report(1, 1, "The chute is left out, so there's nothing to home")
		return nil
	}
	report(1, 3, "Homing the chute")
	if err := m.hw.HomeChute(ctx); err != nil {
		return err
	}
	report(2, 3, "Closing every layer")
	if err := m.selectLayer(ctx, -1); err != nil {
		return fmt.Errorf("closing the layer servos: %w", err)
	}
	report(3, 3, "Parking at bin 1")
	return m.aim(ctx, m.settings().Distributor.FirstBinAngle())
}

// selectLayer sets the layer servos and waits for them to get there.
func (m *Machine) selectLayer(ctx context.Context, layer int) error {
	if err := m.hw.SelectLayer(layer); err != nil {
		return err
	}
	return waitFor(ctx, time.Duration(m.settings().Distributor.DoorSettleMs)*time.Millisecond)
}

// aim turns the chute and waits for it to arrive.
func (m *Machine) aim(ctx context.Context, angle float64) error {
	done, err := m.hw.AimChute(angle)
	if err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}

type reportFunc func(step, total int, detail string)

// startJob runs fn in its own goroutine. onDone, if given, runs on the loop when it
// ends. Only one job runs at a time.
func (m *Machine) startJob(name string, fn func(ctx context.Context, report reportFunc) error, onDone func(err error)) error {
	if m.job != nil {
		return ErrBusy
	}
	ctx, cancel := context.WithCancel(context.Background())
	j := &job{status: JobStatus{Name: name, At: time.Now().UTC()}, cancel: cancel, onDone: onDone}
	m.job = j
	m.changed = true
	report := func(step, total int, detail string) {
		select {
		case m.progress <- jobProgress{job: j, step: step, total: total, detail: detail}:
		default: // progress is advisory; never block the job on it
		}
	}
	go func() {
		err := fn(ctx, report)
		cancel()
		m.results <- jobResult{job: j, err: err}
	}()
	slog.Info("sorter job started", "job", name)
	return nil
}

func (m *Machine) finishJob(r jobResult) {
	if r.job != m.job {
		return
	}
	m.job = nil
	status := r.job.status
	status.At = time.Now().UTC()
	if r.err != nil {
		status.Error = r.err.Error()
		if errors.Is(r.err, context.Canceled) {
			status.Error = "Cancelled"
		}
		slog.Warn("sorter job failed", "job", status.Name, "error", r.err)
	} else {
		slog.Info("sorter job done", "job", status.Name)
	}
	m.lastJob = &status
	if r.job.onDone != nil {
		r.job.onDone(r.err)
	}
	m.refreshChute()
	m.changed = true
}

// checkHealth refreshes the hardware and chute state. Losing the board or the chute's
// home while homed is an error.
func (m *Machine) checkHealth() {
	health := m.hw.Health()
	if health != m.health {
		m.health = health
		m.changed = true
	}
	m.refreshChute()
	m.refreshLights()
	m.refreshSupply()
	switch m.state {
	case StateReady, StateRunning, StatePaused:
	default:
		return
	}
	switch {
	case !health.Connected:
		m.stopForError("Board disconnected: " + health.Message)
	case !m.chute.Homed && !m.chute.LeftOut:
		m.stopForError("The chute lost its home position (was the board reset?). Home again.")
	case m.supply.Known && m.supply.Low && (m.state == StateRunning || m.state == StatePaused):
		m.stopForError(m.supply.lowError().Error())
	}
}

func (m *Machine) refreshSupply() {
	if s := m.hw.SupplyVoltage(); !s.same(m.supply) {
		m.supply = s
		m.changed = true
	}
}

// needsSupply refuses to move the machine on purpose with its motors unpowered: they
// wouldn't move, and nothing would say why.
func (m *Machine) needsSupply() error {
	m.refreshSupply()
	if m.supply.Known && m.supply.Low {
		return m.supply.lowError()
	}
	return nil
}

func (m *Machine) stopForError(msg string) {
	if m.state == StateRunning || m.state == StatePaused {
		_ = m.hw.StopAll()
		m.endRun(time.Now().UTC())
	}
	m.lightsOff()
	m.setState(StateError, msg)
}

func (m *Machine) refreshChute() {
	chute := m.hw.ChuteStatus()
	if m.settings().Distributor.LeftOut {
		chute.LeftOut = true
		chute.Message = "Left out for now (Machine › Chute): sorting runs without homing or aiming it."
	}
	if !chuteStatusEqual(chute, m.chute) {
		m.chute = chute
		m.changed = true
	}
}

func chuteStatusEqual(a, b ChuteStatus) bool {
	if a.Homed != b.Homed || a.LeftOut != b.LeftOut || a.Endstop != b.Endstop || a.Message != b.Message {
		return false
	}
	if (a.Angle == nil) != (b.Angle == nil) {
		return false
	}
	return a.Angle == nil || *a.Angle == *b.Angle
}

// step is one control-loop tick: while running, the sorting stations take a step.
func (m *Machine) step() {
	if m.state != StateRunning {
		return
	}
	m.ticks++
	m.stepSorting(time.Now())
}

func (m *Machine) setState(next State, errMsg string) {
	if m.state == next && m.err == errMsg {
		return
	}
	slog.Info("sorter state", "from", m.state, "to", next, "error", errMsg)
	m.state = next
	m.err = errMsg
	m.since = time.Now().UTC()
	m.changed = true
}

func (m *Machine) snapshot() Snapshot {
	snap := Snapshot{
		State: m.state, Error: m.err, Since: m.since, Hardware: m.health,
		Chute: m.chute, Lights: m.lights, Supply: m.supply, LastJob: m.lastJob, Ticks: m.ticks,
	}
	if m.job != nil {
		status := m.job.status
		snap.Job = &status
	}
	m.sortSnapshot(&snap)
	return snap
}

// publish stores the snapshot and sends it to subscribers when something changed, or
// once a second while running so tick counts stay live.
func (m *Machine) publish() {
	snap := m.snapshot()
	m.snapMu.Lock()
	m.snap = snap
	m.snapMu.Unlock()

	periodic := m.state == StateRunning && time.Since(m.lastSent) >= publishTicksEvery
	if !m.changed && !periodic {
		return
	}
	m.changed = false
	m.lastSent = time.Now()

	m.subsMu.Lock()
	defer m.subsMu.Unlock()
	for ch := range m.subs {
		// Keep only the newest snapshot for a slow subscriber.
		select {
		case <-ch:
		default:
		}
		select {
		case ch <- snap:
		default:
		}
	}
}
