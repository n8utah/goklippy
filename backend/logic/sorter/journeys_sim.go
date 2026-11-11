package sorter

import (
	"context"
	"time"

	"goklippy/backend/logic/vision"
)

// RunSimCameras plays the cameras for the simulated machine: every interval it draws
// each camera channel, tracks its pieces as Perception would, and hands the frame to
// the journey recorder, so the Piece journeys page works without hardware.
func RunSimCameras(ctx context.Context, w *SimWorld, rec *JourneyRecorder, interval time.Duration) {
	trackers := map[Channel]*vision.Tracker{}
	for _, ch := range CameraChannels {
		trackers[ch] = vision.NewTracker()
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		for _, ch := range CameraChannels {
			img, boxes, at := w.Snapshot(ch)
			z := SimZones()
			obs := vision.Observe(z, boxes, 2, at)
			colors := make([][3]float64, len(obs.Pieces))
			for i, p := range obs.Pieces {
				colors[i] = vision.MeanColor(img, p.Box, 2)
			}
			trackers[ch].Update(z, &obs, colors)
			rec.Observe(ch, img, z, obs)
		}
	}
}
