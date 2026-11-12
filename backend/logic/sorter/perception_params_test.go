package sorter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectorParamsScaleWithFrameSize(t *testing.T) {
	threshold, minArea, step := detectorParams(CameraSettings{}, 640, 480)
	assert.Equal(t, []int{90, 120, 2}, []int{threshold, minArea, step}, "the defaults are for 640×480")

	_, minArea, step = detectorParams(CameraSettings{}, 1280, 720)
	assert.Equal(t, 360, minArea, "3× the pixels, 3× the smallest piece")
	assert.Equal(t, 4, step, "twice as wide, every 4th pixel")

	_, minArea, step = detectorParams(CameraSettings{}, 320, 240)
	assert.Equal(t, 30, minArea)
	assert.Equal(t, 2, step, "never coarser than the default for a small frame")

	threshold, minArea, _ = detectorParams(CameraSettings{Threshold: 70, MinArea: 200}, 1280, 720)
	assert.Equal(t, []int{70, 200}, []int{threshold, minArea}, "settings win")
}

func TestCameraConfigPerChannelSize(t *testing.T) {
	cs := CameraSettings{Width: 640, Height: 480, Fps: 10, Sizes: map[string]CameraSize{"c4": {3840, 2160}}}

	c3 := cameraConfig(cs, ChannelC3, "cam3")
	assert.Equal(t, [2]int{640, 480}, [2]int{c3.Width, c3.Height}, "every camera's size")
	assert.False(t, c3.Split())

	c4 := cameraConfig(cs, ChannelC4, "cam4")
	assert.Equal(t, [2]int{3840, 2160}, [2]int{c4.Width, c4.Height}, "its own size")
	assert.True(t, c4.Split(), "tracked on a scaled copy")
	assert.Equal(t, [2]int{1280, 720}, [2]int{c4.DetectWidth, c4.DetectHeight})

	w, h := detectSize(2592, 1944)
	assert.Equal(t, [2]int{1280, 960}, [2]int{w, h}, "keeps the shape")
	w, h = detectSize(1280, 720)
	assert.Equal(t, [2]int{1280, 720}, [2]int{w, h}, "small enough already")

	assert.Equal(t, ChannelC4, channelOf(CameraSettings{Devices: map[string]string{"c4": "cam4"}}, "cam4"))
}
