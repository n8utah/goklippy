package camera

import (
	"bytes"
	"image"
	"image/jpeg"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tinyJPEG(t *testing.T, shade uint8) []byte {
	t.Helper()
	img := image.NewGray(image.Rect(0, 0, 8, 8))
	for i := range img.Pix {
		img.Pix[i] = shade
	}
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, nil))
	return buf.Bytes()
}

func TestSplitJPEGs(t *testing.T) {
	a, b := tinyJPEG(t, 10), tinyJPEG(t, 200)
	var stream bytes.Buffer
	stream.WriteString("junk before\xff")
	stream.Write(a)
	stream.WriteString("\x00\x01 between ")
	stream.Write(b)
	stream.Write(a[:len(a)/2]) // a cut-off frame at the end is ignored

	var got [][]byte
	require.NoError(t, SplitJPEGs(&stream, func(j []byte) error {
		got = append(got, append([]byte(nil), j...))
		return nil
	}))
	require.Len(t, got, 2)
	assert.Equal(t, a, got[0])
	assert.Equal(t, b, got[1])
	for _, j := range got {
		_, err := jpeg.Decode(bytes.NewReader(j))
		assert.NoError(t, err)
	}
}

func TestParseDshowDevicesNewFfmpeg(t *testing.T) {
	out := `[in#0 @ 0000020] "Laptop Camera" (video)
[in#0 @ 0000020]   Alternative name "@device_pnp_\\?\usb#vid_0bda"
[in#0 @ 0000020] "USB Camera" (video)
[in#0 @ 0000020] "Microphone Array" (audio)
[in#0 @ 0000020]   Alternative name "@device_cm_{33D9A762}"
[in#0 @ 0000020] "USB Camera" (video)
dummy: Immediate exit requested`
	assert.Equal(t, []string{"Laptop Camera", "USB Camera"}, ParseDshowDevices(out))
}

func TestParseDshowDevicesOldFfmpeg(t *testing.T) {
	out := "[dshow @ 03f8] DirectShow video devices\r\n" +
		"[dshow @ 03f8]  \"Laptop Camera\"\r\n" +
		"[dshow @ 03f8] DirectShow audio devices\r\n" +
		"[dshow @ 03f8]  \"Microphone Array\"\r\n"
	assert.Equal(t, []string{"Laptop Camera"}, ParseDshowDevices(out))
}

func TestArgs(t *testing.T) {
	args := strings.Join(Config{Device: "lavfi:testsrc", Width: 320, Height: 240, Fps: 10}.args(), " ")
	assert.Contains(t, args, "-f lavfi -i testsrc=size=320x240:rate=10")
	assert.True(t, strings.HasSuffix(args, "-f mjpeg -q:v 4 -"))

	args = strings.Join(Config{Device: "USB Camera", Width: 640, Height: 480, Fps: 15}.args(), " ")
	if runtime.GOOS == "windows" {
		assert.Contains(t, args, "-f dshow -video_size 640x480 -framerate 15 -i video=USB Camera")
	} else {
		assert.Contains(t, args, "-f v4l2")
	}
}
