// Package brickognize identifies LEGO parts from photos with the Brickognize API
// (https://api.brickognize.com), as Sorter V2 does (classification/brickognize.py).
package brickognize

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"sort"
	"strings"
	"time"
)

const (
	// DefaultURL asks for the part and its color.
	DefaultURL = "https://api.brickognize.com/predict/?predict_color=true"
	// MaxImages is the most photos Brickognize takes in one request.
	MaxImages = 8
	// Brickognize runs each view before combining them, so a request gets a base
	// time plus a little per extra image. Generous, so slow internet is slow rather
	// than a failed identification.
	baseTimeout     = 60 * time.Second
	perImageTimeout = 2 * time.Second
)

// Duplo and Primo aren't sorted; their matches are dropped.
var filteredCategories = []string{"primo", "duplo"}

var ErrNoImages = errors.New("at least one image is needed")

// Item is one candidate part.
type Item struct {
	ID            string         `json:"id"` // BrickLink part number
	Name          string         `json:"name"`
	ImgURL        string         `json:"img_url"`
	Category      string         `json:"category"`
	Type          string         `json:"type"`
	Score         float64        `json:"score"`
	ExternalSites []ExternalSite `json:"external_sites"`
	// Rank is the item's place in Brickognize's unfiltered answer, which is what its
	// feedback API refers to.
	Rank int `json:"rank"`
}

type ExternalSite struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Color is one candidate color.
type Color struct {
	ID    string  `json:"id"` // BrickLink color id
	Name  string  `json:"name"`
	Score float64 `json:"score"`
	Rank  int     `json:"rank"`
}

type BoundingBox struct {
	Left        float64 `json:"left"`
	Upper       float64 `json:"upper"`
	Right       float64 `json:"right"`
	Lower       float64 `json:"lower"`
	ImageWidth  float64 `json:"image_width"`
	ImageHeight float64 `json:"image_height"`
	Score       float64 `json:"score"`
}

// Result is Brickognize's answer: items best first (Duplo and Primo removed) and
// colors best first.
type Result struct {
	ListingID   string       `json:"listing_id"`
	BoundingBox *BoundingBox `json:"bounding_box,omitempty"`
	Items       []Item       `json:"items"`
	Colors      []Color      `json:"colors"`
}

// Best is the top item and color, if any.
func (r Result) Best() (*Item, *Color) {
	var item *Item
	var color *Color
	if len(r.Items) > 0 {
		item = &r.Items[0]
	}
	if len(r.Colors) > 0 {
		color = &r.Colors[0]
	}
	return item, color
}

// Client calls the API.
type Client struct {
	URL  string
	HTTP *http.Client
}

func NewClient() *Client {
	return &Client{URL: DefaultURL, HTTP: &http.Client{}}
}

// Identify sends up to MaxImages JPEG photos of one piece. Extra images are dropped
// rather than failing the request.
func (c *Client) Identify(ctx context.Context, jpegs [][]byte) (Result, error) {
	if len(jpegs) == 0 {
		return Result{}, ErrNoImages
	}
	if len(jpegs) > MaxImages {
		jpegs = jpegs[:MaxImages]
	}
	body, contentType, err := multipartImages(jpegs)
	if err != nil {
		return Result{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, baseTimeout+time.Duration(len(jpegs)-1)*perImageTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, body)
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("brickognize: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return Result{}, fmt.Errorf("brickognize: read the answer: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("brickognize: %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	return parseResult(raw)
}

func multipartImages(jpegs [][]byte) (*bytes.Buffer, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for i, img := range jpegs {
		h := textproto.MIMEHeader{}
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="query_image"; filename="image_%d.jpg"`, i))
		h.Set("Content-Type", "image/jpeg")
		part, err := w.CreatePart(h)
		if err != nil {
			return nil, "", err
		}
		if _, err := part.Write(img); err != nil {
			return nil, "", err
		}
	}
	if err := w.Close(); err != nil {
		return nil, "", err
	}
	return &buf, w.FormDataContentType(), nil
}

// parseResult stamps each item and color with its place in the answer, drops Duplo
// and Primo items, and orders colors best first.
func parseResult(raw []byte) (Result, error) {
	var res Result
	if err := json.Unmarshal(raw, &res); err != nil {
		return Result{}, fmt.Errorf("brickognize: unreadable answer: %w", err)
	}
	items := make([]Item, 0, len(res.Items))
	for rank, item := range res.Items {
		item.Rank = rank
		if !filtered(item.Category) {
			items = append(items, item)
		}
	}
	res.Items = items
	for rank := range res.Colors {
		res.Colors[rank].Rank = rank
	}
	sort.SliceStable(res.Colors, func(i, j int) bool { return res.Colors[i].Score > res.Colors[j].Score })
	if res.Colors == nil {
		res.Colors = []Color{}
	}
	return res, nil
}

func filtered(category string) bool {
	lower := strings.ToLower(category)
	for _, f := range filteredCategories {
		if strings.Contains(lower, f) {
			return true
		}
	}
	return false
}
