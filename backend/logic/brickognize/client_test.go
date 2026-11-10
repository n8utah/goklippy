package brickognize

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const answer = `{
  "listing_id": "abc",
  "bounding_box": {"left": 1, "upper": 2, "right": 3, "lower": 4, "image_width": 10, "image_height": 10, "score": 0.9},
  "items": [
    {"id": "3001-duplo", "name": "Duplo Brick 2 x 4", "category": "Duplo, Brick", "score": 0.95},
    {"id": "3001", "name": "Brick 2 x 4", "category": "Brick", "score": 0.91, "img_url": "https://img/3001.png"},
    {"id": "3003", "name": "Brick 2 x 2", "category": "Brick", "score": 0.40}
  ],
  "colors": [
    {"id": "1", "name": "White", "score": 0.10},
    {"id": "5", "name": "Red", "score": 0.85}
  ]
}`

func TestIdentifySendsImagesAndReadsTheAnswer(t *testing.T) {
	var images []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseMultipartForm(1<<20))
		for _, fh := range r.MultipartForm.File["query_image"] {
			f, err := fh.Open()
			require.NoError(t, err)
			b, _ := io.ReadAll(f)
			images = append(images, string(b))
			assert.Equal(t, "image/jpeg", fh.Header.Get("Content-Type"))
		}
		_, _ = w.Write([]byte(answer))
	}))
	defer srv.Close()

	c := &Client{URL: srv.URL, HTTP: srv.Client()}
	res, err := c.Identify(context.Background(), [][]byte{[]byte("one"), []byte("two")})
	require.NoError(t, err)
	assert.Equal(t, []string{"one", "two"}, images)

	require.Len(t, res.Items, 2, "Duplo dropped")
	assert.Equal(t, "3001", res.Items[0].ID)
	assert.Equal(t, 1, res.Items[0].Rank, "rank in the unfiltered answer")
	assert.Equal(t, 2, res.Items[1].Rank)

	require.Len(t, res.Colors, 2)
	assert.Equal(t, "Red", res.Colors[0].Name, "best color first")
	assert.Equal(t, 1, res.Colors[0].Rank)

	item, color := res.Best()
	assert.Equal(t, "3001", item.ID)
	assert.Equal(t, "5", color.ID)
	assert.Equal(t, "abc", res.ListingID)
}

func TestIdentifyCapsImages(t *testing.T) {
	var count int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseMultipartForm(1<<20))
		count = len(r.MultipartForm.File["query_image"])
		_, _ = w.Write([]byte(`{"items": [], "colors": []}`))
	}))
	defer srv.Close()

	imgs := make([][]byte, 11)
	for i := range imgs {
		imgs[i] = []byte("x")
	}
	res, err := (&Client{URL: srv.URL, HTTP: srv.Client()}).Identify(context.Background(), imgs)
	require.NoError(t, err)
	assert.Equal(t, MaxImages, count)
	item, color := res.Best()
	assert.Nil(t, item)
	assert.Nil(t, color)
}

func TestIdentifyErrors(t *testing.T) {
	_, err := NewClient().Identify(context.Background(), nil)
	assert.ErrorIs(t, err, ErrNoImages)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "too many images", http.StatusUnprocessableEntity)
	}))
	defer srv.Close()
	_, err = (&Client{URL: srv.URL, HTTP: srv.Client()}).Identify(context.Background(), [][]byte{[]byte("x")})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too many images")
}
