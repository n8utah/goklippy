package mcu

import (
	"testing"

	"goklippy/backend/klipperconfig"
	"goklippy/backend/mcu/cfgtranslate"
)

func TestStopTrsyncPerStepper(t *testing.T) {
	f, err := klipperconfig.LoadFile("../testdata/ramps-all-steppers.cfg")
	if err != nil {
		t.Fatal(err)
	}
	build, err := cfgtranslate.Build(f)
	if err != nil {
		t.Fatal(err)
	}

	nextOID := cfgtranslate.NextOID(build.Commands)
	stopTrsyncOIDs := make(map[int]int)
	for _, obj := range build.Objects {
		if obj.Kind != cfgtranslate.KindStepper {
			continue
		}
		stopTrsyncOIDs[obj.OID] = nextOID
		nextOID++
	}

	if len(stopTrsyncOIDs) != 5 {
		t.Fatalf("expected 5 stepper stop trsyncs, got %d", len(stopTrsyncOIDs))
	}

	seen := make(map[int]int)
	for stepperOID, trsyncOID := range stopTrsyncOIDs {
		if trsyncOID < cfgtranslate.NextOID(build.Commands) {
			t.Fatalf("stepper %d trsync oid %d should be >= next config oid", stepperOID, trsyncOID)
		}
		if prev, ok := seen[trsyncOID]; ok {
			t.Fatalf("duplicate trsync oid %d for steppers %d and %d", trsyncOID, prev, stepperOID)
		}
		seen[trsyncOID] = stepperOID
	}
}
