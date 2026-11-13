package routes

import (
	"errors"
	"net/http"
	"strconv"

	"goklippy/backend/logic/sorter"

	"github.com/gin-gonic/gin"
)

var sorterCollections *sorter.Collections

// InitCollections wires the collections into the routes.
func InitCollections(collections *sorter.Collections) {
	sorterCollections = collections
}

// ListSorterCollections GET /sorter/collections: every collection with its totals.
func ListSorterCollections(c *gin.Context) {
	cols, err := sorterCollections.List(c.Request.Context())
	if err != nil {
		collectionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"collections": cols})
}

type collectionRequest struct {
	Name      *string `json:"name"`
	Archived  *bool   `json:"archived"`
	ProfileID *int64  `json:"profile_id"`
	// Bins, with profile_id on the current collection: "emptied" (the bins were
	// emptied) or "continue" (keep what's in them; use the unused ones).
	Bins string `json:"bins"`
	// Shared turns sending it to the sort viewer site on or off.
	Shared *bool `json:"shared"`
}

// CreateSorterCollection POST /sorter/collections {"name": "Sam's LEGO", "profile_id": 1}
func CreateSorterCollection(c *gin.Context) {
	var req collectionRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	if req.ProfileID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "profile_id is required: pick the profile it sorts by"})
		return
	}
	col, err := sorterCollections.Create(c.Request.Context(), *req.Name, *req.ProfileID)
	if err != nil {
		collectionError(c, err)
		return
	}
	c.JSON(http.StatusCreated, col)
}

// GetSorterCollection GET /sorter/collections/:id
func GetSorterCollection(c *gin.Context) {
	id, ok := collectionID(c)
	if !ok {
		return
	}
	col, err := sorterCollections.Get(c.Request.Context(), id)
	if err != nil {
		collectionError(c, err)
		return
	}
	c.JSON(http.StatusOK, col)
}

// UpdateSorterCollection PATCH /sorter/collections/:id {"name": "...", "archived": true,
// "profile_id": 2, "bins": "emptied|continue", "shared": true}: renames, archives (or
// unarchives), changes the profile it sorts by, and/or shares it to the sort viewer.
func UpdateSorterCollection(c *gin.Context) {
	id, ok := collectionID(c)
	if !ok {
		return
	}
	var req collectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if req.ProfileID != nil && req.Bins != "" && req.Bins != "emptied" && req.Bins != "continue" {
		c.JSON(http.StatusBadRequest, gin.H{"error": `bins must be "emptied" or "continue"`})
		return
	}
	ctx := c.Request.Context()
	col, err := sorterCollections.Get(ctx, id)
	if err == nil && req.ProfileID != nil {
		if err = sorterMachine.ChangeCollectionProfile(ctx, id, *req.ProfileID, req.Bins == "emptied"); err == nil {
			col, err = sorterCollections.Get(ctx, id)
		}
	}
	if err == nil && req.Name != nil {
		col, err = sorterCollections.Rename(ctx, id, *req.Name)
	}
	if err == nil && req.Archived != nil {
		col, err = sorterCollections.Archive(ctx, id, *req.Archived)
	}
	if err == nil && req.Shared != nil {
		if err = sorterSharing.SetShared(ctx, id, *req.Shared); err == nil {
			col, err = sorterCollections.Get(ctx, id)
		}
	}
	if err != nil {
		collectionError(c, err)
		return
	}
	c.JSON(http.StatusOK, col)
}

// SelectSorterCollection POST /sorter/collections/:id/select: the collection the next
// run sorts into. Picking another than the current one starts the bins afresh.
func SelectSorterCollection(c *gin.Context) {
	id, ok := collectionID(c)
	if !ok {
		return
	}
	col, err := sorterCollections.Get(c.Request.Context(), id)
	if err == nil {
		err = sorterMachine.SelectCollection(c.Request.Context(), col)
	}
	if err != nil {
		collectionError(c, err)
		return
	}
	c.JSON(http.StatusOK, sorterMachine.Snapshot())
}

// GetSorterCollectionPlan GET /sorter/collections/:id/plan: the bins it's being sorted
// into, with what each takes and how many pieces it has had.
func GetSorterCollectionPlan(c *gin.Context) {
	id, ok := collectionID(c)
	if !ok {
		return
	}
	plan, err := sorterCollections.Plan(c.Request.Context(), id)
	if err != nil {
		collectionError(c, err)
		return
	}
	c.JSON(http.StatusOK, plan)
}

// ClearSorterCollectionUnknowns POST /sorter/collections/:id/clear-unknowns: deletes its
// unknown pieces and their photos, before they're fed through again.
func ClearSorterCollectionUnknowns(c *gin.Context) {
	id, ok := collectionID(c)
	if !ok {
		return
	}
	n, err := sorterCollections.ClearUnknowns(c.Request.Context(), id)
	if err != nil {
		collectionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"cleared": n})
}

func collectionID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "collection id must be a number"})
		return 0, false
	}
	return id, true
}

func collectionError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, sorter.ErrCollectionNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, sorter.ErrCollectionName):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, sorter.ErrCollectionArchived), errors.Is(err, sorter.ErrCollectionSorting):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		sortingError(c, err)
	}
}
