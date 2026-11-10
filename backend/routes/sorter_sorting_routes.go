package routes

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"goklippy/backend/logic/brickognize"
	"goklippy/backend/logic/sorter"
	"goklippy/backend/logic/sorter/profile"

	"github.com/gin-gonic/gin"
)

// Hive's compiled profiles carry the whole part map and run to tens of MB.
const maxProfileBytes = 128 << 20

var (
	sorterSorting    *sorter.SortingService
	sorterIdentifier sorter.Identifier
)

// InitSorting wires the sorting profiles, bins and part identification into the
// routes.
func InitSorting(svc *sorter.SortingService, identifier sorter.Identifier) {
	sorterSorting = svc
	sorterIdentifier = identifier
}

const maxIdentifyPhotoBytes = 15 << 20

// IdentifySorterPiece POST /sorter/identify, multipart: up to 8 "image" files (JPEG
// or PNG) of one piece and an optional "piece_mm". Sends them to Brickognize and
// says where the active profile would send the best match. Nothing is saved.
func IdentifySorterPiece(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, brickognize.MaxImages*maxIdentifyPhotoBytes+1<<20)
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "send the photos as multipart form files named image"})
		return
	}
	files := form.File["image"]
	if len(files) == 0 || len(files) > brickognize.MaxImages {
		c.JSON(http.StatusBadRequest, gin.H{"error": "send 1 to 8 photos"})
		return
	}
	images := make([][]byte, 0, len(files))
	for _, fh := range files {
		if fh.Size > maxIdentifyPhotoBytes {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": fh.Filename + " is larger than 15 MB"})
			return
		}
		f, err := fh.Open()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "could not read " + fh.Filename})
			return
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "could not read " + fh.Filename})
			return
		}
		images = append(images, data)
	}
	pieceMm, _ := strconv.ParseFloat(c.PostForm("piece_mm"), 64)

	res, err := sorter.IdentifyAndRoute(c.Request.Context(), sorterIdentifier, sorterSorting, images, pieceMm)
	if errors.Is(err, sorter.ErrBadImage) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

// ListSortingProfiles GET /sorter/profiles
func ListSortingProfiles(c *gin.Context) {
	list, err := sorterSorting.Profiles(c.Request.Context())
	if err != nil {
		sortingError(c, err)
		return
	}
	if list == nil {
		list = []sorter.StoredProfile{}
	}
	c.JSON(http.StatusOK, gin.H{"profiles": list})
}

// ImportSortingProfile POST /sorter/profiles with a sorting_profile.json as the body.
func ImportSortingProfile(c *gin.Context) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxProfileBytes+1))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not read the profile"})
		return
	}
	if len(body) > maxProfileBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "the profile is larger than 128 MB"})
		return
	}
	p, err := sorterSorting.Import(c.Request.Context(), body)
	if err != nil {
		sortingError(c, err)
		return
	}
	c.JSON(http.StatusCreated, p)
}

type activateRequest struct {
	Mode string `json:"mode"`
}

// ActivateSortingProfile POST /sorter/profiles/:id/activate {"mode": "keep|reset|preassign"}
func ActivateSortingProfile(c *gin.Context) {
	id, ok := profileID(c)
	if !ok {
		return
	}
	var req activateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "mode is required"})
		return
	}
	if err := sorterSorting.Activate(c.Request.Context(), id, sorter.Activation(req.Mode)); err != nil {
		sortingError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// DeleteSortingProfile DELETE /sorter/profiles/:id
func DeleteSortingProfile(c *gin.Context) {
	id, ok := profileID(c)
	if !ok {
		return
	}
	if err := sorterSorting.Delete(c.Request.Context(), id); err != nil {
		sortingError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GetSorterBins GET /sorter/bins: the tower with each bin's categories.
func GetSorterBins(c *gin.Context) {
	c.JSON(http.StatusOK, sorterSorting.Bins())
}

type setBinRequest struct {
	Categories []string `json:"categories"`
}

// SetSorterBin PUT /sorter/bins/:layer/:bin {"categories": ["id", ...]}
func SetSorterBin(c *gin.Context) {
	layer, err1 := strconv.Atoi(c.Param("layer"))
	bin, err2 := strconv.Atoi(c.Param("bin"))
	if err1 != nil || err2 != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "layer and bin must be numbers"})
		return
	}
	var req setBinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "categories is required"})
		return
	}
	if err := sorterSorting.SetBin(c.Request.Context(), sorter.BinKey{Layer: layer, Bin: bin}, req.Categories); err != nil {
		sortingError(c, err)
		return
	}
	c.JSON(http.StatusOK, sorterSorting.Bins())
}

// ResetSorterBins POST /sorter/bins/reset unassigns every bin.
func ResetSorterBins(c *gin.Context) {
	if err := sorterSorting.ResetBins(c.Request.Context()); err != nil {
		sortingError(c, err)
		return
	}
	c.JSON(http.StatusOK, sorterSorting.Bins())
}

type routeRequest struct {
	PartID  string  `json:"part_id" binding:"required"`
	ColorID string  `json:"color_id"`
	PieceMm float64 `json:"piece_mm"`
}

// RouteSorterPiece POST /sorter/route {"part_id": "3001", "color_id": "5"}: where a
// piece would go now. Nothing is saved.
func RouteSorterPiece(c *gin.Context) {
	var req routeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "part_id is required"})
		return
	}
	res, err := sorterSorting.Route(c.Request.Context(), req.PartID, req.ColorID, req.PieceMm, false)
	if err != nil {
		sortingError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func profileID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "profile id must be a number"})
		return 0, false
	}
	return id, true
}

func sortingError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, sorter.ErrProfileNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, profile.ErrInvalid), errors.Is(err, sorter.ErrInvalidActivation),
		errors.Is(err, sorter.ErrUnknownCategory):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, sorter.ErrNoActiveProfile), errors.Is(err, sorter.ErrProfileActive):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		sorterError(c, err)
	}
}
