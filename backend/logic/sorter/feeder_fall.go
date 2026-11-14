package sorter

// Where a channel's pieces fall off its exit, learned from its falls. An exit move takes
// the leading piece there in one go (as far as the next piece allows) instead of
// creeping up in fixed nudges. The point is how far past the exit zone's start a piece's
// center gets before it drops, in rotor degrees. It starts at the middle of the exit
// zone and is kept for the machine's life (a stop doesn't reset it):
//
//   - a piece that fell short of the point: the point moves back to where it fell;
//   - one that fell on a move aimed at the point: back fallStepDeg, to find the least
//     that works (a smaller move brings the next piece less far);
//   - one that fell on a nudge after an aimed move left it there: where it fell;
//   - one taken to or past the point that didn't fall: just past where it got to.

const fallStepDeg = 0.5

// fallPoint is how far into ch's exit its pieces fall off.
func (f *Feeder) fallPoint(ch Channel, settings MachineSettings) float64 {
	if at, ok := f.fallAt[ch]; ok {
		return at
	}
	return exitWidth(settings, ch) / 2
}

// learnFall updates ch's fall point from an exit move that took its lead reached
// degrees past the exit's start. aimed: the move was aimed at the fall point.
func (f *Feeder) learnFall(ch Channel, settings MachineSettings, reached float64, aimed, fell bool) {
	at := f.fallPoint(ch, settings)
	switch {
	case fell && reached < at:
		at = reached
	case fell && aimed:
		at -= fallStepDeg
	case fell:
		at = reached
	case reached >= at:
		at = reached + fallStepDeg
	default:
		return
	}
	at = max(at, 0)
	if w := exitWidth(settings, ch); w > 0 {
		at = min(at, w)
	}
	f.fallAt[ch] = at
}

// exitWidth is the width of ch's exit zone (0 when its zones aren't set).
func exitWidth(settings MachineSettings, ch Channel) float64 {
	z, ok := settings.Cameras.Zones[string(ch)]
	if !ok {
		return 0
	}
	if w := z.Exit.Width(); w < 360 {
		return w
	}
	return 0
}
