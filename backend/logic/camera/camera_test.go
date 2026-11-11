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
	// ffmpeg 9 on this laptop, with a second identical USB camera added.
	out := "[in#0 @ 00000191b0d31a40] \"Laptop Camera\" (video)\r\n" +
		"[in#0 @ 00000191b0d31a40]   Alternative name \"@device_pnp_\\\\?\\usb#vid_0bda&pid_5634&mi_00#7&125ae00b&0&0000#{65e8773d-8f56-11d0-a3b9-00a0c9223196}\\global\"\r\n" +
		"[in#0 @ 00000191b0d31a40] \"USB Camera\" (video)\r\n" +
		"[in#0 @ 00000191b0d31a40]   Alternative name \"@device_pnp_\\\\?\\usb#vid_0bda&pid_5844&mi_00#7&3390689c&0&0000#{65e8773d-8f56-11d0-a3b9-00a0c9223196}\\global\"\r\n" +
		"[in#0 @ 00000191b0d31a40] \"USB Camera\" (video)\r\n" +
		"[in#0 @ 00000191b0d31a40]   Alternative name \"@device_pnp_\\\\?\\usb#vid_0bda&pid_5844&mi_00#7&1f2e3d4c&0&0000#{65e8773d-8f56-11d0-a3b9-00a0c9223196}\\global\"\r\n" +
		"[in#0 @ 00000191b0d31a40] \"Microphone Array (Realtek(R) Audio)\" (audio)\r\n" +
		"[in#0 @ 00000191b0d31a40]   Alternative name \"@device_cm_{33D9A762-90C8-11D0-BD43-00A0C911CE86}\\wave_{A9BF}\"\r\n" +
		"Error opening input file dummy.\r\n"
	devices := ParseDshowDevices(out)
	require.Len(t, devices, 3, "identical cameras stay separate; audio is left out")
	assert.Equal(t, "Laptop Camera", devices[0].Name)
	assert.Equal(t, "USB Camera", devices[1].Name)
	assert.Equal(t, "USB Camera", devices[2].Name)
	assert.Equal(t, `@device_pnp_\\?\usb#vid_0bda&pid_5844&mi_00#7&3390689c&0&0000#{65e8773d-8f56-11d0-a3b9-00a0c9223196}\global`, devices[1].Path)
	assert.Equal(t, devices[1].Path, devices[1].ID)
	assert.Equal(t, "0bda:5844 · 3390689c", devices[1].Hint)
	assert.Equal(t, "0bda:5844 · 1f2e3d4c", devices[2].Hint)
}

func TestParseDshowDevicesOldFfmpeg(t *testing.T) {
	out := "[dshow @ 03f8] DirectShow video devices\r\n" +
		"[dshow @ 03f8]  \"Laptop Camera\"\r\n" +
		"[dshow @ 03f8]  \"USB Camera\"\r\n" +
		"[dshow @ 03f8] DirectShow audio devices\r\n" +
		"[dshow @ 03f8]  \"Microphone Array\"\r\n"
	assert.Equal(t, []Device{
		{Name: "Laptop Camera", ID: "Laptop Camera"},
		{Name: "USB Camera", ID: "USB Camera"},
	}, ParseDshowDevices(out))
}

func TestParseDshowModes(t *testing.T) {
	// From the USB camera on this laptop.
	out := "[in#0 @ 0000022d] DirectShow video device options (from video devices)\r\n" +
		"[in#0 @ 0000022d]  Pin \"Capture\" (alternative pin name \"0\")\r\n" +
		"[in#0 @ 0000022d]   vcodec=mjpeg  min s=1280x720 fps=30 max s=1280x720 fps=30\r\n" +
		"[in#0 @ 0000022d]   vcodec=mjpeg  min s=1280x720 fps=30 max s=1280x720 fps=30 (pc, bt470bg/bt709/unknown, center)\r\n" +
		"[in#0 @ 0000022d]   vcodec=mjpeg  min s=640x480 fps=30 max s=640x480 fps=30\r\n" +
		"[in#0 @ 0000022d]   pixel_format=yuyv422  min s=848x480 fps=15 max s=848x480 fps=15\r\n"
	assert.Equal(t, []Mode{
		{Width: 1280, Height: 720, Fps: 30, Format: "mjpeg"},
		{Width: 640, Height: 480, Fps: 30, Format: "mjpeg"},
		{Width: 848, Height: 480, Fps: 15, Format: "yuyv422"},
	}, ParseDshowModes(out))
}

func TestArgs(t *testing.T) {
	args := strings.Join(Config{Device: "lavfi:testsrc", Width: 320, Height: 240, Fps: 10}.args(), " ")
	assert.Contains(t, args, "-f lavfi -i testsrc=size=320x240:rate=10")
	assert.True(t, strings.HasSuffix(args, "-f mjpeg -q:v 4 -"))

	args = strings.Join(Config{Device: "USB Camera", Width: 640, Height: 480, Fps: 15}.args(), " ")
	if runtime.GOOS == "windows" {
		assert.Contains(t, args, "-f dshow -video_size 640x480 -i video=USB Camera -r 15")
	} else {
		assert.Contains(t, args, "-f v4l2")
	}
}
