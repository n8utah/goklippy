package sorter

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"goklippy/backend/logic/vision"
)

// When an incident opens, every camera's picture and every channel's state at that
// moment are kept with it, so it can be looked into later exactly as it was.

// IncidentCamera is one channel at the moment an incident opened. C1 has no camera, so
// it has its state only.
type IncidentCamera struct {
	Channel Channel              `json:"channel"`
	Image   bool                 `json:"image"` // a picture was kept
	TakenAt time.Time            `json:"taken_at"`
	State   IncidentChannelState `json:"state"`
}

// IncidentChannelState is what a channel was doing and seeing.
type IncidentChannelState struct {
	MachineState State `json:"machine_state"`
	// Doing is the channel's "Right now" line.
	Doing   string          `json:"doing,omitempty"`
	Summary *ChannelSummary `json:"summary,omitempty"`
	// Observation is what its camera saw in the picture (piece boxes in its pixels).
	Observation *vision.Observation `json:"observation,omitempty"`
	// Focus is where the problem is in the picture, when it's known: the stuck piece,
	// the pile, the piece that won't fall.
	Focus []vision.Box `json:"focus,omitempty"`
}

// IncidentFocus is where an incident's problem is: a box in a channel's picture.
type IncidentFocus struct {
	Channel Channel    `json:"channel"`
	Box     vision.Box `json:"box"`
}

// focusNear is the piece now seen nearest box on ch (where it was when the recovery
// began), or box itself when it isn't seen. A pile is every pile on the channel.
func (m *Machine) focusNear(ch Channel, box vision.Box, pile bool) []IncidentFocus {
	if m.sort == nil || m.sort.deps.Vision == nil {
		return nil
	}
	obs := m.sort.deps.Vision.Observation(ch)
	var out []IncidentFocus
	if pile {
		for _, p := range obs.Pieces {
			if obs.Clump(p.Box) {
				out = append(out, IncidentFocus{ch, p.Box})
			}
		}
	}
	if len(out) == 0 {
		if p, ok := pieceNearIn(obs, box); ok {
			out = append(out, IncidentFocus{ch, p.Box})
		} else if box != (vision.Box{}) {
			out = append(out, IncidentFocus{ch, box})
		}
	}
	return out
}

// focusZone is every piece now seen in zone on ch (every piece for ZoneNone).
func (m *Machine) focusZone(ch Channel, zone vision.Zone) []IncidentFocus {
	if m.sort == nil || m.sort.deps.Vision == nil {
		return nil
	}
	var out []IncidentFocus
	for _, p := range m.sort.deps.Vision.Observation(ch).Pieces {
		if zone == vision.ZoneNone || p.Zone == zone {
			out = append(out, IncidentFocus{ch, p.Box})
		}
	}
	return out
}

// IncidentCameraStore keeps the cameras with the incidents.
type IncidentCameraStore interface {
	AddIncidentCamera(ctx context.Context, incidentID int64, cam IncidentCamera, path string) error
	// IncidentCameraPath is the kept picture of one channel; ErrIncidentImageNotFound
	// when there's none.
	IncidentCameraPath(ctx context.Context, incidentID int64, ch Channel) (string, error)
	// ClearIncidentCameras forgets every incident's pictures and states.
	ClearIncidentCameras(ctx context.Context) (int, error)
	// DeleteIncidentsExcept removes every incident but keep (0 for none) with its
	// cameras, and returns their ids.
	DeleteIncidentsExcept(ctx context.Context, keep int64) ([]int64, error)
}

var (
	ErrNoIncidentStore       = errors.New("incident pictures aren't kept here")
	ErrIncidentImageNotFound = errors.New("no picture kept for that incident and channel")
)

// jpegSource is a vision source that has its cameras' frames as sent.
type jpegSource interface {
	JPEG(ch Channel) ([]byte, time.Time, bool)
}

// incidentShot is one channel's capture before it's saved.
type incidentShot struct {
	cam  IncidentCamera
	jpeg []byte
}

// captureIncident takes every channel's picture and state now and queues them to be
// kept with the incident. Runs on the control loop: it only copies.
func (m *Machine) captureIncident(id int64, at time.Time, focus []IncidentFocus) {
	st := m.sort
	if id == 0 || st == nil || st.deps.Records == nil {
		return
	}
	m.snapMu.RLock()
	snap := m.snap
	m.snapMu.RUnlock()
	var shots []incidentShot
	for _, ch := range []Channel{ChannelC1, ChannelC2, ChannelC3, ChannelC4} {
		cam := IncidentCamera{Channel: ch, TakenAt: at, State: IncidentChannelState{
			MachineState: snap.State, Doing: snap.Doing[ch],
		}}
		if s, ok := snap.Channels[ch]; ok {
			cam.State.Summary = &s
		}
		for _, f := range focus {
			if f.Channel == ch {
				cam.State.Focus = append(cam.State.Focus, f.Box)
			}
		}
		var jpg []byte
		if ch != ChannelC1 && st.deps.Vision != nil {
			obs := st.deps.Vision.Observation(ch)
			cam.State.Observation = &obs
			jpg = frameJPEG(st.deps.Vision, ch)
		}
		shots = append(shots, incidentShot{cam: cam, jpeg: jpg})
	}
	st.deps.Records.saveIncidentCameras(id, shots)
}

// frameJPEG is the channel's newest frame as a JPEG: as the camera sent it, or encoded.
func frameJPEG(v VisionSource, ch Channel) []byte {
	if js, ok := v.(jpegSource); ok {
		if jpg, _, ok := js.JPEG(ch); ok && len(jpg) > 0 {
			return bytes.Clone(jpg)
		}
	}
	img, _, ok := v.Frame(ch)
	if !ok || img == nil {
		return nil
	}
	return encodeJPEG(img)
}

func encodeJPEG(img image.Image) []byte {
	var buf bytes.Buffer
	if jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}) != nil {
		return nil
	}
	return buf.Bytes()
}

func (r *Records) incidentsDir() string { return filepath.Join(r.dataDir, "incidents") }

// saveIncidentCameras queues the pictures and states to be kept with the incident.
func (r *Records) saveIncidentCameras(id int64, shots []incidentShot) {
	store, ok := r.store.(IncidentCameraStore)
	if !ok {
		return
	}
	r.enqueue(fmt.Sprintf("incident %d cameras", id), func(ctx context.Context) error {
		for _, s := range shots {
			path := ""
			if len(s.jpeg) > 0 && r.dataDir != "" {
				rel := filepath.ToSlash(filepath.Join("incidents", strconv.FormatInt(id, 10), string(s.cam.Channel)+".jpg"))
				full := filepath.Join(r.dataDir, filepath.FromSlash(rel))
				if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
					return err
				}
				if err := os.WriteFile(full, s.jpeg, 0o644); err != nil {
					return err
				}
				path = rel
				s.cam.Image = true
			}
			if err := store.AddIncidentCamera(ctx, id, s.cam, path); err != nil {
				return err
			}
		}
		return nil
	})
}

// IncidentImage is the full path of an incident's kept picture of a channel.
func (r *Records) IncidentImage(ctx context.Context, id int64, ch Channel) (string, error) {
	store, ok := r.store.(IncidentCameraStore)
	if !ok {
		return "", ErrNoIncidentStore
	}
	rel, err := store.IncidentCameraPath(ctx, id, ch)
	if err != nil {
		return "", err
	}
	return r.ImagePath(rel)
}

// IncidentPictures is how many incident pictures are kept, and their size.
func (r *Records) IncidentPictures() PhotoUsage {
	if r.dataDir == "" {
		return PhotoUsage{}
	}
	return dirUsage(r.incidentsDir())
}

// ClearIncidentPictures removes every incident's pictures and kept states; the
// incidents stay.
func (r *Records) ClearIncidentPictures(ctx context.Context) (PhotoUsage, error) {
	store, ok := r.store.(IncidentCameraStore)
	if !ok || r.dataDir == "" {
		return PhotoUsage{}, ErrNoIncidentStore
	}
	r.Flush()
	before := r.IncidentPictures()
	if _, err := store.ClearIncidentCameras(ctx); err != nil {
		return PhotoUsage{}, err
	}
	if err := os.RemoveAll(r.incidentsDir()); err != nil {
		return PhotoUsage{}, err
	}
	return before, nil
}

// ClearIncidents removes every incident with its pictures but the one the machine is
// holding now (keep; 0 for none), and returns how many. An incident left marked open by
// a restart goes too: nothing can resolve it any more.
func (r *Records) ClearIncidents(ctx context.Context, keep int64) (int, error) {
	store, ok := r.store.(IncidentCameraStore)
	if !ok {
		return 0, ErrNoIncidentStore
	}
	r.Flush()
	ids, err := store.DeleteIncidentsExcept(ctx, keep)
	if err != nil {
		return 0, err
	}
	if r.dataDir != "" {
		for _, id := range ids {
			if err := os.RemoveAll(filepath.Join(r.incidentsDir(), strconv.FormatInt(id, 10))); err != nil {
				slog.Warn("removing an incident's pictures failed", "incident", id, "error", err)
			}
		}
	}
	return len(ids), nil
}
