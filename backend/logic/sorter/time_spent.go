package sorter

import "time"

// Where the time goes: each control tick while sorting, every channel is in one of a
// few states, and the run adds them up. Run › Sort shows the shares per channel, so
// the slowest link (a channel mostly waiting on the next, or mostly empty) shows.
const (
	TimeMoving  = "moving"  // turning: a pulse, an index, a bulk nudge
	TimeBusy    = "busy"    // C4 photographing, identifying or aiming; C2/C3 holding after a fall
	TimeWaiting = "waiting" // ready, but the next channel can't take a piece (C4: waiting to turn)
	TimeEmpty   = "empty"   // nothing to do: no pieces, or no picture to go on
	TimeStuck   = "stuck"   // a stuck piece being recovered, or stopped for an incident
)

// timeChannels are the channels time is kept for.
var timeChannels = []Channel{ChannelC1, ChannelC2, ChannelC3, ChannelC4}

// maxTickGap: a gap longer than this between ticks (the run was paused, or the loop
// stalled) isn't counted.
const maxTickGap = time.Second

// timeSpent is a run's time per channel per state.
type timeSpent struct {
	last time.Time
	ms   map[Channel]map[string]int64
}

func newTimeSpent() *timeSpent {
	return &timeSpent{ms: map[Channel]map[string]int64{}}
}

// tick returns the time since the last tick that counts, and starts the next.
func (t *timeSpent) tick(now time.Time) time.Duration {
	dt := now.Sub(t.last)
	t.last = now
	if dt <= 0 || dt > maxTickGap {
		return 0
	}
	return dt
}

func (t *timeSpent) add(ch Channel, state string, dt time.Duration) {
	if dt <= 0 || state == "" {
		return
	}
	if t.ms[ch] == nil {
		t.ms[ch] = map[string]int64{}
	}
	t.ms[ch][state] += dt.Milliseconds()
}

func (t *timeSpent) addAll(state string, dt time.Duration) {
	for _, ch := range timeChannels {
		t.add(ch, state, dt)
	}
}

// snapshot is a copy for the dashboard.
func (t *timeSpent) snapshot() map[Channel]map[string]int64 {
	out := make(map[Channel]map[string]int64, len(t.ms))
	for ch, states := range t.ms {
		cp := make(map[string]int64, len(states))
		for k, v := range states {
			cp[k] = v
		}
		out[ch] = cp
	}
	return out
}
