// Package camera captures USB cameras through ffmpeg: one ffmpeg process per camera
// writes MJPEG to its stdout, and each JPEG in it is a frame. No cgo.
package camera

import (
	"bufio"
	"bytes"
	"errors"
	"io"
)

// maxFrameBytes guards against a stream that never ends a JPEG.
const maxFrameBytes = 8 << 20

var errFrameTooLarge = errors.New("camera: a frame ran past 8 MB without ending")

// SplitJPEGs reads a stream of concatenated JPEGs (ffmpeg's -f mjpeg output) and
// calls fn with each complete one, from its start (FF D8) to its end (FF D9). Bytes
// between frames are skipped. fn must not keep the slice; it's reused.
func SplitJPEGs(r io.Reader, fn func(jpeg []byte) error) error {
	br := bufio.NewReaderSize(r, 256<<10)
	var frame bytes.Buffer
	inFrame := false
	var prev byte
	for {
		b, err := br.ReadByte()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		switch {
		case !inFrame:
			if prev == 0xFF && b == 0xD8 {
				inFrame = true
				frame.Reset()
				frame.WriteByte(0xFF)
				frame.WriteByte(0xD8)
				b = 0 // so this D8 can't pair with the next byte
			}
		default:
			frame.WriteByte(b)
			if prev == 0xFF && b == 0xD9 {
				inFrame = false
				if err := fn(frame.Bytes()); err != nil {
					return err
				}
				b = 0
			} else if frame.Len() > maxFrameBytes {
				return errFrameTooLarge
			}
		}
		prev = b
	}
}
