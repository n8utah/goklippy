package cfgtranslate

import (
	"testing"

	"goklippy/backend/klipperconfig"
	"goklippy/backend/profiles"
)

func TestSupplementSteppersFromProfile(t *testing.T) {
	f, err := klipperconfig.Parse(`[stepper_x]
step_pin: ar54
dir_pin: ar55
enable_pin: !ar38
`)
	if err != nil {
		t.Fatal(err)
	}

	profile := profiles.Profile{
		Slots: map[string]profiles.Slot{
			"stepper_x": {Type: "stepper", Pins: map[string]string{"step": "ar54", "dir": "ar55", "enable": "!ar38"}},
			"stepper_y": {Type: "stepper", Pins: map[string]string{"step": "ar60", "dir": "ar61", "enable": "!ar56"}},
			"stepper_z": {Type: "stepper", Pins: map[string]string{"step": "ar46", "dir": "ar48", "enable": "!ar62"}},
		},
	}

	SupplementSteppersFromProfile(f, profile)

	for _, name := range []string{"stepper_x", "stepper_y", "stepper_z"} {
		sec, ok := f.Sections[name]
		if !ok {
			t.Fatalf("missing section %s", name)
		}
		if sec.Options["step_pin"] == "" || sec.Options["dir_pin"] == "" {
			t.Fatalf("section %s missing pins: %+v", name, sec.Options)
		}
	}

	// Existing X section keeps original enable pin formatting.
	if f.Sections["stepper_x"].Options["enable_pin"] != "!ar38" {
		t.Fatalf("stepper_x enable_pin = %q", f.Sections["stepper_x"].Options["enable_pin"])
	}
	if f.Sections["stepper_y"].Options["step_pin"] != "ar60" {
		t.Fatalf("stepper_y step_pin = %q", f.Sections["stepper_y"].Options["step_pin"])
	}

	build, err := Build(f)
	if err != nil {
		t.Fatal(err)
	}

	stepperCount := 0
	for _, obj := range build.Objects {
		if obj.Kind == KindStepper {
			stepperCount++
		}
	}
	if stepperCount != 3 {
		t.Fatalf("expected 3 steppers, got %d", stepperCount)
	}

	MapObjectsToSlots(build.Objects, profile, f)
	for _, slotID := range []string{"stepper_x", "stepper_y", "stepper_z"} {
		found := false
		for _, obj := range build.Objects {
			if obj.Kind == KindStepper && obj.SlotID == slotID {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("no stepper mapped to slot %s", slotID)
		}
	}
}
