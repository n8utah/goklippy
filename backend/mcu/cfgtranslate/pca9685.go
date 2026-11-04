package cfgtranslate

import (
	"fmt"
	"strconv"
	"strings"

	"goklippy/backend/klipperconfig"
)

// PCA9685 servo boards on the MCU's I2C bus. A board is a section
//
//	[pca9685 servo_board]
//	i2c_bus: twi            # MCU bus name (AVR: twi); or i2c_software_scl_pin/_sda_pin
//	i2c_address: 64         # 0x40, the board default
//	i2c_speed: 100000
//
// and its servos are ordinary [servo] sections whose pin names the board and a
// channel, Klipper style: `pin: servo_board:pwm3` (or `servo_board:3`). The
// MCU only gets an I2C device; the host writes the chip's registers.

const (
	// KindI2C is an I2C device on the MCU (config_i2c).
	KindI2C ObjectKind = "i2c"

	// VirtualOIDBase numbers objects that have no MCU oid of their own, such as
	// PCA9685 servo channels. They sit well above real oids so the two never meet.
	VirtualOIDBase = 1000

	PCA9685Channels       = 16
	defaultPCA9685Address = 0x40
	defaultI2CSpeed       = 100000
	// The only hardware I2C bus on AVR (RAMPS / Arduino Mega: SCL ar21, SDA ar20).
	defaultI2CBus = "twi"

	// Klipper's [servo] defaults.
	defaultServoMinPulse = 0.001
	defaultServoMaxPulse = 0.002
	defaultServoMaxAngle = 180.0
)

// addPCA9685Boards configures every [pca9685 ...] section. It runs before the other
// sections so servos can name a board defined anywhere in the file.
func (b *Builder) addPCA9685Boards(f *klipperconfig.File) error {
	for _, name := range f.Order {
		if !strings.HasPrefix(name, "pca9685 ") {
			continue
		}
		if err := b.addPCA9685(name, f.Sections[name]); err != nil {
			return err
		}
	}
	return nil
}

func (b *Builder) addPCA9685(name string, sec klipperconfig.Section) error {
	board := strings.TrimSpace(strings.TrimPrefix(name, "pca9685 "))
	if board == "" {
		return fmt.Errorf("%s: board name is required, e.g. [pca9685 servo_board]", name)
	}
	address, err := optionInt(sec, "i2c_address", defaultPCA9685Address)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	if address < 0 || address > 0x7f {
		return fmt.Errorf("%s: i2c_address %d is not a 7-bit address", name, address)
	}
	speed, err := optionInt(sec, "i2c_speed", defaultI2CSpeed)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	if speed <= 0 {
		return fmt.Errorf("%s: i2c_speed must be positive", name)
	}

	oid := b.allocOID()
	b.lines = append(b.lines, fmt.Sprintf("config_i2c oid=%d", oid))
	meta := map[string]string{
		"device":  "pca9685",
		"board":   board,
		"address": strconv.Itoa(address),
	}
	scl, sda := sec.Options["i2c_software_scl_pin"], sec.Options["i2c_software_sda_pin"]
	switch {
	case scl != "" && sda != "":
		pulseTicks := b.clockFreq / (2 * speed)
		b.lines = append(b.lines, fmt.Sprintf(
			"i2c_set_sw_bus oid=%d scl_pin=%s sda_pin=%s pulse_ticks=%d address=%d",
			oid, scl, sda, pulseTicks, address,
		))
		meta["scl_pin"], meta["sda_pin"] = scl, sda
	case scl != "" || sda != "":
		return fmt.Errorf("%s: software I2C needs both i2c_software_scl_pin and i2c_software_sda_pin", name)
	default:
		bus := strings.TrimSpace(sec.Options["i2c_bus"])
		if bus == "" {
			bus = defaultI2CBus
		}
		b.lines = append(b.lines, fmt.Sprintf(
			"i2c_set_bus oid=%d i2c_bus=%s rate=%d address=%d",
			oid, bus, speed, address,
		))
		meta["bus"] = bus
	}
	if b.pcaBoards == nil {
		b.pcaBoards = map[string]int{}
	}
	if _, dup := b.pcaBoards[board]; dup {
		return fmt.Errorf("%s: board %q is defined twice", name, board)
	}
	b.pcaBoards[board] = oid
	b.addObject(name, oid, KindI2C, meta)
	return nil
}

// parsePCA9685Pin splits "board:pwm3" (or "board:3") into the board and channel.
// ok is false when the pin doesn't name a board at all (an ordinary MCU pin).
func parsePCA9685Pin(pin string) (board string, channel int, ok bool, err error) {
	board, rest, found := strings.Cut(strings.TrimSpace(pin), ":")
	if !found {
		return "", 0, false, nil
	}
	channel, err = strconv.Atoi(strings.TrimPrefix(strings.TrimSpace(rest), "pwm"))
	if err != nil || channel < 0 || channel >= PCA9685Channels {
		return board, 0, true, fmt.Errorf("pin %q: channel must be pwm0 to pwm%d", pin, PCA9685Channels-1)
	}
	return strings.TrimSpace(board), channel, true, nil
}

// addPCA9685Servo registers a servo on a board channel. It gets a virtual oid; the
// session drives it with I2C writes to the board's oid.
func (b *Builder) addPCA9685Servo(name, pin, board string, channel int, sec klipperconfig.Section) error {
	boardOID, ok := b.pcaBoards[board]
	if !ok {
		return fmt.Errorf("%s: pin %q names board %q, but there is no [pca9685 %s] section", name, pin, board, board)
	}
	minPulse, err := optionFloat(sec, "minimum_pulse_width", defaultServoMinPulse)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	maxPulse, err := optionFloat(sec, "maximum_pulse_width", defaultServoMaxPulse)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	if minPulse <= 0 || maxPulse <= minPulse || maxPulse >= 0.020 {
		return fmt.Errorf("%s: pulse widths must satisfy 0 < minimum_pulse_width < maximum_pulse_width < 0.020", name)
	}
	maxAngle, err := optionFloat(sec, "maximum_servo_angle", defaultServoMaxAngle)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	if maxAngle <= 0 {
		return fmt.Errorf("%s: maximum_servo_angle must be positive", name)
	}

	oid := VirtualOIDBase + b.nextVirtualOID
	b.nextVirtualOID++
	b.addObject(name, oid, KindPWMOut, map[string]string{
		"pin":       pin,
		"device":    "servo",
		"pwm":       "pca9685",
		"board":     board,
		"board_oid": strconv.Itoa(boardOID),
		"channel":   strconv.Itoa(channel),
		"min_pulse": strconv.FormatFloat(minPulse, 'f', -1, 64),
		"max_pulse": strconv.FormatFloat(maxPulse, 'f', -1, 64),
		"max_angle": strconv.FormatFloat(maxAngle, 'f', -1, 64),
	})
	return nil
}

func optionInt(sec klipperconfig.Section, key string, fallback int) (int, error) {
	raw := strings.TrimSpace(sec.Options[key])
	if raw == "" {
		return fallback, nil
	}
	// Base 0 accepts both 64 and 0x40.
	v, err := strconv.ParseInt(raw, 0, 32)
	if err != nil {
		return 0, fmt.Errorf("%s: %q is not a whole number", key, raw)
	}
	return int(v), nil
}

func optionFloat(sec klipperconfig.Section, key string, fallback float64) (float64, error) {
	raw := strings.TrimSpace(sec.Options[key])
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %q is not a number", key, raw)
	}
	return v, nil
}
