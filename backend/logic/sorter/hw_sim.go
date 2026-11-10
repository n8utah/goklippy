package sorter

import (
	"context"
	"fmt"
	"sync"
)

// SimHardware stands in for the machine: moves take their real time (scaled by
// TimeScale) and positions are tracked, but nothing turns.
type SimHardware struct {
	settings func() MachineSettings
	// TimeScale multiplies every move's duration; 0 finishes moves at once (tests).
	TimeScale float64

	mu        sync.Mutex
	positions map[Channel]float64 // rotor degrees; the distributor's is the chute angle
	layer     int
	connected bool
	homed     bool
	moves     []SimMove
	onMove    func(ch Channel, from, to float64)
}

// OnMove calls fn after every move with the channel's rotor angle before and after
// (the chute's angle for the distributor), so a simulated world can move pieces.
func (h *SimHardware) OnMove(fn func(ch Channel, from, to float64)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onMove = fn
}

func (h *SimHardware) moved(ch Channel, from, to float64) {
	h.mu.Lock()
	fn := h.onMove
	h.mu.Unlock()
	if fn != nil {
		fn(ch, from, to)
	}
}

// SimMove records one MoveChannel call.
type SimMove struct {
	Channel Channel
	Degrees float64
	Rpm     float64
}

func NewSimHardware(settings func() MachineSettings) *SimHardware {
	return &SimHardware{
		settings:  settings,
		TimeScale: 1,
		positions: make(map[Channel]float64),
		layer:     -1,
		connected: true,
	}
}

// SetConnected simulates the board being plugged in or not.
func (h *SimHardware) SetConnected(connected bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.connected = connected
}

func (h *SimHardware) MoveChannel(ch Channel, deg, rpm float64) (<-chan error, error) {
	if _, err := ParseChannel(string(ch)); err != nil {
		return nil, err
	}
	if rpm <= 0 {
		return nil, fmt.Errorf("%s: speed must be positive", ch)
	}
	h.mu.Lock()
	if !h.connected {
		h.mu.Unlock()
		return nil, ErrNoBoard
	}
	from := h.positions[ch]
	h.positions[ch] += deg
	h.moves = append(h.moves, SimMove{Channel: ch, Degrees: deg, Rpm: rpm})
	d := moveDuration(deg, rpm, channelAccel(h.settings(), ch))
	h.mu.Unlock()
	h.moved(ch, from, from+deg)
	return doneAfter(seconds(d.Seconds() * h.TimeScale)), nil
}

func (h *SimHardware) SelectLayer(layer int) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.connected {
		return ErrNoBoard
	}
	if layer < -1 || layer >= len(h.settings().Distributor.Layers) {
		return fmt.Errorf("layer %d does not exist", layer+1)
	}
	h.layer = layer
	return nil
}

func (h *SimHardware) StopAll() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.connected {
		return ErrNoBoard
	}
	return nil
}

func (h *SimHardware) Health() HardwareHealth {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.connected {
		return HardwareHealth{Simulated: true, Message: ErrNoBoard.Error()}
	}
	return HardwareHealth{Connected: true, Simulated: true, Port: "simulated"}
}

func (h *SimHardware) HomeChute(ctx context.Context) error {
	h.mu.Lock()
	if !h.connected {
		h.mu.Unlock()
		return ErrNoBoard
	}
	d := moveDuration(h.positions[ChannelDistributor], h.settings().Distributor.HomeRpm, 0)
	h.mu.Unlock()
	if err := waitFor(ctx, seconds(d.Seconds()*h.TimeScale)); err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.positions[ChannelDistributor] = 0
	h.homed = true
	return nil
}

func (h *SimHardware) ZeroChute() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.connected {
		return ErrNoBoard
	}
	h.positions[ChannelDistributor] = 0
	h.homed = true
	return nil
}

func (h *SimHardware) AimChute(angle float64) (<-chan error, error) {
	d := h.settings().Distributor
	h.mu.Lock()
	if !h.connected {
		h.mu.Unlock()
		return nil, ErrNoBoard
	}
	if !h.homed {
		h.mu.Unlock()
		return nil, ErrChuteNotHomed
	}
	if angle < 0 || angle > d.MaxAngle {
		h.mu.Unlock()
		return nil, fmt.Errorf("%.1f° is outside the chute's travel (0–%.0f°)", angle, d.MaxAngle)
	}
	from := h.positions[ChannelDistributor]
	delta := angle - from
	h.positions[ChannelDistributor] = angle
	h.moves = append(h.moves, SimMove{Channel: ChannelDistributor, Degrees: delta, Rpm: d.Rpm})
	h.mu.Unlock()
	h.moved(ChannelDistributor, from, angle)
	return doneAfter(seconds(moveDuration(delta, d.Rpm, d.Accel).Seconds() * h.TimeScale)), nil
}

func (h *SimHardware) ChuteStatus() ChuteStatus {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.connected {
		return ChuteStatus{Message: ErrNoBoard.Error()}
	}
	status := ChuteStatus{Homed: h.homed, Endstop: "open"}
	if h.homed {
		angle := h.positions[ChannelDistributor]
		status.Angle = &angle
		if angle == 0 {
			status.Endstop = "triggered"
		}
	}
	return status
}

// Position is a channel's rotor angle in degrees since start.
func (h *SimHardware) Position(ch Channel) float64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.positions[ch]
}

// Layer is the selected layer, -1 for none.
func (h *SimHardware) Layer() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.layer
}

// Moves lists every move so far.
func (h *SimHardware) Moves() []SimMove {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]SimMove(nil), h.moves...)
}
