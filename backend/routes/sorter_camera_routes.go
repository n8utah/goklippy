package routes

import (
	"bytes"
	"errors"
	"fmt"
	"image/jpeg"
	"net/http"
	"time"

	"goklippy/backend/logic/camera"
	"goklippy/backend/logic/sorter"
	"goklippy/backend/logic/vision"

	"github.com/gin-gonic/gin"
)

var (
	sorterFeed       sorter.CameraFeed
	sorterPerception *sorter.Perception // nil in simulation
	cameraFinder     = sorter.NewCameraFinder()
)

// InitCameras wires the camera feed in. perception is nil when simulating.
func InitCameras(feed sorter.CameraFeed, perception *sorter.Perception) {
	sorterFeed = feed
	sorterPerception = perception
}

// MeasureSorterRing POST /sorter/cameras/:channel/measure: finds the channel's ring
// (rim and hole) in its newest frame. It only measures; the page puts the ring into
// the zones and saves them.
func MeasureSorterRing(c *gin.Context) {
	ch, ok := cameraChannel(c)
	if !ok {
		return
	}
	jpg, _, ok := sorterFeed.JPEG(ch)
	if !ok {
		c.JSON(http.StatusConflict, gin.H{"error": sorter.ErrNoFrame.Error()})
		return
	}
	img, err := jpeg.Decode(bytes.NewReader(jpg))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	m, err := vision.MeasureRing(img)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	b := img.Bounds()
	c.JSON(http.StatusOK, gin.H{"measure": m, "frame_w": b.Dx(), "frame_h": b.Dy()})
}

type roundRequest struct {
	Step float64 `json:"step"` // degrees between pictures; 0 uses 2
}

// CaptureSorterRound POST /sorter/cameras/:channel/background/round {"step": 2}: a
// job turns the channel all the way round, capturing it empty every step degrees,
// so a patterned rotor's turning doesn't look like pieces.
func CaptureSorterRound(c *gin.Context) {
	ch, ok := cameraChannel(c)
	if !ok {
		return
	}
	if sorterPerception == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "the machine is simulated: there are no cameras"})
		return
	}
	var req roundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if err := sorterMachine.CaptureViews(c.Request.Context(), ch, sorterPerception, req.Step); err != nil {
		sorterError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// FindSorterCameras POST /sorter/cameras/find {"devices": [...], "degrees": 15}: turns
// each channel a little and back while watching the cameras, to propose which camera
// is where. It runs as a sorter job; GET shows the result.
func FindSorterCameras(c *gin.Context) {
	if sorterPerception == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "the machine is simulated: there are no cameras to find"})
		return
	}
	var req sorter.CameraFindRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	for _, d := range req.Devices {
		if !sorterPerception.MayOpen(d) {
			c.JSON(http.StatusNotFound, gin.H{"error": sorter.ErrUnknownDevice.Error()})
			return
		}
	}
	if err := sorterMachine.FindCameras(c.Request.Context(), sorterPerception, cameraFinder, req); err != nil {
		sorterError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GetSorterCameraFind GET /sorter/cameras/find: the last search's result.
func GetSorterCameraFind(c *gin.Context) {
	r, err := cameraFinder.Last()
	if errors.Is(err, sorter.ErrNotFinding) {
		c.JSON(http.StatusOK, gin.H{"result": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": r})
}

// GetSorterCameraFindImage GET /sorter/cameras/find/:channel/:which (before|after):
// the best camera's picture before or after that channel turned.
func GetSorterCameraFindImage(c *gin.Context) {
	ch, err := sorter.ParseChannel(c.Param("channel"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	jpg, err := cameraFinder.Image(ch, c.Param("which") == "after")
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "image/jpeg", jpg)
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

// cameraDevice is a found camera and the channels set to use it.
type cameraDevice struct {
	camera.Device
	Channels []string `json:"channels"`
}

// ListSorterCameraDevices GET /sorter/cameras/devices: the cameras ffmpeg can open,
// which ffmpeg, and which channel each is assigned to. Assignments saved by name
// (before device paths were known) match every camera of that name.
func ListSorterCameraDevices(c *gin.Context) {
	cs := sorterSettings.Machine().Cameras
	list, err := camera.ListDevices(c.Request.Context(), cs.Ffmpeg)
	devices := make([]cameraDevice, 0, len(list.Devices))
	ids := make([]string, 0, len(list.Devices))
	for _, d := range list.Devices {
		cd := cameraDevice{Device: d, Channels: []string{}}
		for _, ch := range sorter.CameraChannels {
			if assigned := cs.Devices[string(ch)]; assigned != "" && (assigned == d.ID || assigned == d.Name) {
				cd.Channels = append(cd.Channels, string(ch))
			}
		}
		devices = append(devices, cd)
		ids = append(ids, d.ID)
	}
	if sorterPerception != nil {
		sorterPerception.NoteDevices(ids)
	}
	resp := gin.H{"devices": devices, "ffmpeg": list.FFmpeg, "version": list.Version}
	if list.Warning != "" {
		resp["warning"] = list.Warning
	}
	if err != nil {
		resp["error"] = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// previewDevice reads ?device= for the preview routes.
func previewDevice(c *gin.Context) (string, bool) {
	if sorterPerception == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "the machine is simulated: there are no cameras to preview"})
		return "", false
	}
	device := c.Query("device")
	if device == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device is required"})
		return "", false
	}
	return device, true
}

// GetSorterCameraPreviewStatus GET /sorter/cameras/preview/status?device=: starts (or
// keeps) a preview capture and says how it's doing.
func GetSorterCameraPreviewStatus(c *gin.Context) {
	device, ok := previewDevice(c)
	if !ok {
		return
	}
	_, _, status, err := sorterPerception.Preview(device)
	switch {
	case errors.Is(err, sorter.ErrUnknownDevice):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusOK, gin.H{"camera": status})
	}
}

// ListSorterCameraModes GET /sorter/cameras/modes?device=: the sizes a listed camera
// offers, so the page can warn when the chosen size isn't one of them.
func ListSorterCameraModes(c *gin.Context) {
	device, ok := previewDevice(c)
	if !ok {
		return
	}
	if !sorterPerception.MayOpen(device) {
		c.JSON(http.StatusNotFound, gin.H{"error": sorter.ErrUnknownDevice.Error()})
		return
	}
	modes, err := camera.ListModes(c.Request.Context(), sorterSettings.Machine().Cameras.Ffmpeg, device)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"modes": []camera.Mode{}, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"modes": modes})
}

// StreamSorterCameraPreview GET /sorter/cameras/preview/stream?device=: MJPEG of any
// listed camera, assigned or not, for picking which camera goes where.
func StreamSorterCameraPreview(c *gin.Context) {
	device, ok := previewDevice(c)
	if !ok {
		return
	}
	if _, _, _, err := sorterPerception.Preview(device); errors.Is(err, sorter.ErrUnknownDevice) {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	streamMJPEG(c, func() ([]byte, time.Time, bool) {
		jpg, at, _, err := sorterPerception.Preview(device) // each call keeps it running
		return jpg, at, err == nil
	})
}

// GetSorterCameraFrame GET /sorter/cameras/:channel/frame: the newest JPEG.
func GetSorterCameraFrame(c *gin.Context) {
	ch, ok := cameraChannel(c)
	if !ok {
		return
	}
	jpg, _, ok := sorterFeed.JPEG(ch)
	// ?full=1: the full-size picture of a camera capturing larger than it's tracked
	// at (a channel's own size above the tracking width), when there is one.
	if c.Query("full") == "1" {
		if hr, isHR := sorterFeed.(interface {
			HiRes(sorter.Channel) ([]byte, time.Time, float64, bool)
		}); isHR {
			if full, _, _, fullOK := hr.HiRes(ch); fullOK {
				jpg, ok = full, true
			}
		}
	}
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
	streamMJPEG(c, func() ([]byte, time.Time, bool) { return sorterFeed.JPEG(ch) })
}

// streamMJPEG sends each new frame from next until the viewer goes away.
func streamMJPEG(c *gin.Context, next func() ([]byte, time.Time, bool)) {
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
		jpg, at, ok := next()
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
