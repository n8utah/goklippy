package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"goklippy/backend/config"
	"goklippy/backend/logic/brickognize"
	"goklippy/backend/logic/sorter"
	"goklippy/backend/logic/vision"
	"goklippy/backend/mcu"
	"goklippy/backend/models"
	"goklippy/backend/routes"

	"github.com/jmoiron/sqlx"
)

// startSorter builds the sorter (settings, hardware, profiles, records, the stations)
// and starts its loops. With simulate, a simulated machine and pretend LEGO stand in
// for the board, the cameras and Brickognize.
func startSorter(ctx context.Context, cfg config.SorterConfig, db *sqlx.DB) (*sorter.Machine, error) {
	settings := sorter.NewSettingsService(ctx, models.NewSorterSettingsStore(db))
	machineSettings := settings.Machine
	if cfg.Simulate {
		machineSettings = simSettings(settings.Machine)
	}
	sorting := sorter.NewSortingService(ctx, models.NewSortingStore(db), machineSettings)

	dataDir := filepath.Clean(cfg.DataDir)
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	records := sorter.NewRecords(models.NewRecordsStore(db), dataDir)
	go records.Run(ctx)

	var hw sorter.Hardware
	deps := sorter.SortDeps{Sorting: sorting, Records: records}
	var identifier sorter.Identifier = brickognize.NewClient()
	if cfg.Simulate {
		sim := sorter.NewSimHardware(machineSettings)
		world := sorter.NewSimWorld(sim, machineSettings, nil)
		world.Random = true
		hw, deps.Vision, deps.Identifier = sim, world, world
		slog.Info("sorter running on simulated hardware with pretend LEGO")
	} else {
		hw = sorter.NewMCUHardware(mcu.DefaultManager(), settings.Machine)
		// Cameras come later (phase 2); until then the stations see no frames.
		deps.Vision, deps.Identifier = sorter.NoVision{}, identifier
	}

	machine := sorter.NewMachine(hw, machineSettings, 0)
	machine.AttachSorting(deps)
	go machine.Run(ctx)

	routes.InitSorter(machine, settings)
	routes.InitSorting(sorting, identifier)
	routes.InitRecords(records)
	slog.Info("sorter data folder", "path", dataDir)
	return machine, nil
}

// simSettings are the saved settings with the simulated world's zones on every
// camera channel.
func simSettings(machine func() sorter.MachineSettings) func() sorter.MachineSettings {
	return func() sorter.MachineSettings {
		s := machine()
		zones := sorter.SimZones()
		s.Cameras.Zones = map[string]vision.ChannelZones{"c2": zones, "c3": zones, "c4": zones}
		return s
	}
}
