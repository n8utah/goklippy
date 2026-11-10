package routes

import (
	"errors"
	"net/http"
	"strconv"

	"goklippy/backend/logic/sorter"

	"github.com/gin-gonic/gin"
)

var sorterRecords *sorter.Records

// InitRecords wires the piece, run and incident records into the routes.
func InitRecords(records *sorter.Records) {
	sorterRecords = records
}

// ListSorterPieces GET /sorter/pieces?run=&status=&q=&limit=&offset=
func ListSorterPieces(c *gin.Context) {
	runID, _ := strconv.ParseInt(c.Query("run"), 10, 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.Query("offset"))
	pieces, total, err := sorterRecords.Store().ListPieces(c.Request.Context(), sorter.PieceFilter{
		RunID: runID, Status: c.Query("status"), Search: c.Query("q"), Limit: limit, Offset: offset,
	})
	if err != nil {
		sorterError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"pieces": pieces, "total": total})
}

// GetSorterPiece GET /sorter/pieces/:id
func GetSorterPiece(c *gin.Context) {
	p, err := sorterRecords.Store().GetPiece(c.Request.Context(), c.Param("id"))
	if errors.Is(err, sorter.ErrPieceNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		sorterError(c, err)
		return
	}
	c.JSON(http.StatusOK, p)
}

// GetSorterPieceImage GET /sorter/pieces/:id/images/:idx serves one photo.
func GetSorterPieceImage(c *gin.Context) {
	idx, err := strconv.Atoi(c.Param("idx"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "image index must be a number"})
		return
	}
	p, err := sorterRecords.Store().GetPiece(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "piece not found"})
		return
	}
	for _, img := range p.Images {
		if img.Index != idx {
			continue
		}
		path, err := sorterRecords.ImagePath(img.Path)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "image not found"})
			return
		}
		c.Header("Cache-Control", "private, max-age=86400")
		c.File(path)
		return
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "image not found"})
}

// ListSorterRuns GET /sorter/runs
func ListSorterRuns(c *gin.Context) {
	runs, err := sorterRecords.Store().ListRuns(c.Request.Context(), 50)
	if err != nil {
		sorterError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"runs": runs})
}

// ListSorterIncidents GET /sorter/incidents
func ListSorterIncidents(c *gin.Context) {
	incidents, err := sorterRecords.Store().ListIncidents(c.Request.Context(), 100)
	if err != nil {
		sorterError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"incidents": incidents})
}

// GetSorterStats GET /sorter/stats: lifetime totals.
func GetSorterStats(c *gin.Context) {
	stats, err := sorterRecords.Store().Stats(c.Request.Context())
	if err != nil {
		sorterError(c, err)
		return
	}
	c.JSON(http.StatusOK, stats)
}

type resolveIncidentRequest struct {
	Action string `json:"action" binding:"required"`
}

// ResolveSorterIncident POST /sorter/incident/resolve {"action": "done"}
func ResolveSorterIncident(c *gin.Context) {
	var req resolveIncidentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "action is required"})
		return
	}
	err := sorterMachine.ResolveIncident(c.Request.Context(), req.Action)
	if errors.Is(err, sorter.ErrNoIncident) {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		sorterError(c, err)
		return
	}
	c.JSON(http.StatusOK, sorterMachine.Snapshot())
}
