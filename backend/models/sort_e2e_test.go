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

func TestSortStallRaisesAnIncident(t *testing.T) {
	settings := func() sorter.MachineSettings {
		s := fastSettings()
		s.Platter.IdentifyTimeoutMs = 60_000 // Brickognize hangs, and we wait for it
		s.Platter.StallMs = 300
		return s
	}
	m, _, store := startSimSort(t, settings, []sorter.SimPart{sorter.SimCatalog[0]}, failingIdentifier{block: true})
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

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
