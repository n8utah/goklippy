package camera

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

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
	args := strings.Join(Config{Device: "lavfi:testsrc", Width: 320, Height: 240, Fps: 10}.args(""), " ")
	assert.Contains(t, args, "-f lavfi -i testsrc=size=320x240:rate=10")
	assert.True(t, strings.HasSuffix(args, "-f mjpeg -q:v 4 -"))

	args = strings.Join(Config{Device: "USB Camera", Width: 640, Height: 480, Fps: 15}.args(""), " ")
	if runtime.GOOS == "windows" {
		assert.Contains(t, args, "-f dshow -video_size 640x480 -i video=USB Camera -r 15")
	} else {
		assert.Contains(t, args, "-f v4l2")
	}
}

func TestArgsSplit(t *testing.T) {
	cfg := Config{Device: "4K Camera", Width: 3840, Height: 2160, Fps: 10, DetectWidth: 1280, DetectHeight: 720}
	require.True(t, cfg.Split())
	args := strings.Join(cfg.args("tcp://127.0.0.1:5000"), " ")
	if runtime.GOOS == "windows" {
		// 30 fps from the camera (assumed) to 10: keep every 3rd before decoding.
		assert.Contains(t, args, `-f dshow -vcodec mjpeg -video_size 3840x2160 -bsf:v noise=drop=mod(n\,3) -i video=4K Camera`)
	}
	cfg.SourceFps = 60
	assert.Equal(t, 6, cfg.keepEvery())
	cfg.SourceFps, cfg.Fps = 30, 30
	assert.Equal(t, 1, cfg.keepEvery(), "nothing to drop")
	assert.Contains(t, args, "-map 0:v -vf scale=1280:720 -r 10 -an -f mjpeg -q:v 4 -")
	assert.True(t, strings.HasSuffix(args, "-map 0:v -an -c:v copy -bsf:v mjpeg2jpeg -f mjpeg tcp://127.0.0.1:5000"), args)

	assert.False(t, Config{Width: 1280, Height: 720, DetectWidth: 1280, DetectHeight: 720}.Split(), "nothing to scale")
	assert.False(t, Config{Width: 1280, Height: 720}.Split())
}

// A split capture of the test picture keeps full-size frames next to the scaled ones.
func TestCaptureSplit(t *testing.T) {
	if _, err := exec.LookPath(ResolveFFmpeg("")); err != nil {
		t.Skip("no ffmpeg")
	}
	cam := New(Config{Device: "lavfi:testsrc", Width: 640, Height: 480, Fps: 10, DetectWidth: 320, DetectHeight: 240})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go cam.Run(ctx)
	require.Eventually(t, func() bool {
		_, a := cam.Latest()
		_, b := cam.LatestFull()
		return a && b
	}, 15*time.Second, 50*time.Millisecond)
	small, _ := cam.Latest()
	full, _ := cam.LatestFull()
	si, err := jpeg.DecodeConfig(bytes.NewReader(small.JPEG))
	require.NoError(t, err)
	fi, err := jpeg.DecodeConfig(bytes.NewReader(full.JPEG))
	require.NoError(t, err)
	assert.Equal(t, [2]int{320, 240}, [2]int{si.Width, si.Height})
	assert.Equal(t, [2]int{640, 480}, [2]int{fi.Width, fi.Height})
}
