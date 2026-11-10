package routes

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"goklippy/backend/logic/camera"
	"goklippy/backend/logic/sorter"

	"github.com/gin-gonic/gin"
)

var (
	sorterFeed       sorter.CameraFeed
	sorterPerception *sorter.Perception // nil in simulation
)

// InitCameras wires the camera feed in. perception is nil when simulating.
func InitCameras(feed sorter.CameraFeed, perception *sorter.Perception) {
	sorterFeed = feed
	sorterPerception = perception
}

func cameraChannel(c *gin.Context) (sorter.Channel, bool) {
	ch, err := sorter.ParseChannel(c.Param("channel"))
	if err == nil && (ch == sorter.ChannelC2 || ch == sorter.ChannelC3 || ch == sorter.ChannelC4) {
		return ch, true
	}
	c.JSON(http.StatusNotFound, gin.H{"error": sorter.ErrNotCameraCh.Error()})
	return "", false
}

// GetSorterCameras GET /sorter/cameras: each camera channel's state.
func GetSorterCameras(c *gin.Context) {
	if sorterPerception == nil {
		c.JSON(http.StatusOK, gin.H{"simulated": true, "channels": gin.H{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"simulated": false, "channels": sorterPerception.Statuses()})
}

// ListSorterCameraDevices GET /sorter/cameras/devices: what ffmpeg can open.
func ListSorterCameraDevices(c *gin.Context) {
	devices, err := camera.ListDevices(c.Request.Context(), sorterSettings.Machine().Cameras.Ffmpeg)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"devices": []string{}, "error": err.Error()})
		return
	}
	if devices == nil {
		devices = []string{}
	}
	c.JSON(http.StatusOK, gin.H{"devices": devices})
}

// GetSorterCameraFrame GET /sorter/cameras/:channel/frame: the newest JPEG.
func GetSorterCameraFrame(c *gin.Context) {
	ch, ok := cameraChannel(c)
	if !ok {
		return
	}
	jpg, _, ok := sorterFeed.JPEG(ch)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": sorter.ErrNoFrame.Error()})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "image/jpeg", jpg)
}

const streamPoll = 50 * time.Millisecond

// StreamSorterCamera GET /sorter/cameras/:channel/stream: MJPEG for an <img>.
func StreamSorterCamera(c *gin.Context) {
	ch, ok := cameraChannel(c)
	if !ok {
		return
	}
	// A stream runs as long as it's watched; lift the server's write timeout.
	_ = http.NewResponseController(c.Writer).SetWriteDeadline(time.Time{})
	const boundary = "goklippyframe"
	c.Header("Content-Type", "multipart/x-mixed-replace; boundary="+boundary)
	c.Header("Cache-Control", "no-store")
	c.Status(http.StatusOK)

	ticker := time.NewTicker(streamPoll)
	defer ticker.Stop()
	var last time.Time
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
		}
		jpg, at, ok := sorterFeed.JPEG(ch)
		if !ok || !at.After(last) {
			continue
		}
		last = at
		if _, err := fmt.Fprintf(c.Writer, "--%s\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", boundary, len(jpg)); err != nil {
			return
		}
		if _, err := c.Writer.Write(jpg); err != nil {
			return
		}
		if _, err := c.Writer.Write([]byte("\r\n")); err != nil {
			return
		}
		c.Writer.Flush()
	}
}

// GetSorterCameraObservation GET /sorter/cameras/:channel/observation: what
// perception sees now, with the zones it used, for drawing over the stream.
func GetSorterCameraObservation(c *gin.Context) {
	ch, ok := cameraChannel(c)
	if !ok {
		return
	}
	obs := sorterFeed.Observation(ch)
	zones, hasZones := sorterSettings.Machine().Cameras.Zones[string(ch)]
	if sorterPerception == nil {
		zones, hasZones = sorter.SimZones(), true
	}
	resp := gin.H{"observation": obs}
	if hasZones {
		resp["zones"] = zones
	}
	c.JSON(http.StatusOK, resp)
}

// CaptureSorterBackground POST /sorter/cameras/:channel/background: the newest frame
// becomes the empty channel the detector compares against.
func CaptureSorterBackground(c *gin.Context) {
	ch, ok := cameraChannel(c)
	if !ok {
		return
	}
	if sorterPerception == nil {
		c.JSON(http.StatusOK, gin.H{"ok": true, "simulated": true})
		return
	}
	err := sorterPerception.CaptureBackground(ch)
	switch {
	case errors.Is(err, sorter.ErrNoCamera), errors.Is(err, sorter.ErrNoFrame):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case err != nil:
		sorterError(c, err)
	default:
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}
