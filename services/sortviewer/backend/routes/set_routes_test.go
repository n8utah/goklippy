package routes

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"goklippy/services/sortviewer/backend/logic"
	"goklippy/services/sortviewer/backend/models"
)

// One set, 100-1, of two red 2x4 bricks and a white plate.
var tinyRebrickable = map[string]string{
	"colors":             "id,name,rgb,is_trans\n4,Red,C91A09,False\n15,White,FFFFFF,False",
	"themes":             "id,name,parent_id\n1,City,",
	"parts":              "part_num,name,part_cat_id\n3001,Brick 2 x 4,11\n3023,Plate 1 x 2,14",
	"part_relationships": "rel_type,child_part_num,parent_part_num",
	"sets":               "set_num,name,year,theme_id,num_parts,img_url\n100-1,Red House,2020,1,3,",
	"inventories":        "id,version,set_num\n1,1,100-1",
	"inventory_parts":    "inventory_id,part_num,color_id,quantity,is_spare,img_url\n1,3001,4,2,False,\n1,3023,15,1,False,",
}

func (s *testSite) useSets() {
	s.svc.UseSets(logic.SetsConfig{
		Path:       filepath.Join(s.t.TempDir(), "sets.db"),
		Open:       func(path string) (logic.SetCatalog, error) { return models.OpenCatalog(path) },
		NewBuilder: func(path string) (logic.CatalogBuilder, error) { return models.NewCatalogBuilder(path) },
		Source: func(_ context.Context, name string) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(tinyRebrickable[name])), nil
		},
	})
	s.t.Cleanup(s.svc.CloseSets)
}

func TestSetsBeforeTheDataIsBuilt(t *testing.T) {
	s := newSite(t)
	s.pushSort()
	admin := s.browser()
	code, _ := admin.do("GET", "/auth/login", nil)
	require.Equal(t, http.StatusFound, code)

	code, body := admin.do("GET", "/api/collections/7/sets", nil)
	assert.Equal(t, http.StatusServiceUnavailable, code)
	assert.Contains(t, body["error"], "isn't ready")
	_, body = admin.do("GET", "/api/admin/sets", nil)
	assert.Equal(t, false, body["ready"])
}

func TestSetsForACollection(t *testing.T) {
	s := newSite(t)
	s.useSets()
	require.NoError(t, s.svc.BuildSets(context.Background()))
	s.pushSort() // one red 3001
	admin := s.browser()
	code, _ := admin.do("GET", "/auth/login", nil)
	require.Equal(t, http.StatusFound, code)

	code, body := admin.do("GET", "/api/collections/7/sets?min=1", nil)
	require.Equal(t, 200, code)
	assert.EqualValues(t, 1, body["total"])
	assert.NotNil(t, body["built_at"])
	sets := body["sets"].([]any)
	require.Len(t, sets, 1)
	set := sets[0].(map[string]any)
	assert.Equal(t, "100-1", set["set_num"])
	assert.EqualValues(t, 1, set["have"])
	assert.EqualValues(t, 3, set["need"])

	code, body = admin.do("GET", "/api/collections/7/sets/100-1", nil)
	require.Equal(t, 200, code)
	assert.Len(t, body["parts"], 2)
	code, _ = admin.do("GET", "/api/collections/7/sets/nope-1", nil)
	assert.Equal(t, 404, code)

	// Someone not signed in sees nothing.
	code, _ = s.browser().do("GET", "/api/collections/7/sets", nil)
	assert.Equal(t, http.StatusUnauthorized, code)

	code, body = admin.do("GET", "/api/admin/sets", nil)
	require.Equal(t, 200, code)
	assert.Equal(t, true, body["ready"])
	assert.EqualValues(t, 1, body["sets"])
	code, _ = admin.do("POST", "/api/admin/sets/refresh", nil)
	assert.Equal(t, http.StatusAccepted, code)
	require.Eventually(t, func() bool { return !s.svc.SetsStatus(context.Background()).Building }, 10*time.Second, 20*time.Millisecond)
	_, body = admin.do("GET", "/api/collections/7/sets", nil)
	assert.EqualValues(t, 1, body["total"], "still there after the rebuild")
}
