package routes

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"goklippy/backend/profiles"

	"github.com/gin-gonic/gin"
)

type updateHotspotsRequest struct {
	Hotspots []profiles.Hotspot `json:"hotspots"`
}

// GetBoardProfileEditor GET /board-profiles/:id/editor
func GetBoardProfileEditor(c *gin.Context) {
	id := c.Param("id")
	p, err := profiles.LoadByID(boardProfilesDir, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	usedSlots := map[string]struct{}{}
	for _, h := range p.Diagram.Hotspots {
		for _, slotID := range h.SlotIDs {
			usedSlots[slotID] = struct{}{}
		}
	}

	available := make([]gin.H, 0)
	for slotID, slot := range p.Slots {
		if _, ok := usedSlots[slotID]; ok {
			continue
		}
		available = append(available, gin.H{
			"id":    slotID,
			"label": slot.Label,
			"type":  slot.Type,
			"group": slot.Group,
		})
	}

	diagram := buildDiagramResponse(id, p)
	c.JSON(http.StatusOK, gin.H{
		"profile_id":      id,
		"profile_name":    p.Name,
		"diagram":         diagram,
		"hotspots":        p.Diagram.Hotspots,
		"slots":           p.Slots,
		"available_slots": available,
	})
}

// UpdateBoardProfileHotspots PUT /board-profiles/:id/hotspots
func UpdateBoardProfileHotspots(c *gin.Context) {
	id := c.Param("id")
	if _, err := profiles.LoadByID(boardProfilesDir, id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	var req updateHotspotsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	for i, h := range req.Hotspots {
		if h.ID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "hotspot id is required"})
			return
		}
		if len(h.SlotIDs) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "hotspot slot_ids is required"})
			return
		}
		if h.Shape == "" {
			req.Hotspots[i].Shape = "rect"
		}
	}

	if err := profiles.SaveHotspots(boardProfilesDir, id, req.Hotspots); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	p, _ := profiles.LoadByID(boardProfilesDir, id)
	c.JSON(http.StatusOK, gin.H{
		"hotspots": p.Diagram.Hotspots,
		"message":  "board profile hotspots saved",
	})
}

func buildDiagramResponse(id string, p profiles.Profile) gin.H {
	imageType := p.Diagram.ImageType
	if imageType == "" {
		ext := strings.ToLower(filepath.Ext(p.Diagram.Image))
		if ext == ".svg" {
			imageType = "svg"
		} else {
			imageType = "raster"
		}
	}

	viewBox := p.Diagram.ViewBox
	if viewBox == "" && p.Diagram.Width > 0 && p.Diagram.Height > 0 {
		viewBox = fmt.Sprintf("0 0 %.0f %.0f", p.Diagram.Width, p.Diagram.Height)
	}

	resp := gin.H{
		"image_type": imageType,
		"viewBox":    viewBox,
		"width":      p.Diagram.Width,
		"height":     p.Diagram.Height,
	}

	imagePath := filepath.Join(boardProfilesDir, filepath.FromSlash(p.Diagram.Image))
	if _, err := os.Stat(imagePath); err == nil {
		resp["image_url"] = "/api/v1/board-profiles/" + id + "/image"
	} else if p.Diagram.ImageURL != "" {
		resp["image_url"] = p.Diagram.ImageURL
	}

	return resp
}
