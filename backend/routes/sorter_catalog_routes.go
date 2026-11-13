package routes

import (
	"errors"
	"net/http"
	"strconv"

	"goklippy/backend/logic/sorter"

	"github.com/gin-gonic/gin"
)

// catalogOf is the records' catalog, or answers that there isn't one.
func catalogOf(c *gin.Context) (sorter.CatalogStore, bool) {
	cat, err := sorterRecords.Catalog()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return nil, false
	}
	return cat, true
}

// ListSorterCatalog GET /sorter/catalog?q=&color=&category=&run=&sort=seen|name|part|color|last&dir=asc|desc
func ListSorterCatalog(c *gin.Context) {
	cat, ok := catalogOf(c)
	if !ok {
		return
	}
	runID, _ := strconv.ParseInt(c.Query("run"), 10, 64)
	sort := c.DefaultQuery("sort", "seen")
	desc := c.Query("dir") == "desc" || (c.Query("dir") == "" && (sort == "seen" || sort == "last"))
	entries, err := cat.ListCatalog(c.Request.Context(), sorter.CatalogFilter{
		Search: c.Query("q"), ColorID: c.Query("color"), Category: c.Query("category"),
		RunID: runID, Sort: sort, Desc: desc,
	})
	if err != nil {
		sorterError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"parts": entries})
}

// GetSorterCatalogPart GET /sorter/catalog/part?part=3001&color=5: one part and colour
// with every kept photo of it and its pieces.
func GetSorterCatalogPart(c *gin.Context) {
	cat, ok := catalogOf(c)
	if !ok {
		return
	}
	part, err := cat.CatalogPart(c.Request.Context(), c.Query("part"), c.Query("color"))
	if errors.Is(err, sorter.ErrPartNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		sorterError(c, err)
		return
	}
	c.JSON(http.StatusOK, part)
}

// GetSorterCatalogFacets GET /sorter/catalog/facets: the colours, categories and runs
// to filter by, and the totals.
func GetSorterCatalogFacets(c *gin.Context) {
	cat, ok := catalogOf(c)
	if !ok {
		return
	}
	facets, err := cat.CatalogFacets(c.Request.Context())
	if err != nil {
		sorterError(c, err)
		return
	}
	c.JSON(http.StatusOK, facets)
}

// ClearSorterCatalog DELETE /sorter/catalog
func ClearSorterCatalog(c *gin.Context) {
	cat, ok := catalogOf(c)
	if !ok {
		return
	}
	if err := cat.ClearCatalog(c.Request.Context()); err != nil {
		sorterError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"cleared": true})
}

// GetSorterPhotos GET /sorter/photos: how many piece photos are kept, and their size.
func GetSorterPhotos(c *gin.Context) {
	c.JSON(http.StatusOK, sorterRecords.Photos())
}

// ClearSorterPhotos DELETE /sorter/photos: removes every piece photo; the records and
// the catalog stay.
func ClearSorterPhotos(c *gin.Context) {
	freed, err := sorterRecords.ClearPhotos(c.Request.Context())
	if errors.Is(err, sorter.ErrNoPhotoStore) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		sorterError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"freed": freed})
}

// FillSorterCatalog POST /sorter/catalog/fill: counts the identified pieces in the
// records that aren't in the catalog yet.
func FillSorterCatalog(c *gin.Context) {
	cat, ok := catalogOf(c)
	if !ok {
		return
	}
	added, err := cat.FillCatalog(c.Request.Context())
	if err != nil {
		sorterError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"added": added})
}
