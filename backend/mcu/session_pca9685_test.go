package mcu

import (
	"encoding/hex"
	"fmt"
	"os"
	"testing"

	"goklippy/backend/mcu/msgproto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPCAPrescaleAndCounts(t *testing.T) {
	// 25 MHz / (4096 * 50) = 122.07 -> 121, as in the sorter firmware.
	assert.Equal(t, 121, pcaPrescale(50))
	// The real period is 122 * 4096 / 25 MHz = 19.99 ms, so 1.5 ms is 307 of 4096.
	assert.Equal(t, 307, pcaServoCounts(0.0015))
	assert.Equal(t, 102, pcaServoCounts(0.0005))
	assert.Equal(t, 512, pcaServoCounts(0.0025))
	assert.Equal(t, 1, pcaServoCounts(0))
	assert.Equal(t, 4095, pcaServoCounts(1))
}

func TestPCAReadState(t *testing.T) {
	present, setUp := pcaReadState(0xff, 0xff) // empty bus: pull-ups read high
	assert.False(t, present)
	assert.False(t, setUp)

	present, setUp = pcaReadState(0x11, 0x1e) // power-on defaults: asleep, 200 Hz
	assert.True(t, present)
	assert.False(t, setUp)

	present, setUp = pcaReadState(0x20, 121) // after our setup
	assert.True(t, present)
	assert.True(t, setUp)

	present, setUp = pcaReadState(0x30, 121) // right prescale but asleep
	assert.True(t, present)
	assert.False(t, setUp)
}

func TestServoPulse(t *testing.T) {
	assert.InDelta(t, 0.0005, servoPulse(0, 0.0005, 0.0025, 180), 1e-12)
	assert.InDelta(t, 0.0015, servoPulse(90, 0.0005, 0.0025, 180), 1e-12)
	assert.InDelta(t, 0.0025, servoPulse(200, 0.0005, 0.0025, 180), 1e-12)
	assert.InDelta(t, 0.0005, servoPulse(-10, 0.0005, 0.0025, 180), 1e-12)
	assert.InDelta(t, 0.0015, servoPulse(135, 0.001, 0.002, 270), 1e-12)
}

func TestPCAChannelWrite(t *testing.T) {
	// LED9_ON_L = 0x06 + 4*9 = 0x2A; off count 307 = 0x133.
	assert.Equal(t, []byte{0x2A, 0, 0, 0x33, 0x01}, pcaChannelWrite(9, 307))
	init := pcaInitWrites()
	assert.Equal(t, []byte{pcaRegPreScale, 121}, init[1])
	assert.Equal(t, []byte{pcaRegAllLEDOnL, 0, 0, 0, 0x10}, init[len(init)-1])
}

// The commands encode against a real Klipper firmware dictionary (atmega2560 build).
func TestPCACommandsEncodeWithFirmwareDictionary(t *testing.T) {
	dict, err := os.ReadFile("../testdata/atmega2560-klipper.dict")
	require.NoError(t, err)
	p := msgproto.NewMessageParser("")
	p.Process_identify(dict, false)

	require.NotEmpty(t, p.Create_command("config_i2c oid=3"))
	require.NotEmpty(t, p.Create_command("i2c_set_bus oid=3 i2c_bus=twi rate=100000 address=64"))
	require.Empty(t, p.Create_command("i2c_set_bus oid=3 i2c_bus=i2c9 rate=100000 address=64"), "unknown bus must not panic")

	data := pcaChannelWrite(9, 307)
	cmd := p.Create_command(fmt.Sprintf("i2c_transfer oid=3 write=%s read_len=0", hex.EncodeToString(data)))
	require.NotEmpty(t, cmd)
	block := append(make([]int, msgproto.MESSAGE_HEADER_SIZE), cmd...)
	params := p.Parse(append(block, make([]int, msgproto.MESSAGE_TRAILER_SIZE)...))
	assert.Equal(t, "i2c_transfer", params["#name"])
	assert.Equal(t, []int{0x2A, 0, 0, 0x33, 0x01}, params["write"])
	assert.EqualValues(t, 0, params["read_len"])
}
