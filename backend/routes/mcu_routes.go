package routes

import (
	"net/http"
	"time"

	"goklippy/backend/config"
	"goklippy/backend/logic/discovery"

	"github.com/gin-gonic/gin"
)

var mcuSettings config.MCUConfig

// InitMCUConfig stores MCU serial defaults for discovery routes.
func InitMCUConfig(cfg config.MCUConfig) {
	mcuSettings = cfg
}

// GetMCUPorts GET /mcu/ports
func GetMCUPorts(c *gin.Context) {
	if cached, ok := discovery.Cached(); ok {
		c.JSON(http.StatusOK, cached)
		return
	}
	ports, err := discovery.ListPorts()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"scanned_at": nil,
		"ports":      ports,
	})
}

// ScanMCUPorts POST /mcu/scan
func ScanMCUPorts(c *gin.Context) {
	timeout := time.Duration(mcuSettings.ScanTimeoutMs) * time.Millisecond
	if timeout == 0 {
		timeout = 2 * time.Second
	}
	result, err := discovery.Scan(mcuSettings.DefaultBaud, timeout)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}
