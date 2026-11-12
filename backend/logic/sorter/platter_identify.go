package sorter

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/jpeg"
	"log/slog"
	"math"
	"sort"
	"sync"
	"time"

	"goklippy/backend/logic/brickognize"
	"goklippy/backend/logic/vision"
)

// Identifying a piece, after Sorter V2's classification (two_piece/base.py):
//
//   - Of C4's burst, the photos with the whole piece in view, sharpest first, up to
//     Platter.SendFrames are sent; the rest are spares.
//   - Several requests go to Brickognize at once, not one after another: all the chosen
//     photos together ("combined"), the most settled one alone ("single"), and with
//     Platter.UpstreamFrames, the chosen ones plus the piece's best pictures from C2 and
//     C3 ("with C2/C3": its journey, which only ever holds this piece). The most
//     confident answer that names a part wins; if one with a C2/C3 picture were wrong,
//     that request just loses.
//   - If none names a part, up to Platter.OtherViews spares are sent once more, each
//     alone, at once: the ones least like what was sent. A piece is often known by
//     another angle (Sorter V2: a minifigure head unmatched on four frames of its face,
//     76% on one from the side).

const (
	strategyCombined = "combined"
	strategySingle   = "single"
	strategyUpstream = "with C2/C3"
	strategyOther    = "another view"
)

// burstPhoto is one C4 photo, cut out and judged.
type burstPhoto struct {
	jpg   []byte
	at    time.Time
	sharp float64
	whole bool      // the piece's box is clear of the picture's edge
	thumb []float64 // a small grey picture, for telling views apart
}

type identifyAttempt struct {
	label string
	res   brickognize.Result
	err   error
}

// identify cuts out the shots, saves them with the piece, and asks Brickognize.
func (p *Platter) identify(id string, shots []platterShot, ps PlatterSettings) identResult {
	photos := make([]burstPhoto, 0, len(shots))
	for _, sh := range shots {
		jpg, err := sh.crop()
		if err != nil {
			slog.Warn("cropping a photo of a piece failed", "piece", id, "error", err)
			continue
		}
		ph := burstPhoto{jpg: jpg, at: sh.at, whole: !touchesEdge(sh.box, sh.frame.Bounds())}
		if img, err := jpeg.Decode(bytes.NewReader(jpg)); err == nil {
			ph.sharp, ph.thumb = sharpness(img, img.Bounds()), greyThumb(img)
		}
		photos = append(photos, ph)
	}
	if len(photos) == 0 {
		return identResult{pieceID: id, err: errors.New("no photo of it could be cut out")}
	}
	chosen := chooseBurst(photos, ps.SendFrames)
	isChosen := map[int]bool{}
	for _, i := range chosen {
		isChosen[i] = true
	}
	for i, ph := range photos {
		p.deps.Records.SaveImage(id, i, ph.jpg, isChosen[i], ph.at)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(ps.IdentifyTimeoutMs)*time.Millisecond)
	defer cancel()

	send := make([][]byte, len(chosen))
	for k, i := range chosen {
		send[k] = photos[i].jpg
	}
	reqs := []identifyAttempt{{label: strategyCombined}}
	payload := map[string][][]byte{strategyCombined: send}
	if len(send) > 1 {
		reqs = append(reqs, identifyAttempt{label: strategySingle})
		payload[strategySingle] = send[len(send)-1:] // the most settled
	}
	if p.deps.Upstream != nil && ps.UpstreamFrames > 0 {
		up := p.deps.Upstream(id, min(ps.UpstreamFrames, brickognize.MaxImages-1))
		if len(up) > 0 {
			mine := send[max(0, len(send)-(brickognize.MaxImages-len(up))):]
			reqs = append(reqs, identifyAttempt{label: strategyUpstream})
			payload[strategyUpstream] = append(append([][]byte{}, mine...), up...)
		}
	}
	p.askAll(ctx, reqs, payload)
	best := bestAttempt(reqs)

	if !names(best) && ps.OtherViews > 0 {
		var spare []int
		for i, ph := range photos {
			if !isChosen[i] && ph.whole {
				spare = append(spare, i)
			}
		}
		if others := leastAlike(photos, chosen, spare, ps.OtherViews); len(others) > 0 {
			more := make([]identifyAttempt, len(others))
			pay := map[string][][]byte{}
			for k, i := range others {
				label := strategyOther + " " + string(rune('1'+k))
				more[k] = identifyAttempt{label: label}
				pay[label] = [][]byte{photos[i].jpg}
			}
			p.askAll(ctx, more, pay)
			if alt := bestAttempt(more); names(alt) {
				best = alt
			}
		}
	}
	slog.Info("piece identified", "piece", id, "by", best.label, "requests", len(reqs), "error", best.err)
	return identResult{pieceID: id, result: best.res, err: best.err, strategy: best.label}
}

// askAll sends every request at once and fills in their answers.
func (p *Platter) askAll(ctx context.Context, reqs []identifyAttempt, payload map[string][][]byte) {
	var wg sync.WaitGroup
	for i := range reqs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			reqs[i].res, reqs[i].err = p.deps.Identifier.Identify(ctx, payload[reqs[i].label])
		}()
	}
	wg.Wait()
}

func names(a identifyAttempt) bool { return a.err == nil && len(a.res.Items) > 0 }

// bestAttempt is the most confident answer that names a part (the earlier on a tie),
// else the first that answered at all, else the first error.
func bestAttempt(reqs []identifyAttempt) identifyAttempt {
	best := -1
	for i, a := range reqs {
		if names(a) && (best < 0 || a.res.Items[0].Score > reqs[best].res.Items[0].Score) {
			best = i
		}
	}
	if best >= 0 {
		return reqs[best]
	}
	for _, a := range reqs {
		if a.err == nil {
			return a
		}
	}
	return reqs[0]
}

// chooseBurst picks up to n photos to send: those with the whole piece in view,
// sharpest first (all of them if none is whole), returned in the order taken.
func chooseBurst(photos []burstPhoto, n int) []int {
	n = min(max(1, n), brickognize.MaxImages)
	idx := make([]int, len(photos))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool {
		pa, pb := photos[idx[a]], photos[idx[b]]
		if pa.whole != pb.whole {
			return pa.whole
		}
		return pa.sharp > pb.sharp
	})
	if whole := countWhole(photos); whole > 0 {
		n = min(n, whole)
	}
	chosen := append([]int(nil), idx[:min(n, len(idx))]...)
	sort.Ints(chosen)
	return chosen
}

func countWhole(photos []burstPhoto) int {
	n := 0
	for _, ph := range photos {
		if ph.whole {
			n++
		}
	}
	return n
}

// leastAlike picks up to n of spare, each the one least like everything sent or picked
// so far.
func leastAlike(photos []burstPhoto, sent, spare []int, n int) []int {
	seen := make([][]float64, 0, len(sent)+n)
	for _, i := range sent {
		seen = append(seen, photos[i].thumb)
	}
	left := append([]int(nil), spare...)
	var picked []int
	for len(left) > 0 && len(picked) < n {
		bestK, bestD := 0, -1.0
		for k, i := range left {
			d := math.Inf(1)
			for _, t := range seen {
				d = min(d, thumbDiff(photos[i].thumb, t))
			}
			if math.IsInf(d, 1) {
				d = 0
			}
			if d > bestD {
				bestK, bestD = k, d
			}
		}
		i := left[bestK]
		picked = append(picked, i)
		seen = append(seen, photos[i].thumb)
		left = append(left[:bestK], left[bestK+1:]...)
	}
	return picked
}

const thumbSide = 16

// greyThumb is a 16×16 grey picture of img, evened out (mean 0, spread 1).
func greyThumb(img image.Image) []float64 {
	b := img.Bounds()
	out := make([]float64, thumbSide*thumbSide)
	if b.Dx() < 1 || b.Dy() < 1 {
		return out
	}
	for y := range thumbSide {
		for x := range thumbSide {
			r, g, bl, _ := img.At(b.Min.X+x*b.Dx()/thumbSide, b.Min.Y+y*b.Dy()/thumbSide).RGBA()
			out[y*thumbSide+x] = 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(bl)
		}
	}
	mean, sq := 0.0, 0.0
	for _, v := range out {
		mean += v
	}
	mean /= float64(len(out))
	for _, v := range out {
		sq += (v - mean) * (v - mean)
	}
	sd := math.Sqrt(sq/float64(len(out))) + 1e-6
	for i := range out {
		out[i] = (out[i] - mean) / sd
	}
	return out
}

func thumbDiff(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	d := 0.0
	for i := range a {
		d += math.Abs(a[i] - b[i])
	}
	return d / float64(len(a))
}

// touchesEdge: the box reaches the edge of the picture, so part of the piece may be
// out of view.
func touchesEdge(b vision.Box, frame image.Rectangle) bool {
	return b.X1 <= frame.Min.X+1 || b.Y1 <= frame.Min.Y+1 || b.X2 >= frame.Max.X-2 || b.Y2 >= frame.Max.Y-2
}
