package routes

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"goklippy/backend/logic/sorter"

	"github.com/gin-gonic/gin"
)

var (
	sorterMachine  *sorter.Machine
	sorterSettings *sorter.SettingsService
)

// InitSorter wires the sorter's control loop and settings into the routes.
func InitSorter(machine *sorter.Machine, settings *sorter.SettingsService) {
	sorterMachine = machine
	sorterSettings = settings
}

const maxSorterSettingsBytes = 1 << 20

// GetSorterSettings GET /sorter/settings/:name
// saved is false (and data null) until the page first saves this document.
func GetSorterSettings(c *gin.Context) {
	data, saved, err := sorterSettings.Get(c.Request.Context(), c.Param("name"))
	if err != nil {
		sorterError(c, err)
		return
	}
	if !saved {
		data = json.RawMessage("null")
	}
	c.JSON(http.StatusOK, gin.H{"name": c.Param("name"), "saved": saved, "data": data})
}

// PutSorterSettings PUT /sorter/settings/:name with the document as the body.
func PutSorterSettings(c *gin.Context) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxSorterSettingsBytes+1))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not read body"})
		return
	}
	if len(body) > maxSorterSettingsBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "settings document is too large"})
		return
	}
	if err := sorterSettings.Put(c.Request.Context(), c.Param("name"), body); err != nil {
		sorterError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GetSorterState GET /sorter/state
func GetSorterState(c *gin.Context) {
	c.JSON(http.StatusOK, sorterMachine.Snapshot())
}

type sorterCommandRequest struct {
	Command string `json:"command" binding:"required"`
}

// SorterCommand POST /sorter/commands {"command": "home|start|pause|resume|stop"}
func SorterCommand(c *gin.Context) {
	var req sorterCommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "command is required"})
		return
	}
	snap, err := sorterMachine.Do(c.Request.Context(), sorter.Command(req.Command))
	if err != nil {
		sorterError(c, err)
		return
	}
	c.JSON(http.StatusOK, snap)
}

type sorterMoveRequest struct {
	Degrees float64 `json:"degrees"`
	Rpm     float64 `json:"rpm"`
}

// MoveSorterChannel POST /sorter/channels/:channel/move {"degrees": 72, "rpm": 10}
// Positive degrees turn the channel forward. Ramps use the channel's acceleration.
func MoveSorterChannel(c *gin.Context) {
	ch, err := sorter.ParseChannel(c.Param("channel"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	var req sorterMoveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if err := sorterMachine.MoveChannel(c.Request.Context(), ch, req.Degrees, req.Rpm); err != nil {
		sorterError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// SorterWebSocket GET /sorter/ws streams state snapshots: one on connect, then on
// every change.
func SorterWebSocket(c *gin.Context) {
	conn, err := mcuWSUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	updates, stop := sorterMachine.Subscribe()
	defer stop()

	closed := make(chan struct{})
	go func() {
		defer close(closed)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	type stateMessage struct {
		Type  string          `json:"type"`
		State sorter.Snapshot `json:"state"`
	}
	if err := conn.WriteJSON(stateMessage{Type: "state", State: sorterMachine.Snapshot()}); err != nil {
		return
	}
	for {
		select {
		case <-closed:
			return
		case snap := <-updates:
			if err := conn.WriteJSON(stateMessage{Type: "state", State: snap}); err != nil {
				return
			}
		}
	}
}

func sorterError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, sorter.ErrInvalidSettings), errors.Is(err, sorter.ErrUnknownChannel),
		errors.Is(err, sorter.ErrBinUnreachable), errors.Is(err, sorter.ErrNoSuchBin):
		status = http.StatusBadRequest
	case errors.Is(err, sorter.ErrInvalidTransition), errors.Is(err, sorter.ErrBusy),
		errors.Is(err, sorter.ErrChuteNotHomed), errors.Is(err, sorter.ErrNoActiveProfile),
		errors.Is(err, sorter.ErrNoLight), errors.Is(err, sorter.ErrSupplyLow):
		status = http.StatusConflict
	case errors.Is(err, sorter.ErrNoBoard):
		status = http.StatusServiceUnavailable
	}
	if status == http.StatusInternalServerError {
		slog.Error("sorter request failed", "path", c.FullPath(), "error", err)
	}
	c.JSON(status, gin.H{"error": err.Error()})
}

// ClearSorterChannels POST /sorter/channels/clear: runs C2–C4 for the set time to empty
// them (into the bucket when the chute is in), as a job.
func ClearSorterChannels(c *gin.Context) {
	if err := sorterMachine.ClearChannels(c.Request.Context()); err != nil {
		sorterError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ForgetSorterIgnored POST /sorter/channels/:channel/forget-ignored: the channel's
// camera watches the spots of stuck pieces it was working around again.
func ForgetSorterIgnored(c *gin.Context) {
	ch, err := sorter.ParseChannel(c.Param("channel"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if err := sorterMachine.ForgetIgnored(c.Request.Context(), ch); err != nil {
		sorterError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ListSorterShakes GET /sorter/shakes: the shake profiles, gentlest first.
func ListSorterShakes(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"shakes": sorter.ShakeProfiles})
}

type shakeRequest struct {
	Profile string `json:"profile" binding:"required"`
}

// ShakeSorterChannel POST /sorter/channels/:channel/shake {"profile": "heavy"}
func ShakeSorterChannel(c *gin.Context) {
	ch, err := sorter.ParseChannel(c.Param("channel"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	var req shakeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "profile is required"})
		return
	}
	if err := sorterMachine.ShakeChannel(c.Request.Context(), ch, req.Profile); err != nil {
		if errors.Is(err, sorter.ErrUnknownShake) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		sorterError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
