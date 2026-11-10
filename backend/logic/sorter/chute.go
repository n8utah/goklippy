package sorter

import (
	"errors"
	"fmt"
	"math"
)

// Chute geometry, after Sorter V2 (subsystems/distribution/chute.py).
//
// The chute homes counter-clockwise onto its switch, which is 0°, and from there only
// ever turns clockwise, up to MaxAngle: it can't cross the home stop. Each layer is
// split into Sections equal sections (the bays between the frame's columns), each
// section holding K = bins/Sections bins. The geometry doesn't depend on K:
//
//	pitch      = 360 / Sections                  (section start to section start)
//	bin center = FirstOffset + s*pitch + (i + 0.5) * SectionWidth/K
//
// so one calibration serves 12-bin (K=2) and 18-bin (K=3) layers alike. A bin whose
// angle, wrapped onto the circle, lands past MaxAngle is in the dead wedge just before
// home and can't be reached.

var (
	ErrBinUnreachable = errors.New("the chute can't reach that bin")
	ErrNoSuchBin      = errors.New("no such bin")
)

// BinAddress names a bin the way the UI does: layer and bin number, both from 1.
type BinAddress struct {
	Layer int `json:"layer"`
	Bin   int `json:"bin"`
}

// BinPosition is where a bin is on the tower and where the chute aims for it.
type BinPosition struct {
	Layer     int     `json:"layer"`      // from 1
	Bin       int     `json:"bin"`        // from 1, counting up clockwise from home
	Section   int     `json:"section"`    // from 1
	InSection int     `json:"in_section"` // from 1
	Angle     float64 `json:"angle"`      // chute angle, degrees clockwise from home
	Reachable bool    `json:"reachable"`
}

// SectionPitch is the angle from one section's start to the next.
func (d DistributorSettings) SectionPitch() float64 {
	return 360 / float64(max(1, d.Sections))
}

// checkGeometry reports settings that can't describe a real tower.
func (d DistributorSettings) checkGeometry() error {
	switch {
	case d.Sections < 1:
		return fmt.Errorf("%w: the chute needs at least one section", ErrInvalidSettings)
	case d.SectionWidth <= 0 || d.SectionWidth > d.SectionPitch()+1e-9:
		return fmt.Errorf("%w: section width %.2f° must be above 0 and at most %.2f°", ErrInvalidSettings, d.SectionWidth, d.SectionPitch())
	case d.MaxAngle <= 0 || d.MaxAngle > 360:
		return fmt.Errorf("%w: the chute's travel limit must be between 0 and 360°", ErrInvalidSettings)
	}
	return nil
}

// binsPerSection is K for one layer (layer counts from 1).
func (d DistributorSettings) binsPerSection(layer int) (int, error) {
	if layer < 1 || layer > len(d.Layers) {
		return 0, fmt.Errorf("%w: layer %d does not exist", ErrNoSuchBin, layer)
	}
	bins := d.Layers[layer-1].Bins
	if bins < d.Sections || bins%d.Sections != 0 {
		return 0, fmt.Errorf("%w: layer %d has %d bins, which doesn't split into %d sections", ErrInvalidSettings, layer, bins, d.Sections)
	}
	return bins / d.Sections, nil
}

// binAngle is the unwrapped aim for bin i (from 0) of section s (from 0) with k bins
// per section.
func (d DistributorSettings) binAngle(s, i, k int) float64 {
	return d.FirstOffset + float64(s)*d.SectionPitch() + (float64(i)+0.5)*d.SectionWidth/float64(k)
}

// wrapAim puts an angle onto the chute's travel: it's reachable if, wrapped onto the
// circle, it lies within [0, MaxAngle].
func (d DistributorSettings) wrapAim(angle float64) (float64, bool) {
	norm := math.Mod(angle, 360)
	if norm < 0 {
		norm += 360
	}
	return norm, norm <= d.MaxAngle
}

// BinPosition locates one bin.
func (d DistributorSettings) BinPosition(addr BinAddress) (BinPosition, error) {
	if err := d.checkGeometry(); err != nil {
		return BinPosition{}, err
	}
	k, err := d.binsPerSection(addr.Layer)
	if err != nil {
		return BinPosition{}, err
	}
	if addr.Bin < 1 || addr.Bin > k*d.Sections {
		return BinPosition{}, fmt.Errorf("%w: layer %d has no bin %d", ErrNoSuchBin, addr.Layer, addr.Bin)
	}
	s, i := (addr.Bin-1)/k, (addr.Bin-1)%k
	angle, ok := d.wrapAim(d.binAngle(s, i, k))
	return BinPosition{
		Layer: addr.Layer, Bin: addr.Bin, Section: s + 1, InSection: i + 1,
		Angle: angle, Reachable: ok,
	}, nil
}

// LayerBins lists every bin of every layer, in bin order. Layers whose bin count
// doesn't split into the sections are left empty.
func (d DistributorSettings) LayerBins() [][]BinPosition {
	out := make([][]BinPosition, len(d.Layers))
	for l := range d.Layers {
		for b := 1; ; b++ {
			pos, err := d.BinPosition(BinAddress{Layer: l + 1, Bin: b})
			if err != nil {
				break
			}
			out[l] = append(out[l], pos)
		}
	}
	return out
}

// FirstBinAngle is where the chute parks after homing: the first bin of the first
// section, sized for the first layer (as Sorter V2's back-off to bin 1).
func (d DistributorSettings) FirstBinAngle() float64 {
	k := 1
	if len(d.Layers) > 0 && d.Sections > 0 && d.Layers[0].Bins >= d.Sections {
		k = d.Layers[0].Bins / d.Sections
	}
	angle, _ := d.wrapAim(d.binAngle(0, 0, k))
	return angle
}

// ChuteCalibration is the geometry worked out from two captured bins.
type ChuteCalibration struct {
	SectionWidth float64 `json:"section_width"`
	FirstOffset  float64 `json:"first_section_offset"`
}

// CalibrateChute works out the geometry from the chute angles of the first and last
// bin of one section with k bins: their centers are W/k*(k-1) apart, and the first
// sits half a bin past the section's start.
func CalibrateChute(firstAngle, lastAngle float64, k, sections int) (ChuteCalibration, error) {
	if k < 2 {
		return ChuteCalibration{}, fmt.Errorf("calibrate on a layer with at least 2 bins per section")
	}
	if sections < 1 {
		return ChuteCalibration{}, fmt.Errorf("the chute needs at least one section")
	}
	if lastAngle <= firstAngle {
		return ChuteCalibration{}, fmt.Errorf("the last bin (%.2f°) must be clockwise of the first (%.2f°)", lastAngle, firstAngle)
	}
	width := (lastAngle - firstAngle) * float64(k) / float64(k-1)
	if pitch := 360 / float64(sections); width > pitch {
		return ChuteCalibration{}, fmt.Errorf("those captures make a section %.2f° wide, more than the %.2f° between sections; capture two bins of the same section", width, pitch)
	}
	return ChuteCalibration{SectionWidth: width, FirstOffset: firstAngle - width/(2*float64(k))}, nil
}
