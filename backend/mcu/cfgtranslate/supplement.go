package cfgtranslate

import (
	"goklippy/backend/klipperconfig"
	"goklippy/backend/profiles"
)

// SupplementSteppersFromProfile adds missing [stepper_*] sections from the board
// profile so every driver slot on the diagram can be controlled live.
func SupplementSteppersFromProfile(f *klipperconfig.File, profile profiles.Profile) {
	if f == nil {
		return
	}
	for slotID, slot := range profile.Slots {
		if slot.Type != "stepper" {
			continue
		}
		if sec, ok := f.Sections[slotID]; ok && sec.Options["step_pin"] != "" {
			continue
		}
		stepPin := slot.Pins["step"]
		dirPin := slot.Pins["dir"]
		if stepPin == "" || dirPin == "" {
			continue
		}
		f.SetOption(slotID, "step_pin", stepPin)
		f.SetOption(slotID, "dir_pin", dirPin)
		if enablePin := slot.Pins["enable"]; enablePin != "" {
			f.SetOption(slotID, "enable_pin", enablePin)
		}
		f.SetOption(slotID, "microsteps", "16")
		f.SetOption(slotID, "rotation_distance", "40")
	}
}
