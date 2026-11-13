package mcu

import (
	"encoding/hex"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"time"

	"goklippy/backend/mcu/cfgtranslate"
)

// PCA9685 servo boards on the MCU's I2C bus (see cfgtranslate/pca9685.go). The MCU
// only relays I2C transfers; the chip keeps sending each servo's pulse on its own
// once its registers are set.

const (
	pcaRegMode1      = 0x00
	pcaRegMode2      = 0x01
	pcaRegLED0OnL    = 0x06
	pcaRegAllLEDOnL  = 0xFA
	pcaRegPreScale   = 0xFE
	pcaMode1Sleep    = 0x10
	pcaMode1AutoIncr = 0x20
	pcaMode2OutDrv   = 0x04 // totem-pole outputs, what servos want
	pcaFullOff       = 0x10 // bit 4 of LEDn_OFF_H: output held low, no pulse
	pcaOscHz         = 25_000_000
	pcaSteps         = 4096
	pcaServoHz       = 50
)

// pcaPrescale is the PRE_SCALE value for freq (datasheet: round(osc / (4096 * freq)) - 1).
func pcaPrescale(freq float64) int {
	return int(math.Round(pcaOscHz/(pcaSteps*freq))) - 1
}

// pcaServoCounts is how many of the 4096 steps of the chip's actual period (set by
// the rounded prescale) a pulse of pulseSec lasts.
func pcaServoCounts(pulseSec float64) int {
	periodSec := float64(pcaPrescale(pcaServoHz)+1) * pcaSteps / pcaOscHz
	counts := int(math.Round(pulseSec / periodSec * pcaSteps))
	return min(max(counts, 1), pcaSteps-1)
}

// pcaChannelWrite sets one channel's pulse: on at count 0, off after counts.
func pcaChannelWrite(channel, counts int) []byte {
	return []byte{byte(pcaRegLED0OnL + 4*channel), 0, 0, byte(counts & 0xff), byte(counts >> 8 & 0x0f)}
}

// pcaInitWrites puts the chip at 50 Hz with every output off, like the sorter firmware:
// sleep to set the prescaler, wake with auto-increment, then all channels full off.
func pcaInitWrites() [][]byte {
	return [][]byte{
		{pcaRegMode1, pcaMode1Sleep},
		{pcaRegPreScale, byte(pcaPrescale(pcaServoHz))},
		{pcaRegMode1, pcaMode1AutoIncr},
		{pcaRegMode2, pcaMode2OutDrv},
		{pcaRegAllLEDOnL, 0, 0, 0, pcaFullOff},
	}
}

// servoPulse maps angle (0..maxAngle) onto minPulse..maxPulse seconds.
func servoPulse(angle, minPulse, maxPulse, maxAngle float64) float64 {
	angle = min(max(angle, 0), maxAngle)
	return minPulse + (maxPulse-minPulse)*angle/maxAngle
}

// i2cTransfer writes data to an I2C device and reads readLen bytes back. A missing or
// unpowered chip returns an error here; the MCU itself carries on.
func (s *Session) i2cTransfer(oid int, data []byte, readLen int) ([]int, error) {
	params, err := s.Conn.QueryCommand(fmt.Sprintf(
		"i2c_transfer oid=%d write=%s read_len=%d", oid, hex.EncodeToString(data), readLen,
	), "i2c_response")
	if err != nil {
		return nil, err
	}
	if status := fmt.Sprint(params["i2c_bus_status"]); status != "SUCCESS" && status != "0" {
		return nil, fmt.Errorf("i2c %s", status)
	}
	response, _ := params["response"].([]int)
	return response, nil
}

// InitPCA9685Boards sets up every PCA9685 board after connect. A board that doesn't
// answer is logged and tried again on its next servo move, so it can be plugged in or
// powered later without reconnecting.
func (s *Session) InitPCA9685Boards() {
	for _, obj := range s.Objects {
		if obj.Kind != cfgtranslate.KindI2C || obj.Meta["device"] != "pca9685" {
			continue
		}
		if err := s.initPCA9685(obj.OID); err != nil {
			slog.Warn("pca9685 not ready", "port", s.Port, "board", obj.Meta["board"], "address", obj.Meta["address"], "error", err)
		}
	}
}

func (s *Session) initPCA9685(oid int) error {
	s.pcaMu.Lock()
	defer s.pcaMu.Unlock()
	for _, data := range pcaInitWrites() {
		if _, err := s.i2cTransfer(oid, data, 0); err != nil {
			return err
		}
	}
	// AVR's I2C driver ignores NACKs, so writes "succeed" with no chip there. Reading the
	// prescaler back proves one is: an empty bus reads 0xff.
	got, err := s.i2cTransfer(oid, []byte{pcaRegPreScale}, 1)
	if err != nil {
		return err
	}
	if want := pcaPrescale(pcaServoHz); len(got) != 1 || got[0] != want {
		return fmt.Errorf("no PCA9685 answering (prescale read back %v, want %d)", got, want)
	}
	if s.pcaReady == nil {
		s.pcaReady = map[int]bool{}
	}
	s.pcaReady[oid] = true
	slog.Info("pca9685 ready", "port", s.Port, "oid", oid)
	return nil
}

// PCA9685 board states reported by CheckPCA9685Boards.
const (
	PCA9685StateOK      = "ok"      // chip answered and is set up for servos
	PCA9685StateMissing = "missing" // nothing answered at the address
	PCA9685StateError   = "error"   // the I2C transfer itself failed
)

// PCA9685Status is one board's result from CheckPCA9685Boards.
type PCA9685Status struct {
	Board     string    `json:"board"`
	Address   string    `json:"address"`
	OID       int       `json:"oid"`
	State     string    `json:"state"`
	Message   string    `json:"message"`
	Mode1     int       `json:"mode1"`    // -1 when not read
	Prescale  int       `json:"prescale"` // -1 when not read
	CheckedAt time.Time `json:"checked_at"`
}

// pcaReadState sorts a MODE1/PRE_SCALE reading. An empty bus reads 0xff for both
// (the pull-ups); a chip that lost power comes back asleep at its default prescale.
func pcaReadState(mode1, prescale int) (present, setUp bool) {
	if mode1 == 0xff && prescale == 0xff {
		return false, false
	}
	return true, prescale == pcaPrescale(pcaServoHz) && mode1&pcaMode1Sleep == 0
}

// CheckPCA9685Boards reads every board's MODE1 and PRE_SCALE registers. A chip that
// answers but isn't set up (it was power-cycled) is set up again, which leaves its
// outputs off as they already were after the reset.
func (s *Session) CheckPCA9685Boards() []PCA9685Status {
	statuses := []PCA9685Status{}
	for _, obj := range s.Objects {
		if obj.Kind != cfgtranslate.KindI2C || obj.Meta["device"] != "pca9685" {
			continue
		}
		status := s.checkPCA9685(obj.OID)
		status.Board = obj.Meta["board"]
		status.Address = obj.Meta["address"]
		slog.Debug("pca9685 check", "port", s.Port, "board", status.Board, "address", status.Address,
			"state", status.State, "mode1", status.Mode1, "prescale", status.Prescale)
		statuses = append(statuses, status)
	}
	return statuses
}

func (s *Session) checkPCA9685(oid int) PCA9685Status {
	status := PCA9685Status{OID: oid, Mode1: -1, Prescale: -1, CheckedAt: time.Now().UTC()}
	fail := func(state, msg string) PCA9685Status {
		s.pcaMu.Lock()
		delete(s.pcaReady, oid)
		s.pcaMu.Unlock()
		status.State, status.Message = state, msg
		return status
	}

	s.pcaMu.Lock()
	mode1, err := s.i2cTransfer(oid, []byte{pcaRegMode1}, 1)
	var prescale []int
	if err == nil {
		prescale, err = s.i2cTransfer(oid, []byte{pcaRegPreScale}, 1)
	}
	s.pcaMu.Unlock()
	if err != nil {
		return fail(PCA9685StateError, fmt.Sprintf("I2C transfer failed: %v. If the MCU shut down, check SDA/SCL for a short and connect again.", err))
	}
	if len(mode1) != 1 || len(prescale) != 1 {
		return fail(PCA9685StateError, "the MCU sent back no register data")
	}
	status.Mode1, status.Prescale = mode1[0], prescale[0]

	present, setUp := pcaReadState(status.Mode1, status.Prescale)
	if !present {
		return fail(PCA9685StateMissing, "Nothing answered at this address. Check SDA (D20), SCL (D21), VCC and GND, and the address jumpers.")
	}
	if setUp {
		s.pcaMu.Lock()
		if s.pcaReady == nil {
			s.pcaReady = map[int]bool{}
		}
		s.pcaReady[oid] = true
		s.pcaMu.Unlock()
		status.State, status.Message = PCA9685StateOK, "Board answered and is set to 50 Hz."
		return status
	}
	if err := s.initPCA9685(oid); err != nil {
		return fail(PCA9685StateError, fmt.Sprintf("Board answered but setup failed: %v", err))
	}
	status.State, status.Message = PCA9685StateOK, "Board answered but had reset (power lost?), so it was set up again."
	status.Mode1, status.Prescale = pcaMode1AutoIncr, pcaPrescale(pcaServoHz)
	return status
}

func (s *Session) setPCA9685Servo(obj *cfgtranslate.ObjectSpec, angle float64) error {
	boardOID, err := strconv.Atoi(obj.Meta["board_oid"])
	if err != nil {
		return fmt.Errorf("%s: no board oid", obj.Section)
	}
	channel, _ := strconv.Atoi(obj.Meta["channel"])
	minPulse, _ := strconv.ParseFloat(obj.Meta["min_pulse"], 64)
	maxPulse, _ := strconv.ParseFloat(obj.Meta["max_pulse"], 64)
	maxAngle, _ := strconv.ParseFloat(obj.Meta["max_angle"], 64)
	if maxAngle <= 0 || maxPulse <= minPulse {
		return fmt.Errorf("%s: bad pulse range", obj.Section)
	}

	s.pcaMu.Lock()
	ready := s.pcaReady[boardOID]
	s.pcaMu.Unlock()
	if !ready {
		if err := s.initPCA9685(boardOID); err != nil {
			return fmt.Errorf("PCA9685 board %q (address %s) is not answering: %w", obj.Meta["board"], obj.Meta["address"], err)
		}
	}

	counts := pcaServoCounts(servoPulse(angle, minPulse, maxPulse, maxAngle))
	if _, err := s.i2cTransfer(boardOID, pcaChannelWrite(channel, counts), 0); err != nil {
		// Probably reset or unplugged: set it up again before the next move.
		s.pcaMu.Lock()
		delete(s.pcaReady, boardOID)
		s.pcaMu.Unlock()
		return fmt.Errorf("PCA9685 board %q: %w", obj.Meta["board"], err)
	}
	return nil
}
