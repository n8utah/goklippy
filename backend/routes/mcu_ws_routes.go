package routes

import (
	"net/http"
	"time"

	"goklippy/backend/mcu"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var mcuWSUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type mcuWSMessage struct {
	Type      string                 `json:"type"`
	Readings  map[string]mcu.Reading `json:"readings,omitempty"`
	Session   gin.H                  `json:"session,omitempty"`
	At        time.Time              `json:"at,omitempty"`
}

// ListMCUSessions GET /mcu/sessions
func ListMCUSessions(c *gin.Context) {
	sessions := mcu.DefaultManager().List()
	out := make([]gin.H, 0, len(sessions))
	for _, s := range sessions {
		out = append(out, sessionResponse(s))
	}
	c.JSON(http.StatusOK, gin.H{"sessions": out})
}

// MCUSessionWebSocket GET /mcu/sessions/:id/ws
func MCUSessionWebSocket(c *gin.Context) {
	session, ok := mcu.DefaultManager().Get(c.Param("id"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	conn, err := mcuWSUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	send := func(msg mcuWSMessage) error {
		return conn.WriteJSON(msg)
	}

	if err := send(mcuWSMessage{Type: "session", Session: sessionResponse(session)}); err != nil {
		return
	}

	readings := session.Poll(150 * time.Millisecond)
	if err := send(mcuWSMessage{Type: "readings", Readings: readings, At: time.Now().UTC()}); err != nil {
		return
	}

	updates := session.SubscribeTelemetry()
	defer session.UnsubscribeTelemetry(updates)

	// Read client messages (ping/disconnect) in background.
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	for readings := range updates {
		out := make(map[string]mcu.Reading, len(readings))
		for k, v := range readings {
			out[k] = v
		}
		if err := send(mcuWSMessage{Type: "readings", Readings: out, At: time.Now().UTC()}); err != nil {
			return
		}
	}
}

// StopMCUStepper POST /mcu/sessions/:id/stepper/stop
func StopMCUStepper(c *gin.Context) {
	session, ok := mcu.DefaultManager().Get(c.Param("id"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}
	var req struct {
		OID int `json:"oid"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if err := session.StepperStop(req.OID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	time.Sleep(50 * time.Millisecond)
	pos, _ := session.StepperPosition(req.OID)
	c.JSON(http.StatusOK, gin.H{"ok": true, "position": pos})
}
