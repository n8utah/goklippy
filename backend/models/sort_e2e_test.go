package models

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"goklippy/backend/logic/brickognize"
	"goklippy/backend/logic/sorter"
	"goklippy/backend/logic/vision"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fastSettings runs the simulated machine quickly: high speeds, short pauses.
func fastSettings() sorter.MachineSettings {
	s := sorter.DefaultMachineSettings()
	s.FeederRpm = map[string]float64{"c1": 600, "c2": 600, "c3": 600}
	s.IndexRpm = 600
	s.IndexAccel = 0
	s.Distributor.Rpm = 600
	s.Distributor.Accel = 0
	s.Distributor.DoorSettleMs = 0
	s.Distributor.Layers = []sorter.LayerSettings{{Bins: 12}, {Bins: 12}}
	s.Feeder.DropPauseMs, s.Feeder.ExitPauseMs, s.Feeder.BulkPauseMs, s.Feeder.HoldAfterFallMs = 5, 5, 50, 20
	s.Platter.SettleMs, s.Platter.BurstFrames, s.Platter.IdleIndexMs = 5, 3, 100
	s.Platter.IdentifyTimeoutMs = 3000
	zones := sorter.SimZones()
	s.Cameras.Zones = map[string]vision.ChannelZones{"c2": zones, "c3": zones, "c4": zones}
	return s
}

func TestSortEndToEndInSimulation(t *testing.T) {
	db := openTestDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	settings := fastSettings
	hw := sorter.NewSimHardware(settings)
	supply := []sorter.SimPart{sorter.SimCatalog[0], sorter.SimCatalog[2], sorter.SimCatalog[0], sorter.SimCatalog[5], sorter.SimCatalog[3], sorter.SimCatalog[1]}
	world := sorter.NewSimWorld(hw, settings, supply)

	sorting := sorter.NewSortingService(ctx, NewSortingStore(db), settings)
	p, err := sorting.Import(ctx, []byte(bricksProfile))
	require.NoError(t, err)
	require.NoError(t, sorting.Activate(ctx, p.ID, sorter.ActivateKeep))

	records := sorter.NewRecords(NewRecordsStore(db), t.TempDir())
	go records.Run(ctx)

	m := sorter.NewMachine(hw, settings, 2*time.Millisecond)
	m.AttachSorting(sorter.SortDeps{Vision: world, Identifier: world, Sorting: sorting, Records: records})
	go m.Run(ctx)

	_, err = m.Do(ctx, sorter.CommandHome)
	require.NoError(t, err)
	require.Eventually(t, func() bool { return m.Snapshot().State == sorter.StateReady }, 5*time.Second, 5*time.Millisecond)
	_, err = m.Do(ctx, sorter.CommandStart)
	require.NoError(t, err)

	// The world sees a piece fall as the index starts; the platter records it when
	// the index ends. Wait for both.
	require.Eventually(t, func() bool {
		return len(world.Falls()) == len(supply) && len(m.Snapshot().Aboard) == 0
	}, 30*time.Second, 10*time.Millisecond,
		"%d of %d pieces delivered; %d still on the channels", len(world.Falls()), len(supply), world.OnChannels())
	snap, err := m.Do(ctx, sorter.CommandStop)
	require.NoError(t, err)
	assert.Empty(t, snap.Aboard)
	records.Flush()

	// Every piece fell where its category's bin is: both bricks (red and yellow) in
	// one bin, both plates in another, and the unidentified mystery part in the
	// bucket.
	d := settings().Distributor
	binOf := map[string]sorter.BinPosition{}
	for _, f := range world.Falls() {
		switch f.Part.ID {
		case "x9999":
			assert.Equal(t, 0, f.Layer, "unidentified: every layer closed, so the bucket")
			continue
		}
		require.NotZero(t, f.Layer, "%s went to the bucket", f.Part.ID)
		group := map[string]string{"3001": "bricks", "3003": "bricks", "3020": "plates", "3023": "plates"}[f.Part.ID]
		pos, seen := binOf[group]
		if !seen {
			// Find the bin the chute pointed at.
			for _, layer := range d.LayerBins() {
				for _, b := range layer {
					if b.Layer == f.Layer && abs(b.Angle-f.ChuteAngle) < 0.01 {
						pos = b
					}
				}
			}
			require.NotZero(t, pos.Bin, "the chute pointed at %.2f°, not a bin", f.ChuteAngle)
			binOf[group] = pos
		}
		assert.Equal(t, pos.Layer, f.Layer, "%s", f.Part.ID)
		assert.InDelta(t, pos.Angle, f.ChuteAngle, 0.01, "%s", f.Part.ID)
	}
	assert.NotEqual(t, binOf["bricks"], binOf["plates"], "bricks and plates in different bins")

	// The records say the same.
	store := NewRecordsStore(db)
	pieces, total, err := store.ListPieces(ctx, sorter.PieceFilter{Limit: 50})
	require.NoError(t, err)
	assert.Equal(t, len(supply), total)
	byPart := map[string]int{}
	for _, piece := range pieces {
		assert.Equal(t, sorter.PieceDistributed, piece.Status, "piece %s (%s)", piece.ID, piece.PartID)
		byPart[piece.PartID]++
		if piece.PartID == "3001" {
			assert.Equal(t, "r-bricks", piece.Category)
			assert.Equal(t, binOf["bricks"].Bin, piece.BinNumber)
		}
		full, err := store.GetPiece(ctx, piece.ID)
		require.NoError(t, err)
		assert.Len(t, full.Images, 3, "a burst of photos for %s", piece.PartID)
	}
	assert.Equal(t, map[string]int{"3001": 2, "3020": 1, "3023": 1, "3003": 1, "": 1}, byPart)

	runs, err := store.ListRuns(ctx, 5)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.NotNil(t, runs[0].EndedAt)
	assert.Equal(t, len(supply), runs[0].Distributed)
}

// With cameras over C2, C3 and C4, each piece's journey carries on across the falls
// and ends as its record.
func TestSortLinksJourneysAcrossChannels(t *testing.T) {
	db := openTestDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	settings := func() sorter.MachineSettings {
		s := fastSettings()
		// Real speeds, so the simulated cameras see each piece ride rather than jump
		// (their frames come irregularly on a busy test machine).
		s.FeederRpm = map[string]float64{"c1": 20, "c2": 20, "c3": 20}
		s.IndexRpm = 20
		return s
	}
	hw := sorter.NewSimHardware(settings)
	supply := []sorter.SimPart{sorter.SimCatalog[0], sorter.SimCatalog[2], sorter.SimCatalog[3]}
	world := sorter.NewSimWorld(hw, settings, supply)
	sorting := sorter.NewSortingService(ctx, NewSortingStore(db), settings)
	p, err := sorting.Import(ctx, []byte(bricksProfile))
	require.NoError(t, err)
	require.NoError(t, sorting.Activate(ctx, p.ID, sorter.ActivateKeep))
	records := sorter.NewRecords(NewRecordsStore(db), t.TempDir())
	go records.Run(ctx)
	journeys := sorter.NewJourneyRecorder(t.TempDir(), settings)
	go sorter.RunSimCameras(ctx, world, journeys, 10*time.Millisecond)

	m := sorter.NewMachine(hw, settings, 2*time.Millisecond)
	m.AttachSorting(sorter.SortDeps{Vision: world, Identifier: world, Sorting: sorting, Records: records, Journeys: journeys})
	go m.Run(ctx)
	_, err = m.Do(ctx, sorter.CommandHome)
	require.NoError(t, err)
	require.Eventually(t, func() bool { return m.Snapshot().State == sorter.StateReady }, 5*time.Second, 5*time.Millisecond)
	_, err = m.Do(ctx, sorter.CommandStart)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		return len(world.Falls()) == len(supply) && len(m.Snapshot().Aboard) == 0
	}, 60*time.Second, 10*time.Millisecond, "%d of %d delivered", len(world.Falls()), len(supply))
	_, err = m.Do(ctx, sorter.CommandStop)
	require.NoError(t, err)
	records.Flush()

	pieces, _, err := NewRecordsStore(db).ListPieces(ctx, sorter.PieceFilter{Limit: 10})
	require.NoError(t, err)
	require.Len(t, pieces, len(supply))
	for _, piece := range pieces {
		raw, err := journeys.ForPiece(piece.ID)
		require.NoError(t, err, "piece %s (%s) has no journey", piece.ID, piece.PartID)
		var j sorter.Journey
		require.NoError(t, json.Unmarshal(raw, &j))
		var chans []sorter.Channel
		for _, tr := range j.Tracks {
			chans = append(chans, tr.Channel)
		}
		assert.Equal(t, []sorter.Channel{sorter.ChannelC2, sorter.ChannelC3, sorter.ChannelC4}, chans, "piece %s", piece.PartID)
		for _, h := range j.Handoffs {
			assert.Equal(t, sorter.HandoffLinked, h.Result, "%s→%s", h.From, h.To)
		}
	}
}

func TestSortWithTheChuteLeftOut(t *testing.T) {
	settings := func() sorter.MachineSettings {
		s := fastSettings()
		s.Distributor.LeftOut = true
		return s
	}
	db := openTestDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	hw := sorter.NewSimHardware(settings)
	world := sorter.NewSimWorld(hw, settings, sorter.SimCatalog[:3])
	// No profile is imported or active: with the chute left out none is needed.
	sorting := sorter.NewSortingService(ctx, NewSortingStore(db), settings)
	records := sorter.NewRecords(NewRecordsStore(db), t.TempDir())
	go records.Run(ctx)
	m := sorter.NewMachine(hw, settings, 2*time.Millisecond)
	m.AttachSorting(sorter.SortDeps{Vision: world, Identifier: world, Sorting: sorting, Records: records})
	go m.Run(ctx)

	_, err := m.Do(ctx, sorter.CommandHome)
	require.NoError(t, err)
	require.Eventually(t, func() bool { return m.Snapshot().State == sorter.StateReady }, 5*time.Second, 5*time.Millisecond)
	snap := m.Snapshot()
	assert.False(t, snap.Chute.Homed, "the chute isn't homed")
	assert.True(t, snap.Chute.LeftOut)

	_, err = m.Do(ctx, sorter.CommandStart)
	require.NoError(t, err)
	require.Eventually(t, func() bool { return len(m.Snapshot().Recent) == 3 }, 30*time.Second, 10*time.Millisecond)
	assert.Equal(t, sorter.StateRunning, m.Snapshot().State, "it keeps sorting without a homed chute")
	for _, piece := range m.Snapshot().Recent {
		assert.Equal(t, sorter.PieceDistributed, piece.Status, piece.PartID)
		assert.Equal(t, sorter.RouteNoChute, piece.RouteReason, piece.PartID)
		assert.Zero(t, piece.BinLayer, piece.PartID)
		assert.NotEmpty(t, piece.PartID, "still identified")
		assert.Empty(t, piece.CategoryName, "no profile is asked")
	}
	for _, mv := range hw.Moves() {
		assert.NotEqual(t, sorter.ChannelDistributor, mv.Channel, "the chute never moves")
	}
}

// failingIdentifier fails every request, or blocks until its context ends.
type failingIdentifier struct{ block bool }

func (f failingIdentifier) Identify(ctx context.Context, _ [][]byte) (brickognize.Result, error) {
	if f.block {
		<-ctx.Done()
		return brickognize.Result{}, ctx.Err()
	}
	return brickognize.Result{}, errors.New("brickognize: 503 Service Unavailable")
}

func startSimSort(t *testing.T, settings func() sorter.MachineSettings, supply []sorter.SimPart, id sorter.Identifier) (*sorter.Machine, *sorter.SimWorld, *RecordsStore) {
	t.Helper()
	db := openTestDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	hw := sorter.NewSimHardware(settings)
	world := sorter.NewSimWorld(hw, settings, supply)
	sorting := sorter.NewSortingService(ctx, NewSortingStore(db), settings)
	p, err := sorting.Import(ctx, []byte(bricksProfile))
	require.NoError(t, err)
	require.NoError(t, sorting.Activate(ctx, p.ID, sorter.ActivateKeep))
	store := NewRecordsStore(db)
	records := sorter.NewRecords(store, t.TempDir())
	go records.Run(ctx)
	if id == nil {
		id = world
	}
	m := sorter.NewMachine(hw, settings, 2*time.Millisecond)
	m.AttachSorting(sorter.SortDeps{Vision: world, Identifier: id, Sorting: sorting, Records: records})
	go m.Run(ctx)
	_, err = m.Do(ctx, sorter.CommandHome)
	require.NoError(t, err)
	require.Eventually(t, func() bool { return m.Snapshot().State == sorter.StateReady }, 5*time.Second, 5*time.Millisecond)
	_, err = m.Do(ctx, sorter.CommandStart)
	require.NoError(t, err)
	return m, world, store
}

func TestSortIdentificationFailureGoesToTheBucket(t *testing.T) {
	m, world, store := startSimSort(t, fastSettings, []sorter.SimPart{sorter.SimCatalog[0]}, failingIdentifier{})
	require.Eventually(t, func() bool { return len(world.Falls()) == 1 && len(m.Snapshot().Aboard) == 0 }, 20*time.Second, 10*time.Millisecond)
	assert.Equal(t, 0, world.Falls()[0].Layer, "the bucket")

	recent := m.Snapshot().Recent
	require.Len(t, recent, 1)
	assert.Equal(t, sorter.PieceDistributed, recent[0].Status)
	assert.Equal(t, "misc", recent[0].Category)
	require.NotEmpty(t, recent[0].Problems)
	assert.Equal(t, sorter.ProblemIDFailed, recent[0].Problems[0].Kind)
	_ = store
}

// finishSoon: the run finishes on its own once C2 has had no piece for 2 s.
func finishSoon() sorter.MachineSettings {
	s := fastSettings()
	s.Feeder.AutoStopSec = 2
	s.Feeder.FinishMaxSec = 60
	return s
}

// The hopper runs dry: the run finishes on its own, sorting every piece first, then
// stops (Ready, the run ended) and says why.
func TestSortFinishesOnItsOwnWhenC2RunsDry(t *testing.T) {
	m, world, store := startSimSort(t, finishSoon, []sorter.SimPart{sorter.SimCatalog[0], sorter.SimCatalog[1]}, nil)

	require.Eventually(t, func() bool { return m.Snapshot().Finish != nil }, 30*time.Second, 10*time.Millisecond,
		"finishing once C2 has been empty a while")
	assert.Contains(t, m.Snapshot().Finish.Reason, "no new pieces on C2")
	require.Eventually(t, func() bool { return m.Snapshot().State == sorter.StateReady }, 60*time.Second, 20*time.Millisecond,
		"and then stopped on its own")

	assert.Len(t, world.Falls(), 2, "both pieces sorted before it stopped")
	fin := m.Snapshot().Finish
	require.NotNil(t, fin, "how it finished stays shown")
	assert.True(t, fin.Done)
	assert.Equal(t, "everything left was sorted", fin.Step)
	assert.Zero(t, m.Snapshot().RunID)

	runs, err := store.ListRuns(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.NotNil(t, runs[0].EndedAt, "the run was ended")
	assert.Equal(t, 2, runs[0].Distributed)
}

// Keep feeding calls finishing off, and the run carries on as usual.
func TestSortKeepFeedingCallsFinishingOff(t *testing.T) {
	m, _, _ := startSimSort(t, finishSoon, nil, nil)
	require.Eventually(t, func() bool { f := m.Snapshot().Finish; return f != nil && !f.Done }, 30*time.Second, 10*time.Millisecond)
	assert.Equal(t, "finishing: not feeding, C2 has had no pieces for a while", m.Snapshot().Doing[sorter.ChannelC1])

	require.NoError(t, m.KeepFeeding(context.Background()))
	assert.Nil(t, m.Snapshot().Finish)
	assert.Equal(t, sorter.StateRunning, m.Snapshot().State)
	assert.Error(t, m.KeepFeeding(context.Background()), "nothing to call off")
}

// stallSettings: Brickognize hangs and the platter waits for it, so it stalls.
func stallSettings() sorter.MachineSettings {
	s := fastSettings()
	s.Platter.IdentifyTimeoutMs = 60_000
	s.Platter.StallMs = 300
	return s
}

func TestSortStallIsClearedByTurningTheChannel(t *testing.T) {
	m, world, _ := startSimSort(t, stallSettings, []sorter.SimPart{sorter.SimCatalog[0]}, failingIdentifier{block: true})
	require.Eventually(t, func() bool { return len(world.Falls()) == 1 && len(m.Snapshot().Recent) == 1 }, 20*time.Second, 10*time.Millisecond)
	assert.Equal(t, 0, world.Falls()[0].Layer, "into the bucket, every layer closed")
	piece := m.Snapshot().Recent[0]
	require.NotEmpty(t, piece.Problems)
	assert.Equal(t, sorter.ProblemSweptOff, piece.Problems[len(piece.Problems)-1].Kind)
	assert.Nil(t, m.Snapshot().Incident, "cleared, so no incident")
	require.NotNil(t, m.Snapshot().LastJob)
	assert.Contains(t, m.Snapshot().LastJob.Name, "clear the classification channel")
}

func TestSortStallRaisesAnIncident(t *testing.T) {
	// A piece stuck to C4 that turning can't clear.
	stuck := sorter.SimCatalog[0]
	stuck.HeldOn, stuck.HeldShakes = sorter.ChannelC4, 1_000_000
	m, _, store := startSimSort(t, stallSettings, []sorter.SimPart{stuck}, failingIdentifier{block: true})
	require.Eventually(t, func() bool { return m.Snapshot().Incident != nil }, 20*time.Second, 10*time.Millisecond)
	inc := m.Snapshot().Incident
	assert.Equal(t, sorter.IncidentPlatterStuck, inc.Kind)
	assert.Equal(t, []string{sorter.ResolveDone}, inc.Actions)

	ctx := context.Background()
	assert.Error(t, m.ResolveIncident(ctx, "dance"))
	require.NoError(t, m.ResolveIncident(ctx, sorter.ResolveDone))
	assert.Nil(t, m.Snapshot().Incident)
	assert.ErrorIs(t, m.ResolveIncident(ctx, sorter.ResolveDone), sorter.ErrNoIncident)

	incs, err := store.ListIncidents(ctx, 5)
	require.NoError(t, err)
	require.Len(t, incs, 1)
	assert.Equal(t, sorter.ResolveDone, incs[0].Resolution)

	// Every channel's picture and state from when it opened were kept with it.
	require.Eventually(t, func() bool {
		incs, err = store.ListIncidents(ctx, 5)
		return err == nil && len(incs[0].Cameras) == 4
	}, 5*time.Second, 10*time.Millisecond)
	for _, cam := range incs[0].Cameras {
		assert.Equal(t, cam.Channel != sorter.ChannelC1, cam.Image, "%s: a picture, but none for C1", cam.Channel)
		assert.Equal(t, sorter.StateRunning, cam.State.MachineState)
		assert.NotEmpty(t, cam.State.Doing, "%s: what it was doing", cam.Channel)
		if cam.Channel == sorter.ChannelC4 {
			require.NotNil(t, cam.State.Observation)
			assert.NotEmpty(t, cam.State.Observation.Pieces, "the stuck piece, as C4 saw it")
			assert.NotEmpty(t, cam.State.Focus, "where to look: the piece left on C4")
		} else {
			assert.Empty(t, cam.State.Focus, "%s: nothing wrong there", cam.Channel)
		}
	}
	path, err := store.IncidentCameraPath(ctx, incs[0].ID, sorter.ChannelC4)
	require.NoError(t, err)
	assert.NotEmpty(t, path)
	_, err = store.IncidentCameraPath(ctx, incs[0].ID, sorter.ChannelC1)
	assert.ErrorIs(t, err, sorter.ErrIncidentImageNotFound)

	// Clearing: the incident goes, with its cameras (only one being held is kept).
	ids, err := store.DeleteIncidentsExcept(ctx, 0)
	require.NoError(t, err)
	assert.Equal(t, []int64{incs[0].ID}, ids)
	incs, err = store.ListIncidents(ctx, 5)
	require.NoError(t, err)
	assert.Empty(t, incs)
}

func TestSortLandingHeldIsFreedByShaking(t *testing.T) {
	held := sorter.SimCatalog[0]
	held.HeldOn, held.HeldShakes = sorter.ChannelC3, 1 // one shake of C3 frees it
	shakeFirst := func() sorter.MachineSettings {
		s := fastSettings()
		s.Feeder.ShakeBeforePush = true
		return s
	}
	m, world, _ := startSimSort(t, shakeFirst, []sorter.SimPart{held}, nil)
	require.Eventually(t, func() bool { return len(world.Falls()) == 1 && len(m.Snapshot().Aboard) == 0 }, 60*time.Second, 20*time.Millisecond)
	assert.NotZero(t, world.Falls()[0].Layer, "sorted into its bin after all")
	assert.Nil(t, m.Snapshot().Incident)
	require.NotNil(t, m.Snapshot().LastJob)
	assert.Contains(t, m.Snapshot().LastJob.Name, "held where pieces land")
}

func TestSortLandingStuckRaisesAnIncident(t *testing.T) {
	held := sorter.SimCatalog[0]
	held.HeldOn, held.HeldShakes = sorter.ChannelC3, 1_000_000
	stopForIt := func() sorter.MachineSettings {
		s := fastSettings()
		s.Feeder.WorkAroundStuck = false
		return s
	}
	m, _, _ := startSimSort(t, stopForIt, []sorter.SimPart{held}, nil)
	require.Eventually(t, func() bool { return m.Snapshot().Incident != nil }, 60*time.Second, 20*time.Millisecond)
	assert.Equal(t, sorter.IncidentLandingStuck, m.Snapshot().Incident.Kind)
	require.NoError(t, m.ResolveIncident(context.Background(), sorter.ResolveDone))
	assert.Nil(t, m.Snapshot().Incident)
}

// A piece stuck on C3 for good is worked around: its spot is left out of C3's camera,
// the sort never stops, and the pieces after it are still sorted.
func TestSortWorksAroundAStuckPiece(t *testing.T) {
	held := sorter.SimCatalog[0]
	held.HeldOn, held.HeldShakes = sorter.ChannelC3, 1_000_000
	m, world, store := startSimSort(t, fastSettings, []sorter.SimPart{held, sorter.SimCatalog[1], sorter.SimCatalog[2]}, nil)

	require.Eventually(t, func() bool { return len(world.Falls()) == 2 }, 60*time.Second, 20*time.Millisecond,
		"the two pieces after the stuck one are sorted")
	require.NotNil(t, m.Snapshot().LastJob)
	assert.Contains(t, m.Snapshot().LastJob.Name, "push a stuck piece past", "pushed straight away, no shaking first")

	// Each channel says what it's doing while the run is on.
	doing := m.Snapshot().Doing
	for _, ch := range []sorter.Channel{sorter.ChannelC1, sorter.ChannelC2, sorter.ChannelC3, sorter.ChannelC4} {
		assert.NotEmpty(t, doing[ch], "%s says what it's doing", ch)
	}

	// Where the time went: the push shows as stuck time, and the channels moved.
	spent := m.Snapshot().TimeMs
	assert.Positive(t, spent[sorter.ChannelC3][sorter.TimeStuck])
	assert.Positive(t, spent[sorter.ChannelC3][sorter.TimeMoving])
	assert.Positive(t, spent[sorter.ChannelC4][sorter.TimeMoving])
	assert.Nil(t, m.Snapshot().Incident, "nothing stopped the sort")
	assert.Equal(t, sorter.StateRunning, m.Snapshot().State)
	assert.Equal(t, 1, world.Stuck(sorter.ChannelC3), "the stuck one is still there")
	assert.Equal(t, 1, m.Snapshot().Channels[sorter.ChannelC3].Ignored)

	incs, err := store.ListIncidents(context.Background(), 5)
	require.NoError(t, err)
	require.Len(t, incs, 1)
	assert.Equal(t, sorter.IncidentLandingWorkedAround, incs[0].Kind)
	assert.Equal(t, sorter.ResolveWorkedAround, incs[0].Resolution)
	require.Eventually(t, func() bool {
		incs, err = store.ListIncidents(context.Background(), 5)
		return err == nil && len(incs[0].Cameras) == 4
	}, 5*time.Second, 10*time.Millisecond)
	for _, cam := range incs[0].Cameras {
		if cam.Channel == sorter.ChannelC3 {
			assert.Len(t, cam.State.Focus, 1, "a red box on the stuck piece")
		} else {
			assert.Empty(t, cam.State.Focus, "%s: nothing wrong there", cam.Channel)
		}
	}

	// Taken off by hand and forgotten: C3 is watched in full again.
	require.NoError(t, m.ForgetIgnored(context.Background(), sorter.ChannelC3))
	assert.Equal(t, 0, len(world.Observation(sorter.ChannelC3).Ignored))
}

func TestSortDidntFallIsShakenOff(t *testing.T) {
	clingy := sorter.SimCatalog[2]
	clingy.NoFall = true
	m, world, store := startSimSort(t, fastSettings, []sorter.SimPart{clingy}, nil)
	require.Eventually(t, func() bool { return len(world.Falls()) == 1 }, 20*time.Second, 10*time.Millisecond, "shaken off in the end")
	require.Eventually(t, func() bool {
		recent := m.Snapshot().Recent
		return len(recent) == 1 && len(recent[0].Problems) > 0
	}, 5*time.Second, 10*time.Millisecond)
	assert.Equal(t, sorter.ProblemDidntFall, m.Snapshot().Recent[0].Problems[0].Kind)
	assert.NotZero(t, world.Falls()[0].Layer, "still into its bin: the doors stay set")
	_ = store
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

const kitProfile = `{
  "name": "Order 42", "schema_version": 1, "profile_type": "set",
  "categories": {"kit": {"name": "Order 42"}, "r-bricks": {"name": "Bricks"}},
  "program": {"rules": [
    {"category": "kit", "kit": {"3001": ["5"]}},
    {"category": "r-bricks", "parts": ["3001"], "colors": null}
  ], "default": "misc"},
  "set_inventories": {"kit": {"name": "Order 42", "parts": [{"part_num": "3001", "color_id": 5, "quantity": 1}]}}
}`

func TestSortingKitsAndFullBins(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	settings := func() sorter.MachineSettings {
		s := testSettings()
		s.Distributor.Layers[0].MaxPieces = 2
		return s
	}
	records := NewRecordsStore(db)
	svc := sorter.NewSortingService(ctx, NewSortingStore(db), settings)
	require.NoError(t, svc.AttachCounts(ctx, records))
	p, err := svc.Import(ctx, []byte(kitProfile))
	require.NoError(t, err)
	require.NoError(t, svc.Activate(ctx, p.ID, sorter.ActivateKeep))
	runID, err := records.StartRun(ctx, &p.ID, p.Name, time.Now())
	require.NoError(t, err)

	// deliver routes a red 3001 and records it as delivered where it was sent.
	deliver := func() sorter.RouteResult {
		t.Helper()
		res, err := svc.Route(ctx, "3001", "5", 0, true)
		require.NoError(t, err)
		now := time.Now().UTC()
		piece := sorter.Piece{ID: sorter.NewPieceID(), RunID: runID, Status: sorter.PieceDistributed, SeenAt: now,
			DistributedAt: &now, PartID: "3001", ColorID: "5", Category: res.Category}
		if res.Choice.Bin != nil {
			piece.BinLayer, piece.BinNumber = res.Choice.Bin.Layer, res.Choice.Bin.Bin
			svc.NoteDelivered(sorter.BinKey{Layer: piece.BinLayer, Bin: piece.BinNumber})
		}
		require.NoError(t, records.SavePiece(ctx, piece))
		return res
	}

	first := deliver()
	assert.Equal(t, "kit", first.Category, "the kit needs one red brick")
	assert.Equal(t, []sorter.KitProgress{{Category: "kit", Name: "Order 42", Need: 1, Found: 1}}, svc.KitProgress())
	second := deliver()
	assert.Equal(t, "r-bricks", second.Category, "the kit is full, so the next rule")
	third := deliver()
	assert.Equal(t, second.Choice.Bin.Bin, third.Choice.Bin.Bin)
	fourth := deliver()
	assert.NotEqual(t, second.Choice.Bin.Bin, fourth.Choice.Bin.Bin, "bricks' bin is full (2), so a new one")

	view := svc.Bins()
	brickBin := view.Layers[0].Sections[0].Bins[second.Choice.Bin.Bin-1]
	assert.Equal(t, 2, brickBin.Count)
	assert.True(t, brickBin.Full)

	// A restart counts the bins and the kit up again from the records.
	svc2 := sorter.NewSortingService(ctx, NewSortingStore(db), settings)
	require.NoError(t, svc2.AttachCounts(ctx, records))
	assert.Equal(t, 1, svc2.KitProgress()[0].Found)
	res, err := svc2.Route(ctx, "3001", "5", 0, false)
	require.NoError(t, err)
	assert.Equal(t, "r-bricks", res.Category)
	assert.Equal(t, fourth.Choice.Bin.Bin, res.Choice.Bin.Bin, "still skipping the full bin")

	// Emptying the bin lets it take pieces again.
	require.NoError(t, svc2.EmptyBin(ctx, sorter.BinKey{Layer: 1, Bin: second.Choice.Bin.Bin}))
	assert.Equal(t, 0, svc2.Bins().Layers[0].Sections[0].Bins[second.Choice.Bin.Bin-1].Count)
	time.Sleep(10 * time.Millisecond)
	svc3 := sorter.NewSortingService(ctx, NewSortingStore(db), settings)
	require.NoError(t, svc3.AttachCounts(ctx, records))
	assert.Equal(t, 0, svc3.Bins().Layers[0].Sections[0].Bins[second.Choice.Bin.Bin-1].Count, "emptied stays emptied")
}
