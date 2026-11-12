package routes

import (
	"errors"
	"net/http"
	"os"
	"strconv"

	"goklippy/backend/logic/sorter"

	"github.com/gin-gonic/gin"
)

var journeyRecorder *sorter.JourneyRecorder

// InitJourneys wires the journey recorder in.
func InitJourneys(r *sorter.JourneyRecorder) {
	journeyRecorder = r
}

// ListSorterJourneys GET /sorter/journeys?limit=100: the newest pieces' journeys, those
// still riding included.
func ListSorterJourneys(c *gin.Context) {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "100"))
	if err != nil || limit < 1 || limit > 1000 {
		limit = 100
	}
	c.JSON(http.StatusOK, gin.H{"journeys": journeyRecorder.List(limit)})
}

// GetSorterJourney GET /sorter/journeys/:id: one journey with every frame's decision.
func GetSorterJourney(c *gin.Context) {
	raw, err := journeyRecorder.Get(c.Param("id"))
	if errors.Is(err, sorter.ErrNoJourney) {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		sorterError(c, err)
		return
	}
	c.Data(http.StatusOK, "application/json", raw)
}

// GetSorterPieceJourney GET /sorter/pieces/:id/journey: the journey that became this
// piece (its pictures from C2 and C3), or 404 when it isn't known.
func GetSorterPieceJourney(c *gin.Context) {
	raw, err := journeyRecorder.ForPiece(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no journey is known for this piece"})
		return
	}
	c.Data(http.StatusOK, "application/json", raw)
}

// GetSorterJourneyFile GET /sorter/journeys/:id/files/:name: one of its pictures.
func GetSorterJourneyFile(c *gin.Context) {
	path, err := journeyRecorder.File(c.Param("id"), c.Param("name"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if _, err := os.Stat(path); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no such picture"})
		return
	}
	c.Header("Cache-Control", "private, max-age=86400") // a journey's pictures never change
	c.File(path)
}
