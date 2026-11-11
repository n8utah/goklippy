package cfgtranslate

import (
	"fmt"
	"strconv"
	"strings"

	"goklippy/backend/klipperconfig"
)

const defaultClockFreq = 16000000

// Builder converts Klipper config sections into MCU config commands.
type Builder struct {
	nextOID        int
	nextVirtualOID int
	lines          []string
	objects        []ObjectSpec
	clockFreq      int
	pcaBoards      map[string]int // [pca9685 name] -> its config_i2c oid
}

// NewBuilder creates a config command builder.
func NewBuilder() *Builder {
	return &Builder{nextOID: 0, clockFreq: defaultClockFreq}
}

// BuildHardwareCommands generates MCU config lines for hardware sections.
func BuildHardwareCommands(f *klipperconfig.File) ([]string, error) {
	result, err := Build(f)
	if err != nil {
		return nil, err
	}
	return result.Commands, nil
}

// Build generates config commands, init commands, and the object registry.
func Build(f *klipperconfig.File) (BuildResult, error) {
	b := NewBuilder()
	if err := b.addPCA9685Boards(f); err != nil {
		return BuildResult{}, err
	}
	for _, name := range f.Order {
		sec := f.Sections[name]
		switch {
		case strings.HasPrefix(name, "stepper_"):
			if err := b.addStepper(name, sec); err != nil {
				return BuildResult{}, err
			}
		case name == "fan" || strings.HasPrefix(name, "output_pin "):
			if err := b.addDigitalOut(name, sec); err != nil {
				return BuildResult{}, err
			}
		case strings.HasPrefix(name, "heater_fan "):
			if err := b.addDigitalOut(name, sec); err != nil {
				return BuildResult{}, err
			}
		case strings.HasPrefix(name, "temperature_sensor "), strings.HasPrefix(name, "voltage_sensor "):
			// [voltage_sensor supply] (sensor_pin: analog5) reads the supply through a
			// divider; the sorter turns the reading into volts.
			if err := b.addAnalogIn(name, sec); err != nil {
				return BuildResult{}, err
			}
		case name == "heater_bed" || strings.HasPrefix(name, "extruder"):
			if err := b.addHeater(name, sec); err != nil {
				return BuildResult{}, err
			}
		case strings.HasPrefix(name, "servo "):
			if err := b.addServo(name, sec); err != nil {
				return BuildResult{}, err
			}
		case strings.HasPrefix(name, "stepper_") == false && strings.HasPrefix(name, "tmc"):
			// skip
		}
	}
	// endstop sections: [stepper_x] has endstop_pin, or dedicated endstop sections
	for _, name := range f.Order {
		sec := f.Sections[name]
		if pin := sec.Options["endstop_pin"]; pin != "" && strings.HasPrefix(name, "stepper_") {
			if err := b.addEndstop(name+"_endstop", map[string]string{"pin": pin}); err != nil {
				return BuildResult{}, err
			}
		}
	}
	if len(b.lines) == 0 {
		return BuildResult{}, fmt.Errorf("no hardware sections found to configure")
	}

	init := b.buildInitCommands()
	return BuildResult{
		Commands:     b.lines,
		InitCommands: init,
		Objects:      b.objects,
	}, nil
}

func (b *Builder) buildStepperInitCommands() []string {
	var init []string
	for _, obj := range b.objects {
		if obj.Kind == KindStepper {
			init = append(init, fmt.Sprintf("reset_step_clock oid=%d clock=0", obj.OID))
		}
	}
	return init
}

func (b *Builder) allocOID() int {
	oid := b.nextOID
	b.nextOID++
	return oid
}

func (b *Builder) addObject(section string, oid int, kind ObjectKind, meta map[string]string) {
	label := section
	if parts := strings.SplitN(section, " ", 2); len(parts) == 2 {
		label = parts[1]
	}
	b.objects = append(b.objects, ObjectSpec{
		Section: section,
		OID:     oid,
		Kind:    kind,
		Label:   label,
		Meta:    meta,
	})
}

func (b *Builder) cycleTicks(seconds float64) int {
	return int(float64(b.clockFreq) * seconds)
}

func (b *Builder) addStepper(name string, sec klipperconfig.Section) error {
	stepPin, ok := sec.Options["step_pin"]
	if !ok || stepPin == "" {
		return fmt.Errorf("%s: step_pin is required", name)
	}
	dirPin := sec.Options["dir_pin"]
	stepPulseTicks := int(float64(b.clockFreq) * 0.000002) // 2us step pulse
	oid := b.allocOID()
	line := fmt.Sprintf(
		"config_stepper oid=%d step_pin=%s dir_pin=%s invert_step=0 step_pulse_ticks=%d",
		oid, stepPin, dirPin, stepPulseTicks,
	)
	b.lines = append(b.lines, line)
	b.addObject(name, oid, KindStepper, map[string]string{
		"step_pin":   stepPin,
		"dir_pin":    dirPin,
		"microsteps": sec.Options["microsteps"],
	})

	if enablePin := sec.Options["enable_pin"]; enablePin != "" {
		if err := b.addStepperEnable(name, enablePin); err != nil {
			return err
		}
	}
	return nil
}

func (b *Builder) addStepperEnable(stepperName, enablePin string) error {
	trimmed := strings.TrimSpace(enablePin)
	activeLow := strings.HasPrefix(trimmed, "!")
	pinForMCU := trimmed
	if activeLow {
		pinForMCU = strings.TrimPrefix(trimmed, "!")
	}
	value := 1
	if activeLow {
		value = 0 // active-low enable: hold low to energize driver
	}
	oid := b.allocOID()
	b.lines = append(b.lines, fmt.Sprintf(
		"config_digital_out oid=%d pin=%s value=%d default_value=%d max_duration=0",
		oid, pinForMCU, value, value,
	))
	b.addObject(stepperName+"_enable", oid, KindDigitalOut, map[string]string{
		"pin":        pinForMCU,
		"stepper":    stepperName,
		"active_low": fmt.Sprintf("%v", activeLow),
	})
	return nil
}

// addDigitalOut configures an on/off output. As in Klipper's [output_pin]: "!" on the
// pin inverts it, value is its state once configured and shutdown_value its state
// after an MCU shutdown (both 0 or 1, default 0).
func (b *Builder) addDigitalOut(name string, sec klipperconfig.Section) error {
	pin := strings.TrimSpace(sec.Options["pin"])
	invert := strings.HasPrefix(pin, "!")
	pin = strings.TrimSpace(strings.TrimPrefix(pin, "!"))
	if pin == "" {
		return fmt.Errorf("%s: pin is required", name)
	}
	start, err := onOffOption(sec.Options, "value")
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	shutdown, err := onOffOption(sec.Options, "shutdown_value")
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	maxDuration, err := maxMCUDuration(sec.Options)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	if maxDuration > 0 && start != shutdown {
		return fmt.Errorf("%s: value must equal shutdown_value when maximum_mcu_duration is set", name)
	}
	level := func(on bool) int {
		if on != invert {
			return 1
		}
		return 0
	}
	oid := b.allocOID()
	b.lines = append(b.lines, fmt.Sprintf(
		"config_digital_out oid=%d pin=%s value=%d default_value=%d max_duration=%d",
		oid, pin, level(start), level(shutdown), b.cycleTicks(maxDuration),
	))
	meta := map[string]string{
		"pin":            pin,
		"invert":         fmt.Sprintf("%v", invert),
		"value":          fmt.Sprintf("%d", boolInt(start)),
		"shutdown_level": fmt.Sprintf("%d", level(shutdown)),
	}
	if maxDuration > 0 {
		meta["max_duration"] = strconv.FormatFloat(maxDuration, 'f', -1, 64)
	}
	b.addObject(name, oid, KindDigitalOut, meta)
	return nil
}

// maxMCUDurationLimit is Klipper's ceiling for maximum_mcu_duration.
const maxMCUDurationLimit = 5.0

// maxMCUDuration reads Klipper's maximum_mcu_duration: how long, in seconds, the
// board keeps an output away from its shutdown value without hearing from the host.
// Past that the board shuts down, which puts every output to its shutdown value. The
// host refreshes the output well inside it (see mcu/session_output.go). 0 is off.
func maxMCUDuration(opts map[string]string) (float64, error) {
	raw := strings.TrimSpace(opts["maximum_mcu_duration"])
	if raw == "" {
		return 0, nil
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || v < 0 || v > maxMCUDurationLimit {
		return 0, fmt.Errorf("maximum_mcu_duration must be between 0 and %g seconds, not %q", maxMCUDurationLimit, raw)
	}
	return v, nil
}

// onOffOption reads an output's 0/1 option; missing is off. Klipper allows a PWM
// fraction here, which an on/off output can't do.
func onOffOption(opts map[string]string, key string) (bool, error) {
	switch strings.TrimSpace(opts[key]) {
	case "", "0", "0.0", "0.", "False", "false":
		return false, nil
	case "1", "1.0", "1.", "True", "true":
		return true, nil
	default:
		return false, fmt.Errorf("%s must be 0 or 1 for an on/off output, not %q", key, opts[key])
	}
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (b *Builder) addAnalogIn(name string, sec klipperconfig.Section) error {
	pin := sec.Options["sensor_pin"]
	if pin == "" {
		return fmt.Errorf("%s: sensor_pin is required", name)
	}
	oid := b.allocOID()
	b.lines = append(b.lines, fmt.Sprintf("config_analog_in oid=%d pin=%s", oid, pin))
	b.addObject(name, oid, KindAnalogIn, map[string]string{"sensor_pin": pin})
	return nil
}

func (b *Builder) addHeater(name string, sec klipperconfig.Section) error {
	heaterPin := sec.Options["heater_pin"]
	sensorPin := sec.Options["sensor_pin"]
	if heaterPin == "" || sensorPin == "" {
		return fmt.Errorf("%s: heater_pin and sensor_pin are required", name)
	}
	pwmOID := b.allocOID()
	b.lines = append(b.lines, fmt.Sprintf(
		"config_pwm_out oid=%d pin=%s cycle_ticks=%d value=0 default_value=0 max_duration=0",
		pwmOID, heaterPin, b.cycleTicks(0.010),
	))
	b.addObject(name+"_heater", pwmOID, KindPWMOut, map[string]string{"pin": heaterPin})

	adcOID := b.allocOID()
	b.lines = append(b.lines, fmt.Sprintf("config_analog_in oid=%d pin=%s", adcOID, sensorPin))
	b.addObject(name+"_sensor", adcOID, KindAnalogIn, map[string]string{"sensor_pin": sensorPin})
	return nil
}

func (b *Builder) addServo(name string, sec klipperconfig.Section) error {
	pin := sec.Options["pin"]
	if pin == "" {
		return fmt.Errorf("%s: pin is required", name)
	}
	if board, channel, onBoard, err := parsePCA9685Pin(pin); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	} else if onBoard {
		return b.addPCA9685Servo(name, pin, board, channel, sec)
	}
	oid := b.allocOID()
	// RAMPS D11 (ar11/PB5) uses AVR Timer1, which Klipper reserves — hardware
	// config_pwm_out triggers shutdown. Use software PWM via digital_out instead
	// (same approach as Klipper's pwm_tool when hardware PWM is unavailable).
	b.lines = append(b.lines, fmt.Sprintf(
		"config_digital_out oid=%d pin=%s value=0 default_value=0 max_duration=0",
		oid, pin,
	))
	b.addObject(name, oid, KindPWMOut, map[string]string{"pin": pin, "device": "servo", "pwm": "software"})
	return nil
}

func (b *Builder) addEndstop(name string, opts map[string]string) error {
	pin, pullUp, invert := parseInputPin(opts["pin"])
	if pin == "" {
		return fmt.Errorf("%s: pin is required", name)
	}
	oid := b.allocOID()
	pullUpValue := 0
	if pullUp {
		pullUpValue = 1
	}
	b.lines = append(b.lines, fmt.Sprintf(
		"config_endstop oid=%d pin=%s pull_up=%d",
		oid, pin, pullUpValue,
	))
	// invert: the switch reads low when triggered, so homing waits for pin_value=0.
	b.addObject(name, oid, KindEndstop, map[string]string{"pin": pin, "invert": fmt.Sprintf("%v", invert)})
	return nil
}

// parseInputPin splits Klipper's input pin prefixes off a pin name: "^" enables the
// pull-up, "!" inverts the reading. "~" (pull-down) has no AVR equivalent and is
// dropped. "^!ar3" -> ("ar3", true, true).
func parseInputPin(raw string) (pin string, pullUp, invert bool) {
	pin = strings.TrimSpace(raw)
	for pin != "" {
		switch pin[0] {
		case '^':
			pullUp = true
		case '!':
			invert = true
		case '~', ' ':
		default:
			return pin, pullUp, invert
		}
		pin = pin[1:]
	}
	return pin, pullUp, invert
}

func (b *Builder) buildInitCommands() []string {
	init := b.buildStepperInitCommands()
	slotClock := 0
	for _, obj := range b.objects {
		switch obj.Kind {
		case KindAnalogIn:
			// Periodic ADC sampling (~2 Hz report rate).
			reportTicks := b.cycleTicks(0.500)
			sampleTicks := b.cycleTicks(0.001)
			init = append(init, fmt.Sprintf(
				"query_analog_in oid=%d clock=%d sample_ticks=%d sample_count=1 rest_ticks=%d bytes_per_report=2 min_value=0 max_value=65535 range_check_count=0",
				obj.OID, slotClock, sampleTicks, reportTicks,
			))
			slotClock += reportTicks
		case KindEndstop:
			init = append(init, fmt.Sprintf(
				"endstop_home oid=%d clock=0 sample_ticks=0 sample_count=0 rest_ticks=0 pin_value=0 trsync_oid=0 trigger_reason=0",
				obj.OID,
			))
		case KindPWMOut:
			if obj.Meta["device"] == "servo" && obj.Meta["pwm"] == "software" {
				init = append(init, fmt.Sprintf(
					"set_digital_out_pwm_cycle oid=%d cycle_ticks=%d",
					obj.OID, b.cycleTicks(0.020),
				))
			}
		}
	}
	return init
}

// FinalizeCommands prepends allocate_oids and appends finalize_config with CRC.
func FinalizeCommands(lines []string) []string {
	oidCount := countOIDs(lines)
	withAlloc := make([]string, 0, len(lines)+2)
	withAlloc = append(withAlloc, fmt.Sprintf("allocate_oids count=%d", oidCount))
	withAlloc = append(withAlloc, lines...)
	crc := Crc32Lines(withAlloc)
	out := append(withAlloc, fmt.Sprintf("finalize_config crc=%d", crc))
	return out
}

func countOIDs(lines []string) int {
	max := -1
	for _, line := range lines {
		const marker = " oid="
		i := strings.Index(line, marker)
		if i < 0 || !strings.HasPrefix(line, "config_") {
			continue
		}
		var oid int
		if _, err := fmt.Sscanf(line[i+1:], "oid=%d", &oid); err == nil && oid > max {
			max = oid
		}
	}
	if max < 0 {
		return 0
	}
	return max + 1
}
