package msgproto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCrc16_ccitt(t *testing.T) {
	// Klipper message block: length + seq only before CRC (minimal valid prefix).
	buf := []int{5, 0x10}
	crc := Crc16_ccitt(buf)
	require.Len(t, crc, 2)

	block := Encode_msgblock(0, []int{})
	assert.Equal(t, 5, block[0])
	assert.Equal(t, 0x10, block[1])
	assert.Equal(t, crc[0], block[2])
	assert.Equal(t, crc[1], block[3])
	assert.Equal(t, MESSAGE_SYNC, block[4])
}

func TestParseBufferHex(t *testing.T) {
	p := NewMessageParser("")
	assert.Equal(t, []int{0x06, 0x00, 0x00, 0xcd, 0x01}, p.Parse_buffer("060000cd01"))
	assert.Equal(t, []int{0xfe, 0x79}, p.Parse_buffer("FE79"))
	assert.Empty(t, p.Parse_buffer(""))
	assert.Empty(t, p.Parse_buffer("zz"))
}

func TestDefaultMessagesIdentifyCommand(t *testing.T) {
	p := NewMessageParser("")
	cmd := p.Create_command("identify offset=0 count=40")
	require.NotEmpty(t, cmd)

	block := p.Encode_msgblock(1, cmd)
	assert.GreaterOrEqual(t, len(block), MESSAGE_MIN)
	assert.Equal(t, MESSAGE_SYNC, block[len(block)-1])
}

func TestProcess_identify_roundTrip(t *testing.T) {
	// Minimal dictionary JSON (uncompressed) for unit test.
	raw := []byte(`{
		"version": "test-v1",
		"build_versions": "test-build",
		"commands": {},
		"responses": {"startup": 128},
		"config": {"CLOCK_FREQ": 16000000}
	}`)

	p := NewMessageParser("")
	p.Process_identify(raw, false)

	version, build := p.Get_version_info()
	assert.Equal(t, "test-v1", version)
	assert.Equal(t, "test-build", build)
	assert.Equal(t, 16000000, p.Get_constant_int("CLOCK_FREQ", nil))
}

func Encode_msgblock(seq int, cmd []int) []int {
	p := NewMessageParser("")
	return p.Encode_msgblock(seq, cmd)
}
