package sorter

import (
	"bytes"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"

	"goklippy/backend/logic/vision"
)

// cropRect is the box grown by marginPct of its size on each side, inside the frame.
func cropRect(b vision.Box, marginPct float64, bounds image.Rectangle) image.Rectangle {
	mx := int(float64(b.X2-b.X1) * marginPct / 100)
	my := int(float64(b.Y2-b.Y1) * marginPct / 100)
	return image.Rect(b.X1-mx, b.Y1-my, b.X2+mx, b.Y2+my).Intersect(bounds)
}

// sharpness is the variance of the Laplacian of the crop's brightness: high when it
// has crisp detail, low when it's blurred or flat. Big crops are sampled.
func sharpness(img image.Image, r image.Rectangle) float64 {
	if r.Dx() < 3 || r.Dy() < 3 {
		return 0
	}
	step := max(1, max(r.Dx(), r.Dy())/128)
	lum := func(x, y int) float64 {
		cr, cg, cb, _ := img.At(x, y).RGBA()
		return (0.299*float64(cr) + 0.587*float64(cg) + 0.114*float64(cb)) / 257
	}
	var sum, sumSq float64
	n := 0
	for y := r.Min.Y + step; y < r.Max.Y-step; y += step {
		for x := r.Min.X + step; x < r.Max.X-step; x += step {
			l := 4*lum(x, y) - lum(x-step, y) - lum(x+step, y) - lum(x, y-step) - lum(x, y+step)
			sum += l
			sumSq += l * l
			n++
		}
	}
	if n == 0 {
		return 0
	}
	mean := sum / float64(n)
	return sumSq/float64(n) - mean*mean
}

// writeCrop saves the crop as a JPEG, its longer side scaled down to s.max.
func writeCrop(s save) error {
	if s.img == nil {
		return nil
	}
	r := s.rect.Intersect(s.img.Bounds())
	if r.Empty() {
		return nil
	}
	w, h := r.Dx(), r.Dy()
	scale := 1.0
	if longer := max(w, h); s.max > 0 && longer > s.max {
		scale = float64(s.max) / float64(longer)
	}
	ow, oh := max(1, int(float64(w)*scale)), max(1, int(float64(h)*scale))
	out := image.NewRGBA(image.Rect(0, 0, ow, oh))
	for y := 0; y < oh; y++ {
		sy := r.Min.Y + int(float64(y)/scale)
		for x := 0; x < ow; x++ {
			out.Set(x, y, s.img.At(r.Min.X+int(float64(x)/scale), sy))
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, out, &jpeg.Options{Quality: 85}); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.path, buf.Bytes(), 0o644)
}
