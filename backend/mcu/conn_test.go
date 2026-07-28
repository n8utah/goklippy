package mcu

import (
	"testing"

	"goklippy/backend/mcu/msgproto"

	"github.com/stretchr/testify/require"
)

func TestNextSeqStartsAtZero(t *testing.T) {
	c := &Conn{seq: 0, parser: msgproto.NewMessageParser("")}
	require.Equal(t, 0, c.nextSeq())
	require.Equal(t, 1, c.nextSeq())

	cmd := c.parser.Create_command("identify offset=0 count=40")
	require.NotEmpty(t, cmd)
	block := c.parser.Encode_msgblock(0, cmd)
	require.Equal(t, 0x10, block[1], "first host block must use seq nibble 0 (wire byte 0x10)")
}
