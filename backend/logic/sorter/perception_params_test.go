package sorter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectorParamsScaleWithFrameSize(t *testing.T) {
	threshold, minArea, step := detectorParams(CameraSettings{Width: 640, Height: 480})
	assert.Equal(t, []int{90, 120, 2}, []int{threshold, minArea, step}, "the defaults are for 640×480")

	_, minArea, step = detectorParams(CameraSettings{Width: 1280, Height: 720})
	assert.Equal(t, 360, minArea, "3× the pixels, 3× the smallest piece")
	assert.Equal(t, 4, step, "twice as wide, every 4th pixel")

	_, minArea, step = detectorParams(CameraSettings{Width: 320, Height: 240})
	assert.Equal(t, 30, minArea)
	assert.Equal(t, 2, step, "never coarser than the default for a small frame")

	threshold, minArea, _ = detectorParams(CameraSettings{Width: 1280, Height: 720, Threshold: 70, MinArea: 200})
	assert.Equal(t, []int{70, 200}, []int{threshold, minArea}, "settings win")
}
