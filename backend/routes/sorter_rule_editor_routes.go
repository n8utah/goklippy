package routes

import (
	"errors"
	"net/http"

	"goklippy/backend/logic/sorter"
	"goklippy/backend/logic/sorter/profile"

	"github.com/gin-gonic/gin"
)

// The rule editor (Rules › Profiles): profiles made, copied and edited on the sorter,
// and sets looked up on Rebrickable.

var sorterRebrickable *sorter.Rebrickable

// InitRuleEditor wires the Rebrickable set lookup into the routes.
func InitRuleEditor(rb *sorter.Rebrickable) {
	sorterRebrickable = rb
}

// GetSorterEditorOptions GET /sorter/profile-editor: the part types, sizes and colours
// to pick from, the premade rules and the starter profiles.
func GetSorterEditorOptions(c *gin.Context) {
	opts, err := sorterSorting.EditorOptions(c.Request.Context())
	if err != nil {
		ruleEditorError(c, err)
		return
	}
	c.JSON(http.StatusOK, opts)
}

type newProfileRequest struct {
	Name     string            `json:"name"`
	CopyOf   *int64            `json:"copy_of"`
	Starter  string            `json:"starter"`
	Document *profile.Document `json:"document"`
}

// CreateEditorProfile POST /sorter/profiles/new {"name"} with one of "copy_of": id,
// "starter": key or "document": {...}; just a name makes an empty profile.
func CreateEditorProfile(c *gin.Context) {
	var req newProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	ctx := c.Request.Context()
	var (
		saved sorter.StoredProfile
		err   error
	)
	switch {
	case req.CopyOf != nil:
		saved, err = sorterSorting.CopyProfile(ctx, *req.CopyOf, req.Name)
	case req.Starter != "":
		saved, err = sorterSorting.CreateFromStarter(ctx, req.Starter, req.Name)
	case req.Document != nil:
		saved, err = sorterSorting.CreateEditorProfile(ctx, *req.Document)
	default:
		saved, err = sorterSorting.CreateEditorProfile(ctx, profile.Document{Name: req.Name})
	}
	if err != nil {
		ruleEditorError(c, err)
		return
	}
	c.JSON(http.StatusCreated, saved)
}

// GetEditorProfile GET /sorter/profiles/:id/rules: the profile as editor rules.
func GetEditorProfile(c *gin.Context) {
	id, ok := profileID(c)
	if !ok {
		return
	}
	out, err := sorterSorting.EditableProfile(c.Request.Context(), id)
	if err != nil {
		ruleEditorError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

type saveProfileRequest struct {
	Document profile.Document `json:"document"`
	// Bins: "emptied" or "continue", needed when the collection in the bins sorts by it.
	Bins string `json:"bins"`
}

// SaveEditorProfile PUT /sorter/profiles/:id/rules {"document", "bins"}.
func SaveEditorProfile(c *gin.Context) {
	id, ok := profileID(c)
	if !ok {
		return
	}
	var req saveProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if _, err := sorterMachine.SaveEditorProfile(c.Request.Context(), id, req.Document, req.Bins); err != nil {
		ruleEditorError(c, err)
		return
	}
	out, err := sorterSorting.EditableProfile(c.Request.Context(), id)
	if err != nil {
		ruleEditorError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

type checkPieceRequest struct {
	Document     profile.Document `json:"document"`
	PartID       string           `json:"part_id" binding:"required"`
	ColorID      string           `json:"color_id"`
	PartName     string           `json:"part_name"`
	PartCategory string           `json:"part_category"`
}

// CheckEditorPiece POST /sorter/profiles/check {"document", "part_id", "color_id"}: which
// rule of an unsaved profile takes a part in a colour. Nothing is saved.
func CheckEditorPiece(c *gin.Context) {
	var req checkPieceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "part_id is required"})
		return
	}
	piece := profile.Piece{Part: req.PartID, Color: req.ColorID, PartName: req.PartName, PartCategory: req.PartCategory}
	res, err := sorterSorting.CheckPiece(c.Request.Context(), req.Document, piece)
	if err != nil {
		ruleEditorError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

type setLookupRequest struct {
	Number string `json:"number" binding:"required"`
}

// LookupSorterSet POST /sorter/sets/lookup {"number": "75192-1"}: a set's parts as
// BrickLink ids, without spares or minifigures, ready to add to a set rule.
func LookupSorterSet(c *gin.Context) {
	var req setLookupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "number is required"})
		return
	}
	set, err := sorterRebrickable.LookupSet(c.Request.Context(), req.Number)
	if err != nil {
		ruleEditorError(c, err)
		return
	}
	c.JSON(http.StatusOK, set)
}

// GetSorterRebrickable GET /sorter/rebrickable: {"key_set": true}.
func GetSorterRebrickable(c *gin.Context) {
	set, err := sorterRebrickable.KeySet(c.Request.Context())
	if err != nil {
		ruleEditorError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"key_set": set})
}

type rebrickableRequest struct {
	APIKey string `json:"api_key"`
}

// PutSorterRebrickable PUT /sorter/rebrickable {"api_key": "..."}; empty removes it.
func PutSorterRebrickable(c *gin.Context) {
	var req rebrickableRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if err := sorterRebrickable.SaveKey(c.Request.Context(), req.APIKey); err != nil {
		ruleEditorError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"key_set": req.APIKey != ""})
}

func ruleEditorError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, sorter.ErrProfileNeedsBins):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "needs_bins": true})
	case errors.Is(err, sorter.ErrBuiltinReadOnly):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, sorter.ErrNoSetSource):
		c.JSON(http.StatusPreconditionFailed, gin.H{"error": err.Error(), "needs_source": true})
	case errors.Is(err, sorter.ErrViewerSetsNotReady):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
	case errors.Is(err, sorter.ErrSetNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, sorter.ErrInvalidSettings):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		sortingError(c, err)
	}
}
