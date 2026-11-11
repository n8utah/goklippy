package mcu

import (
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"goklippy/backend/mcu/cfgtranslate"
	"goklippy/backend/mcu/msgproto"
)

// Session holds a live MCU connection with configured objects and readings.
type Session struct {
	mu sync.RWMutex

	ID            string
	Port          string
	Baud          int
	Conn          *Conn
	Objects       []cfgtranslate.ObjectSpec
	Constants     map[string]interface{}
	ClockFreq     float64
	ClockBase     uint64
	ClockBaseAt   time.Time
	stopTrsyncOIDs map[int]int // stepper OID -> dedicated stop trsync OID
	InitCommands   []string
	ConfigLines    []string // finalized config, resent after a board reset
	recoverMu      sync.Mutex
	resetting      atomic.Bool

	motionMu   sync.Mutex
	motionSubs map[chan MotionSnapshot]struct{}
	motionStop chan struct{}
	motionDone chan struct{}

	telemMu       sync.Mutex
	telemSubs     map[chan map[string]Reading]struct{}
	telemRunning  bool

	jogMu      sync.Mutex
	jogKeepers map[int]*stepperJogKeeper

	stepperLocksGuard sync.Mutex
	stepperLocks      map[int]*sync.Mutex // per-stepper: reset_step_clock must not overlap on same oid
	routineKeepersMu  sync.Mutex
	routineKeepers    map[int]*stepperRoutineKeeper
	routineErrors     map[int]string // stepper OID -> why its routine stopped on its own
	routineSerialMu   sync.Mutex // one COM port: serialize routine MCU traffic across oids
	indexRemainders   map[int]float64 // stepper OID -> rounding carried between indexed moves

	pcaMu    sync.Mutex
	pcaReady map[int]bool // PCA9685 board oid -> registers set up

	readings       map[int]Reading
	stepperJogging map[int]bool
	outputLevels   map[int]bool      // digital out OID -> level last set (see session_output.go)
	outputSent     map[int]time.Time // digital out OID -> when its level was last sent
	outputStop     chan struct{}     // stops the output keeper; nil when not running
	err            string
}

// Reading is a cached value for one OID.
type Reading struct {
	Kind      cfgtranslate.ObjectKind `json:"kind"`
	Section   string                  `json:"section"`
	SlotID    string                  `json:"slot_id,omitempty"`
	Value     interface{}             `json:"value"`
	UpdatedAt time.Time               `json:"updated_at"`
	Unit      string                  `json:"unit,omitempty"`
}

// NewSession creates a session wrapper around an open connection.
func NewSession(id, port string, baud int, conn *Conn, objects []cfgtranslate.ObjectSpec) *Session {
	constants := conn.Parser().Get_constants()
	clockFreq := float64(16000000)
	if v, ok := constants["CLOCK_FREQ"]; ok {
		switch n := v.(type) {
		case int64:
			clockFreq = float64(n)
		case float64:
			clockFreq = n
		}
	}
	return &Session{
		ID:             id,
		Port:           port,
		Baud:           baud,
		Conn:           conn,
		Objects:        objects,
		Constants:      constants,
		ClockFreq:      clockFreq,
		readings:       make(map[int]Reading),
		stepperJogging: make(map[int]bool),
		stopTrsyncOIDs: make(map[int]int),
	}
}

// Close shuts down the serial connection.
func (s *Session) Close() error {
	s.StopTelemetry()
	s.stopOutputKeeper()
	s.stopMotion()
	s.stopAllJogKeepers()
	s.stopAllStepperRoutines()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Conn == nil {
		return nil
	}
	err := s.Conn.Close()
	s.Conn = nil
	return err
}

// SyncClock queries get_clock and records host/MCU clock baseline.
func (s *Session) SyncClock() error {
	params, err := s.Conn.QueryCommand("get_clock", "clock")
	if err != nil {
		return err
	}
	clock, ok := params["clock"].(int64)
	if !ok {
		return fmt.Errorf("get_clock: unexpected response")
	}
	s.mu.Lock()
	s.ClockBase = uint64(clock)
	s.ClockBaseAt = time.Now()
	s.mu.Unlock()
	return nil
}

// EstimatedClock returns the current estimated MCU clock tick.
func (s *Session) EstimatedClock() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.ClockBaseAt.IsZero() {
		return 0
	}
	elapsed := time.Since(s.ClockBaseAt)
	ticks := uint64(elapsed.Seconds() * s.ClockFreq)
	return s.ClockBase + ticks
}

// Poll drains inbound messages and returns current readings.
func (s *Session) Poll(drainFor time.Duration) map[string]Reading {
	s.Conn.DrainInbound(drainFor, s.handleMessage)

	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]Reading, len(s.readings))
	for oid, r := range s.readings {
		out[fmt.Sprintf("%d", oid)] = r
	}
	return out
}

func (s *Session) handleMessage(params map[string]interface{}) {
	if params == nil {
		return
	}
	name, _ := params["#name"].(string)
	oid64, hasOID := params["oid"].(int64)
	if !hasOID {
		return
	}
	oid := int(oid64)

	s.mu.Lock()
	defer s.mu.Unlock()

	spec := s.objectByOID(oid)
	now := time.Now()

	switch name {
	case "analog_in_state":
		val := extractAnalogValue(params)
		r := Reading{Kind: cfgtranslate.KindAnalogIn, Value: val, UpdatedAt: now, Unit: "adc"}
		if spec != nil {
			r.Section = spec.Section
			r.SlotID = spec.SlotID
			r.Kind = spec.Kind
		}
		s.readings[oid] = r
	case "endstop_state":
		pinVal, _ := params["pin_value"].(int64)
		r := Reading{Kind: cfgtranslate.KindEndstop, Value: pinVal != 0, UpdatedAt: now}
		if spec != nil {
			r.Section = spec.Section
			r.SlotID = spec.SlotID
		}
		s.readings[oid] = r
	case "stepper_position":
		pos, _ := params["pos"].(int64)
		r := Reading{Kind: cfgtranslate.KindStepper, Value: pos, UpdatedAt: now, Unit: "steps"}
		if spec != nil {
			r.Section = spec.Section
			r.SlotID = spec.SlotID
		}
		s.readings[oid] = r
	}
}

func (s *Session) objectByOID(oid int) *cfgtranslate.ObjectSpec {
	for i := range s.Objects {
		if s.Objects[i].OID == oid {
			return &s.Objects[i]
		}
	}
	return nil
}

func extractAnalogValue(params map[string]interface{}) float64 {
	if values, ok := params["values"].([]int); ok && len(values) >= 2 {
		// Raw bytes from dynamic string — two bytes little-endian uint16.
		v := float64(values[0]) + float64(values[1])*256
		return v / 1024.0
	}
	if v, ok := params["value"].(int64); ok {
		return float64(v) / 1024.0
	}
	return 0
}

// SetDigitalOut toggles a digital output OID.
func (s *Session) SetDigitalOut(oid int, value bool) error {
	v := 0
	if value {
		v = 1
	}
	if err := s.Conn.SendCommand(fmt.Sprintf("update_digital_out oid=%d value=%d", oid, v)); err != nil {
		return err
	}
	s.recordOutputLevel(oid, value)
	return nil
}

// SetServoAngle sets a servo PWM output (0-180 degrees).
func (s *Session) SetServoAngle(oid int, angle float64) error {
	if obj := s.objectByOID(oid); obj != nil && obj.Meta["pwm"] == "pca9685" {
		return s.setPCA9685Servo(obj, angle)
	}
	if angle < 0 {
		angle = 0
	}
	if angle > 180 {
		angle = 180
	}
	return s.queueServoPulse(oid, angle)
}

func (s *Session) stepperLock(oid int) *sync.Mutex {
	s.stepperLocksGuard.Lock()
	defer s.stepperLocksGuard.Unlock()
	if s.stepperLocks == nil {
		s.stepperLocks = make(map[int]*sync.Mutex)
	}
	m, ok := s.stepperLocks[oid]
	if !ok {
		m = &sync.Mutex{}
		s.stepperLocks[oid] = m
	}
	return m
}

// StepperJog moves a stepper continuously or for a fixed step count.
// If the stepper is already jogging continuously, direction and speed are updated in place.
func (s *Session) StepperJog(oid int, forward bool, speedHz float64, steps int) error {
	s.stepperLock(oid).Lock()
	defer s.stepperLock(oid).Unlock()

	if err := s.ensureMCUReady(); err != nil {
		return err
	}
	s.stopStepperRoutineLocked(oid)

	continuous := steps <= 0
	if err := s.stepperJogMove(oid, forward, speedHz, steps); err != nil {
		s.stopJogKeeper(oid)
		s.setStepperJogging(oid, false)
		return err
	}
	if continuous {
		s.setStepperJogging(oid, true)
		s.startJogKeeper(oid, forward, speedHz)
	} else {
		s.setStepperJogging(oid, false)
		s.stopJogKeeper(oid)
	}
	return nil
}

func (s *Session) stepperJogMove(oid int, forward bool, speedHz float64, steps int) error {
	stopTrsyncOID, err := s.stopTrsyncForStepper(oid)
	if err != nil {
		return err
	}
	speedHz = clampJogSpeed(speedHz)
	dir := 0
	if forward {
		dir = 1
	}
	interval := int(s.ClockFreq / speedHz)
	if interval < 1 {
		interval = 1
	}

	startClock, err := s.sendAtMotionClock(motionStartMarginSec, func(startClock uint64) []string {
		cmds := s.stepperStartCommands(oid, stopTrsyncOID, startClock)
		cmds = append(cmds, fmt.Sprintf("set_next_step_dir oid=%d dir=%d", oid, dir))
		return append(cmds, queueStepCommands(oid, interval, steps)...)
	})
	if err != nil {
		return err
	}
	if err := s.failIfShutdown(); err != nil {
		return err
	}
	slog.Info("stepper jog scheduled",
		"port", s.Port,
		"oid", oid,
		"forward", forward,
		"speed_hz", speedHz,
		"start_clock", startClock,
		"interval", interval,
		"steps", steps,
	)
	return nil
}

func (s *Session) isStepperJogging(oid int) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stepperJogging[oid]
}

func (s *Session) setStepperJogging(oid int, jogging bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if jogging {
		s.stepperJogging[oid] = true
	} else {
		delete(s.stepperJogging, oid)
	}
}

// RefreshStepperReadings queries position for all configured steppers.
func (s *Session) RefreshStepperReadings() {
	var oids []int
	s.mu.RLock()
	for _, obj := range s.Objects {
		if obj.Kind == cfgtranslate.KindStepper {
			oids = append(oids, obj.OID)
		}
	}
	s.mu.RUnlock()
	for _, oid := range oids {
		_, _ = s.StepperPosition(oid)
	}
}

// StepperStop halts an in-progress stepper move or routine.
func (s *Session) StepperStop(oid int) error {
	s.stepperLock(oid).Lock()
	defer s.stepperLock(oid).Unlock()

	s.stopStepperRoutineLocked(oid)
	return s.stepperStopUnlocked(oid)
}

func (s *Session) stopTrsyncForStepper(stepperOID int) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	stopTrsyncOID, ok := s.stopTrsyncOIDs[stepperOID]
	if !ok {
		return 0, fmt.Errorf("stepper oid %d: stop trsync not configured", stepperOID)
	}
	return stopTrsyncOID, nil
}

// StepperPosition queries the current stepper position.
func (s *Session) StepperPosition(oid int) (int64, error) {
	params, err := s.Conn.QueryCommand(fmt.Sprintf("stepper_get_position oid=%d", oid), "stepper_position")
	if err != nil {
		return 0, err
	}
	pos, _ := params["pos"].(int64)
	return pos, nil
}

// QueryEndstop requests endstop state update.
func (s *Session) QueryEndstop(oid int) error {
	_, err := s.Conn.QueryCommand(fmt.Sprintf("endstop_query_state oid=%d", oid), "endstop_state")
	return err
}

// ObjectBySlot returns the object spec for a board slot ID.
func (s *Session) ObjectBySlot(slotID string) *cfgtranslate.ObjectSpec {
	for i := range s.Objects {
		if s.Objects[i].SlotID == slotID {
			return &s.Objects[i]
		}
	}
	return nil
}

// Parser returns the message parser for debugging.
func (s *Session) Parser() *msgproto.MessageParser {
	return s.Conn.Parser()
}
