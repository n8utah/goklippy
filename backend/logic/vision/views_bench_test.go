package vision

import (
	"image"
	"testing"
)

// BenchmarkDetectViews: one frame against 180 views (every 2°) at 640×480.
func BenchmarkDetectViews(b *testing.B) {
	z := testZones()
	var views []image.Image
	for a := 0.0; a < 360; a += 2 {
		views = append(views, rotorFrame(a))
	}
	d := NewBackgroundDetector()
	if err := d.SetViews(views); err != nil {
		b.Fatal(err)
	}
	frame := withPiece(rotorFrame(37), at(230, 20))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := d.Detect(frame, z); err != nil {
			b.Fatal(err)
		}
	}
}
