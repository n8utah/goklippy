package routes

import (
	"net/http"
	"time"

	"goklippy/backend/logic/sorter"

	"github.com/gin-gonic/gin"
)

// GetSorterChute GET /sorter/chute: the chute now (angle read from the board), its
// geometry, and every bin it aims at.
func GetSorterChute(c *gin.Context) {
	d := sorterSettings.Machine().Distributor
	c.JSON(http.StatusOK, gin.H{
		"status": sorterMachine.ChuteNow(),
		"geometry": gin.H{
			"sections":             d.Sections,
			"section_pitch":        d.SectionPitch(),
			"section_width":        d.SectionWidth,
			"first_section_offset": d.FirstOffset,
			"max_angle":            d.MaxAngle,
		},
		"layers": d.LayerBins(),
	})
}

type chuteAimRequest struct {
	Angle *float64 `json:"angle" binding:"required"`
}

// AimSorterChute POST /sorter/chute/aim {"angle": 42.5}
func AimSorterChute(c *gin.Context) {
	var req chuteAimRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "angle is required"})
		return
	}
	if err := sorterMachine.AimChute(c.Request.Context(), *req.Angle); err != nil {
		sorterError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ZeroSorterChute POST /sorter/chute/zero makes the current position 0° without the
// home switch.
func ZeroSorterChute(c *gin.Context) {
	if err := sorterMachine.ZeroChute(c.Request.Context()); err != nil {
		sorterError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type chuteCalibrateRequest struct {
	FirstAngle     float64 `json:"first_angle"`
	LastAngle      float64 `json:"last_angle"`
	BinsPerSection int     `json:"bins_per_section"`
}

// CalibrateSorterChute POST /sorter/chute/calibrate works out the section width and
// offset from two captured bins. It only calculates: the page saves the result into
// the machine settings, so the page stays the one writer of that document.
func CalibrateSorterChute(c *gin.Context) {
	var req chuteCalibrateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	cal, err := sorter.CalibrateChute(req.FirstAngle, req.LastAngle, req.BinsPerSection, sorterSettings.Machine().Distributor.Sections)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, cal)
}

// SendToSorterBin POST /sorter/distribution/send {"layer": 1, "bin": 4}
func SendToSorterBin(c *gin.Context) {
	var req sorter.BinAddress
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "layer and bin are required"})
		return
	}
	if err := sorterMachine.SendToBin(c.Request.Context(), req); err != nil {
		sorterError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type testLayerRequest struct {
	Layer   int `json:"layer"`
	DwellMs int `json:"dwell_ms"`
}

// TestSorterLayer POST /sorter/distribution/test {"layer": 1, "dwell_ms": 1000}
func TestSorterLayer(c *gin.Context) {
	var req testLayerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if err := sorterMachine.TestLayer(c.Request.Context(), req.Layer, time.Duration(req.DwellMs)*time.Millisecond); err != nil {
		sorterError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// CancelSorterJob POST /sorter/jobs/cancel
func CancelSorterJob(c *gin.Context) {
	if err := sorterMachine.CancelJob(c.Request.Context()); err != nil {
		sorterError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
