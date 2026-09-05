package cfgtranslate

import (
	"fmt"
	"testing"

	"goklippy/backend/klipperconfig"
)

func TestDumpStepperXConfig(t *testing.T) {
	f, err := klipperconfig.LoadFile("../../testdata/stepper-x-only.cfg")
	if err != nil {
		t.Fatal(err)
	}
	br, err := Build(f)
	if err != nil {
		t.Fatal(err)
	}
	cmds := append([]string{}, br.Commands...)
	nextOID := NextOID(cmds)
	for _, obj := range br.Objects {
		if obj.Kind == KindStepper {
			cmds = append(cmds, fmt.Sprintf("config_trsync oid=%d", nextOID))
			nextOID++
		}
	}
	for i, l := range FinalizeCommands(cmds) {
		t.Logf("%2d: %s", i+1, l)
	}
	t.Log("--- init ---")
	for _, l := range br.InitCommands {
		t.Log(l)
	}
}
