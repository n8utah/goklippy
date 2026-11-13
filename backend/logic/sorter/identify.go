package sorter

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png" // photos may arrive as PNG
	"net/http"

	"goklippy/backend/logic/brickognize"
	"goklippy/backend/logic/sorter/profile"
)

var ErrBadImage = errors.New("not a JPEG or PNG image")

// Identifier names a part from photos; brickognize.Client is one.
type Identifier interface {
	Identify(ctx context.Context, jpegs [][]byte) (brickognize.Result, error)
}

// IdentifyResult is what Brickognize saw and where the active profile would send it.
type IdentifyResult struct {
	Brickognize brickognize.Result `json:"brickognize"`
	Route       *RouteResult       `json:"route,omitempty"`
	RouteError  string             `json:"route_error,omitempty"`
}

// IdentifyAndRoute identifies a piece from photos and routes its best match as a
// dry run: nothing is saved. A missing profile is reported, not an error, so the
// identification still shows.
func IdentifyAndRoute(ctx context.Context, id Identifier, sorting *SortingService, images [][]byte, pieceMm float64) (IdentifyResult, error) {
	jpegs := make([][]byte, 0, len(images))
	for i, img := range images {
		j, err := asJPEG(img)
		if err != nil {
			return IdentifyResult{}, fmt.Errorf("photo %d: %w", i+1, err)
		}
		jpegs = append(jpegs, j)
	}
	res, err := id.Identify(ctx, jpegs)
	if err != nil {
		return IdentifyResult{}, err
	}
	out := IdentifyResult{Brickognize: res}
	item, color := res.Best()
	if item == nil {
		out.RouteError = "Brickognize found no part in the photos."
		return out, nil
	}
	piece := profile.Piece{Part: item.ID, PartCategory: item.Category}
	if color != nil {
		piece.Color, piece.ColorName = color.ID, color.Name
	}
	route, err := sorting.Route(ctx, piece, pieceMm, false)
	if err != nil {
		out.RouteError = err.Error()
		return out, nil
	}
	out.Route = &route
	return out, nil
}

// asJPEG passes JPEGs through and re-encodes PNGs.
func asJPEG(img []byte) ([]byte, error) {
	switch http.DetectContentType(img) {
	case "image/jpeg":
		return img, nil
	case "image/png":
		decoded, _, err := image.Decode(bytes.NewReader(img))
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrBadImage, err)
		}
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, decoded, &jpeg.Options{Quality: 95}); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	default:
		return nil, ErrBadImage
	}
}
