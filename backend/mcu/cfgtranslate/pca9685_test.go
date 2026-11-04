package cfgtranslate

import (
	"testing"

	"goklippy/backend/klipperconfig"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildFromText(t *testing.T, text string) (BuildResult, error) {
	t.Helper()
	f, err := klipperconfig.Parse(text)
	require.NoError(t, err)
	return Build(f)
}

func TestBuildPCA9685Servos(t *testing.T) {
	// The servo comes before its board: boards are configured first either way.
	result, err := buildFromText(t, `
[servo door0]
pin: servo_board:pwm0
minimum_pulse_width: 0.0005
maximum_pulse_width: 0.0025

[pca9685 servo_board]
i2c_address: 0x41

[servo door1]
pin: servo_board:9

[servo gpio]
pin: ar11
`)
	require.NoError(t, err)

	assert.Equal(t, []string{
		"config_i2c oid=0",
		"i2c_set_bus oid=0 i2c_bus=twi rate=100000 address=65",
		"config_digital_out oid=1 pin=ar11 value=0 default_value=0 max_duration=0",
	}, result.Commands)
	// Only the GPIO servo gets a software PWM cycle.
	assert.Equal(t, []string{"set_digital_out_pwm_cycle oid=1 cycle_ticks=320000"}, result.InitCommands)
	assert.Equal(t, "allocate_oids count=2", FinalizeCommands(result.Commands)[0])

	byName := map[string]ObjectSpec{}
	for _, obj := range result.Objects {
		byName[obj.Section] = obj
	}
	board := byName["pca9685 servo_board"]
	assert.Equal(t, KindI2C, board.Kind)
	assert.Equal(t, "servo_board", board.Label)

	door0 := byName["servo door0"]
	assert.Equal(t, KindPWMOut, door0.Kind)
	assert.Equal(t, VirtualOIDBase, door0.OID)
	assert.Equal(t, "servo", door0.Meta["device"])
	assert.Equal(t, "pca9685", door0.Meta["pwm"])
	assert.Equal(t, "0", door0.Meta["board_oid"])
	assert.Equal(t, "0", door0.Meta["channel"])
	assert.Equal(t, "0.0005", door0.Meta["min_pulse"])
	assert.Equal(t, "0.0025", door0.Meta["max_pulse"])

	door1 := byName["servo door1"]
	assert.Equal(t, VirtualOIDBase+1, door1.OID)
	assert.Equal(t, "9", door1.Meta["channel"])
	assert.Equal(t, "0.001", door1.Meta["min_pulse"])
	assert.Equal(t, "180", door1.Meta["max_angle"])

	assert.Equal(t, 1, byName["servo gpio"].OID)
}

func TestBuildPCA9685SoftwareBus(t *testing.T) {
	result, err := buildFromText(t, `
[pca9685 b]
i2c_software_scl_pin: ar21
i2c_software_sda_pin: ar20
i2c_speed: 400000
`)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"config_i2c oid=0",
		"i2c_set_sw_bus oid=0 scl_pin=ar21 sda_pin=ar20 pulse_ticks=20 address=64",
	}, result.Commands)
}

func TestBuildPCA9685Errors(t *testing.T) {
	cases := map[string]string{
		"no board":          "[servo s]\npin: missing:pwm0\n",
		"channel too high":  "[pca9685 b]\n[servo s]\npin: b:pwm16\n",
		"bad channel":       "[pca9685 b]\n[servo s]\npin: b:left\n",
		"bad address":       "[pca9685 b]\ni2c_address: 200\n",
		"one software pin":  "[pca9685 b]\ni2c_software_scl_pin: ar21\n",
		"pulse range":       "[pca9685 b]\n[servo s]\npin: b:pwm0\nminimum_pulse_width: 0.002\nmaximum_pulse_width: 0.001\n",
		"pulse not numeric": "[pca9685 b]\n[servo s]\npin: b:pwm0\nmaximum_pulse_width: wide\n",
	}
	for name, text := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := buildFromText(t, text)
			assert.Error(t, err)
		})
	}
}
