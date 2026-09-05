package routes

import (
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"time"

	"goklippy/backend/klipperconfig"
	"goklippy/backend/mcu"
	"goklippy/backend/mcu/cfgtranslate"
	"goklippy/backend/profiles"

	"github.com/gin-gonic/gin"
)

type mcuConnectRequest struct {
	Port      string `json:"port"`
	Baud      int    `json:"baud"`
	ConfigPath string `json:"config_path"`
	ProfileID string `json:"profile_id"`
	UseWorkspace bool `json:"use_workspace"`
}

type digitalOutRequest struct {
	OID   int  `json:"oid"`
	Value bool `json:"value"`
}

type servoRequest struct {
	OID   int     `json:"oid"`
	Angle float64 `json:"angle"`
}

type stepperJogRequest struct {
	OID      int     `json:"oid"`
	Forward  bool    `json:"forward"`
	SpeedHz  float64 `json:"speed_hz"`
	Steps    int     `json:"steps"`
}

// ConnectMCU POST /mcu/connect
func ConnectMCU(c *gin.Context) {
	start := time.Now()
	requestID := c.GetString("request_id")

	var req mcuConnectRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Port == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "port is required"})
		return
	}

	slog.Info("mcu connect request received",
		"request_id", requestID,
		"port", req.Port,
		"baud", req.Baud,
		"use_workspace", req.UseWorkspace,
		"config_path", req.ConfigPath,
		"profile_id", req.ProfileID,
	)

	if existing, ok := mcu.DefaultManager().GetByPort(req.Port); ok {
		slog.Info("mcu connect reusing existing session",
			"request_id", requestID,
			"port", req.Port,
			"session_id", existing.ID,
			"duration_ms", time.Since(start).Milliseconds(),
		)
		c.JSON(http.StatusOK, sessionResponse(existing))
		return
	}

	f, err := loadConfigForConnect(req)
	if err != nil {
		slog.Warn("mcu connect config load failed", "request_id", requestID, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	profileID := req.ProfileID
	if profileID == "" {
		profileID = "ramps14"
	}
	var profile profiles.Profile
	if boardProfilesDir != "" {
		if loaded, err := profiles.LoadByID(boardProfilesDir, profileID); err == nil {
			profile = loaded
		}
	}

	build, err := cfgtranslate.Build(f)
	if err != nil {
		slog.Warn("mcu connect config build failed", "request_id", requestID, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if profile.ID != "" {
		cfgtranslate.MapObjectsToSlots(build.Objects, profile, f)
	}

	slog.Info("mcu connect config ready",
		"request_id", requestID,
		"port", req.Port,
		"sections", len(f.Sections),
		"objects", len(build.Objects),
		"commands", len(build.Commands),
	)

	baud := req.Baud
	if baud == 0 {
		baud = klipperconfig.MCUBaud(f)
		if baud != 0 {
			slog.Info("mcu connect using baud from config",
				"request_id", requestID,
				"port", req.Port,
				"baud", baud,
			)
		}
	}
	timeout := time.Duration(mcuSettings.ScanTimeoutMs) * time.Millisecond
	if timeout == 0 {
		timeout = mcu.DefaultProbeTimeout
	}

	session, err := mcu.DefaultManager().Connect(req.Port, baud, timeout, build)
	if err != nil {
		slog.Warn("mcu connect failed",
			"request_id", requestID,
			"port", req.Port,
			"baud", baud,
			"objects", len(build.Objects),
			"duration_ms", time.Since(start).Milliseconds(),
			"error", err,
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	slog.Info("mcu connected",
		"request_id", requestID,
		"port", req.Port,
		"baud", session.Baud,
		"session_id", session.ID,
		"objects", len(session.Objects),
		"duration_ms", time.Since(start).Milliseconds(),
	)

	c.JSON(http.StatusOK, sessionResponse(session))
}

func loadConfigForConnect(req mcuConnectRequest) (*klipperconfig.File, error) {
	if req.ConfigPath != "" {
		if klipperConfigService == nil {
			return nil, errConfigService()
		}
		ws, err := klipperConfigService.Load(req.ConfigPath)
		if err != nil {
			return nil, err
		}
		return ws.Merged(), nil
	}
	if req.UseWorkspace || req.ConfigPath == "" {
		if klipperConfigService == nil {
			// Fall back to bundled config with all RAMPS driver slots.
			return klipperconfig.LoadFile(filepath.Join("testdata", "ramps-all-steppers.cfg"))
		}
		ws, err := klipperConfigService.Current()
		if err != nil {
			ws, err = klipperConfigService.Load("ramps-all-steppers.cfg")
			if err != nil {
				ws, err = klipperConfigService.Load("stepper-x-only.cfg")
				if err != nil {
					return nil, err
				}
			}
		}
		return ws.Merged(), nil
	}
	return nil, errConfigService()
}

func errConfigService() error {
	return fmt.Errorf("config service not configured")
}

// DisconnectMCU POST /mcu/disconnect
func DisconnectMCU(c *gin.Context) {
	var req struct {
		SessionID string `json:"session_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.SessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_id is required"})
		return
	}
	if err := mcu.DefaultManager().Disconnect(req.SessionID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GetMCUSession GET /mcu/sessions/:id
func GetMCUSession(c *gin.Context) {
	session, ok := mcu.DefaultManager().Get(c.Param("id"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}
	c.JSON(http.StatusOK, sessionResponse(session))
}

// GetMCUReadings GET /mcu/sessions/:id/readings
func GetMCUReadings(c *gin.Context) {
	session, ok := mcu.DefaultManager().Get(c.Param("id"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}
	readings := session.Poll(200 * time.Millisecond)
	c.JSON(http.StatusOK, gin.H{"readings": readings, "at": time.Now().UTC()})
}

// SetMCUDigitalOut POST /mcu/sessions/:id/digital-out
func SetMCUDigitalOut(c *gin.Context) {
	session, ok := mcu.DefaultManager().Get(c.Param("id"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}
	var req digitalOutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if err := session.SetDigitalOut(req.OID, req.Value); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// SetMCUServo POST /mcu/sessions/:id/servo
func SetMCUServo(c *gin.Context) {
	session, ok := mcu.DefaultManager().Get(c.Param("id"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}
	var req servoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if err := session.SetServoAngle(req.OID, req.Angle); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// JogMCUStepper POST /mcu/sessions/:id/stepper/jog
func JogMCUStepper(c *gin.Context) {
	session, ok := mcu.DefaultManager().Get(c.Param("id"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}
	var req stepperJogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if err := session.StepperJog(req.OID, req.Forward, req.SpeedHz, req.Steps); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	time.Sleep(100 * time.Millisecond)
	pos, _ := session.StepperPosition(req.OID)
	c.JSON(http.StatusOK, gin.H{"ok": true, "position": pos})
}

func sessionResponse(session *mcu.Session) gin.H {
	objects := make([]gin.H, 0, len(session.Objects))
	for _, obj := range session.Objects {
		objects = append(objects, gin.H{
			"section": obj.Section,
			"oid":     obj.OID,
			"kind":    obj.Kind,
			"slot_id": obj.SlotID,
			"label":   obj.Label,
			"meta":    obj.Meta,
		})
	}
	return gin.H{
		"session_id": session.ID,
		"port":       session.Port,
		"baud":       session.Baud,
		"objects":    objects,
		"constants":  session.Constants,
	}
}
