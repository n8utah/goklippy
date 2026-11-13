package routes

import (
	"errors"
	"net/http"
	"strconv"

	"goklippy/backend/logic/sorter"

	"github.com/gin-gonic/gin"
)

// GetSorterIncidentCamera GET /sorter/incidents/:id/cameras/:channel serves the picture
// kept of a channel when the incident opened.
func GetSorterIncidentCamera(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "incident id must be a number"})
		return
	}
	path, err := sorterRecords.IncidentImage(c.Request.Context(), id, sorter.Channel(c.Param("channel")))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no picture kept for that incident and channel"})
		return
	}
	c.Header("Cache-Control", "private, max-age=86400")
	c.File(path)
}

// GetSorterIncidentPictures GET /sorter/incidents/pictures: how many incident pictures
// are kept, and their size.
func GetSorterIncidentPictures(c *gin.Context) {
	c.JSON(http.StatusOK, sorterRecords.IncidentPictures())
}

// ClearSorterIncidentPictures DELETE /sorter/incidents/pictures: removes every
// incident's pictures and kept states; the incidents stay.
func ClearSorterIncidentPictures(c *gin.Context) {
	freed, err := sorterRecords.ClearIncidentPictures(c.Request.Context())
	if errors.Is(err, sorter.ErrNoIncidentStore) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		sorterError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"freed": freed})
}

// ClearSorterIncidents DELETE /sorter/incidents: removes every incident with its
// pictures, but the one the machine is holding now.
func ClearSorterIncidents(c *gin.Context) {
	var keep int64
	if sorterMachine != nil {
		if hold := sorterMachine.Snapshot().Incident; hold != nil {
			keep = hold.ID
		}
	}
	n, err := sorterRecords.ClearIncidents(c.Request.Context(), keep)
	if errors.Is(err, sorter.ErrNoIncidentStore) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		sorterError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"removed": n})
}
