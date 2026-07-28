package routes

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"goklippy/backend/logic/klipperconfig"
	"goklippy/backend/profiles"

	"github.com/gin-gonic/gin"
)

var klipperConfigService *klipperconfig.Service
var boardProfilesDir string

// InitBoardProfilesDir sets the directory containing board profile YAML files.
func InitBoardProfilesDir(dir string) {
	boardProfilesDir = dir
}

// InitKlipperConfigService sets the shared config workspace service.
func InitKlipperConfigService(svc *klipperconfig.Service) {
	klipperConfigService = svc
}

type configLoadRequest struct {
	Path string `json:"path"`
}

type sectionUpdateRequest struct {
	Options map[string]string `json:"options"`
}

type rawConfigRequest struct {
	Content string `json:"content"`
}

// ListKlipperConfigFiles GET /klipper-config/files
func ListKlipperConfigFiles(c *gin.Context) {
	if klipperConfigService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "config service not configured"})
		return
	}
	files, err := klipperConfigService.ListFiles()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"files": files})
}

// LoadKlipperConfig POST /klipper-config/load
func LoadKlipperConfig(c *gin.Context) {
	if klipperConfigService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "config service not configured"})
		return
	}
	var req configLoadRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path is required"})
		return
	}
	ws, err := klipperConfigService.Load(req.Path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, workspaceResponse(ws))
}

// GetKlipperConfigWorkspace GET /klipper-config/workspace
func GetKlipperConfigWorkspace(c *gin.Context) {
	if klipperConfigService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "config service not configured"})
		return
	}
	ws, err := klipperConfigService.Current()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, workspaceResponse(ws))
}

// UpdateKlipperConfigSection PUT /klipper-config/sections/:name
func UpdateKlipperConfigSection(c *gin.Context) {
	ws, err := klipperConfigService.Current()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	var req sectionUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	ws.UpdateSection(c.Param("name"), req.Options)
	c.JSON(http.StatusOK, workspaceResponse(ws))
}

// DeleteKlipperConfigSection DELETE /klipper-config/sections/:name
func DeleteKlipperConfigSection(c *gin.Context) {
	ws, err := klipperConfigService.Current()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	ws.DeleteSection(c.Param("name"))
	ws2, _ := klipperConfigService.Current()
	c.JSON(http.StatusOK, workspaceResponse(ws2))
}

// UpdateKlipperConfigRaw PUT /klipper-config/raw
func UpdateKlipperConfigRaw(c *gin.Context) {
	ws, err := klipperConfigService.Current()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	var req rawConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if err := ws.SetRaw(req.Content); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, workspaceResponse(ws))
}

// SaveKlipperConfig POST /klipper-config/save
func SaveKlipperConfig(c *gin.Context) {
	if err := klipperConfigService.Save(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ws, _ := klipperConfigService.Current()
	c.JSON(http.StatusOK, workspaceResponse(ws))
}

func workspaceResponse(ws *klipperconfig.Workspace) gin.H {
	f := ws.File()
	merged := ws.Merged()
	sections := make([]gin.H, 0, len(f.Order))
	for _, name := range f.Order {
		sec := f.Sections[name]
		sections = append(sections, gin.H{
			"name":    name,
			"options": sec.Options,
		})
	}
	mergedSections := make([]gin.H, 0, len(merged.Order))
	for _, name := range merged.Order {
		sec := merged.Sections[name]
		mergedSections = append(mergedSections, gin.H{
			"name":    name,
			"options": sec.Options,
		})
	}
	return gin.H{
		"path":             ws.Path(),
		"dirty":            ws.Dirty(),
		"sections":         sections,
		"merged_sections":  mergedSections,
		"raw":              f.Serialize(),
		"merged_section":   merged.SectionNames(),
	}
}

// ListBoardProfiles GET /board-profiles
func ListBoardProfiles(c *gin.Context) {
	if boardProfilesDir == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "profiles not configured"})
		return
	}
	ids, err := profiles.List(boardProfilesDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"profiles": ids})
}

// GetBoardProfile GET /board-profiles/:id
func GetBoardProfile(c *gin.Context) {
	id := c.Param("id")
	p, err := profiles.LoadByID(boardProfilesDir, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, p)
}

// GetBoardProfileDiagram GET /board-profiles/:id/diagram
func GetBoardProfileDiagram(c *gin.Context) {
	id := c.Param("id")
	p, err := profiles.LoadByID(boardProfilesDir, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

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
		"hotspots":   p.Diagram.Hotspots,
		"slots":      p.Slots,
	}

	imagePath := filepath.Join(boardProfilesDir, filepath.FromSlash(p.Diagram.Image))
	if _, err := os.Stat(imagePath); err == nil {
		resp["image_url"] = "/api/v1/board-profiles/" + id + "/image"
	} else if p.Diagram.ImageURL != "" {
		resp["image_url"] = p.Diagram.ImageURL
	} else {
		resp["image_url"] = nil
		resp["note"] = "board image pending"
	}

	c.JSON(http.StatusOK, resp)
}
