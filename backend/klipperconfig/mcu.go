package klipperconfig

import "strconv"

// MCUBaud returns the serial baud rate from [mcu] or 0 when unset/invalid.
func MCUBaud(f *File) int {
	if f == nil {
		return 0
	}
	sec, ok := f.Get("mcu")
	if !ok {
		return 0
	}
	raw := sec.Options["baud"]
	if raw == "" {
		return 0
	}
	baud, err := strconv.Atoi(raw)
	if err != nil || baud <= 0 {
		return 0
	}
	return baud
}
