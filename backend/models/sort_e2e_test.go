package models

import (
	"context"
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
}

func TestSortLandingHeldIsFreedByShaking(t *testing.T) {
	held := sorter.SimCatalog[0]
	held.HeldOn, held.HeldShakes = sorter.ChannelC3, 1 // one shake of C3 frees it
	m, world, _ := startSimSort(t, fastSettings, []sorter.SimPart{held}, nil)
	require.Eventually(t, func() bool { return len(world.Falls()) == 1 && len(m.Snapshot().Aboard) == 0 }, 60*time.Second, 20*time.Millisecond)
	assert.NotZero(t, world.Falls()[0].Layer, "sorted into its bin after all")
	assert.Nil(t, m.Snapshot().Incident)
	require.NotNil(t, m.Snapshot().LastJob)
	assert.Contains(t, m.Snapshot().LastJob.Name, "held where pieces land")
}

func TestSortLandingStuckRaisesAnIncident(t *testing.T) {
	held := sorter.SimCatalog[0]
	held.HeldOn, held.HeldShakes = sorter.ChannelC3, 1_000_000
	m, _, _ := startSimSort(t, fastSettings, []sorter.SimPart{held}, nil)
	require.Eventually(t, func() bool { return m.Snapshot().Incident != nil }, 60*time.Second, 20*time.Millisecond)
	assert.Equal(t, sorter.IncidentLandingStuck, m.Snapshot().Incident.Kind)
	require.NoError(t, m.ResolveIncident(context.Background(), sorter.ResolveDone))
	assert.Nil(t, m.Snapshot().Incident)
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
