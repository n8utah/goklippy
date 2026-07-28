package cfgtranslate

import (
	"hash/crc32"
	"strings"
)

// Crc32Lines matches Klipper host CRC over joined config command lines.
func Crc32Lines(lines []string) uint32 {
	payload := strings.Join(lines, "\n")
	return crc32.ChecksumIEEE([]byte(payload)) & 0xffffffff
}
