package routes

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goklippy/backend/profiles"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestListBoardProfiles(t *testing.T) {
	gin.SetMode(gin.TestMode)
	InitBoardProfilesDir(filepath.Join("..", "profiles"))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/board-profiles", nil)

	ListBoardProfiles(c)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "ramps14")
}

func TestGetBoardProfile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	InitBoardProfilesDir(filepath.Join("..", "profiles"))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/board-profiles/ramps14", nil)
	c.Params = gin.Params{{Key: "id", Value: "ramps14"}}

	GetBoardProfile(c)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "RAMPS 1.4")
}

func TestGetBoardProfileDiagram(t *testing.T) {
	gin.SetMode(gin.TestMode)
	InitBoardProfilesDir(filepath.Join("..", "profiles"))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/board-profiles/ramps14/diagram", nil)
	c.Params = gin.Params{{Key: "id", Value: "ramps14"}}

	GetBoardProfileDiagram(c)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"image_type":"raster"`)
	require.Contains(t, w.Body.String(), `/api/v1/board-profiles/ramps14/image`)
}

func TestServeBoardProfileImage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	InitBoardProfilesDir(filepath.Join("..", "profiles"))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/board-profiles/ramps14/image", nil)
	c.Params = gin.Params{{Key: "id", Value: "ramps14"}}

	ServeBoardProfileImage(c)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "image/png", w.Header().Get("Content-Type"))
}

func TestGetBoardProfileEditor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	InitBoardProfilesDir(filepath.Join("..", "profiles"))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/board-profiles/ramps14/editor", nil)
	c.Params = gin.Params{{Key: "id", Value: "ramps14"}}

	GetBoardProfileEditor(c)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"available_slots"`)
	require.Contains(t, w.Body.String(), `"hotspots"`)
}

func TestUpdateBoardProfileHotspots(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	src := filepath.Join("..", "profiles", "ramps14.yaml")
	data, err := os.ReadFile(src)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ramps14.yaml"), data, 0o644))
	InitBoardProfilesDir(dir)

	body := `{"hotspots":[{"id":"driver_x","slot_ids":["stepper_x"],"label":"X Stepper","shape":"rect","x":100,"y":120,"width":54,"height":20,"accepts":["stepper"]}]}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/board-profiles/ramps14/hotspots", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "ramps14"}}

	UpdateBoardProfileHotspots(c)
	require.Equal(t, http.StatusOK, w.Code)

	loaded, err := profiles.LoadByID(dir, "ramps14")
	require.NoError(t, err)
	require.Len(t, loaded.Diagram.Hotspots, 1)
	require.Equal(t, float64(100), loaded.Diagram.Hotspots[0].X)
}
