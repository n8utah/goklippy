package cfgtranslate

import (
	"strings"

	"goklippy/backend/klipperconfig"
	"goklippy/backend/profiles"
)

// MapObjectsToSlots fills SlotID on objects by matching config pins to profile slots.
func MapObjectsToSlots(objects []ObjectSpec, profile profiles.Profile, f *klipperconfig.File) {
	if f == nil {
		return
	}
	for i := range objects {
		if objects[i].SlotID != "" {
			continue
		}
		if slot, ok := profile.Slots[objects[i].Section]; ok {
			if slotTypeKind(slot.Type) == objects[i].Kind || compatibleKind(slot.Type, objects[i].Kind) {
				objects[i].SlotID = objects[i].Section
				continue
			}
		}
		sec, ok := f.Sections[objects[i].Section]
		if !ok {
			continue
		}
		objects[i].SlotID = matchSlot(profile.Slots, objects[i].Kind, sec.Options)
	}
}

func matchSlot(slots map[string]profiles.Slot, kind ObjectKind, opts map[string]string) string {
	for slotID, slot := range slots {
		if slotTypeKind(slot.Type) != kind && !compatibleKind(slot.Type, kind) {
			continue
		}
		if pinsMatch(slot, kind, opts) {
			return slotID
		}
	}
	return ""
}

func slotTypeKind(slotType string) ObjectKind {
	switch slotType {
	case "stepper":
		return KindStepper
	case "thermistor":
		return KindAnalogIn
	case "fan", "heater", "relay":
		return KindDigitalOut
	case "servo":
		return KindPWMOut
	case "endstop":
		return KindEndstop
	default:
		return ObjectKind(slotType)
	}
}

func compatibleKind(slotType string, kind ObjectKind) bool {
	if slotType == "heater" && kind == KindPWMOut {
		return true
	}
	if slotType == "fan" && kind == KindDigitalOut {
		return true
	}
	return false
}

func pinsMatch(slot profiles.Slot, kind ObjectKind, opts map[string]string) bool {
	switch kind {
	case KindStepper:
		return normalizePin(slot.Pins["step"]) == normalizePin(opts["step_pin"])
	case KindAnalogIn:
		return normalizePin(slot.Pins["sensor"]) == normalizePin(opts["sensor_pin"])
	case KindDigitalOut, KindPWMOut:
		cfgPin := opts["pin"]
		if cfgPin == "" {
			cfgPin = opts["heater_pin"]
		}
		slotPin := slot.Pins["pin"]
		return normalizePin(slotPin) == normalizePin(cfgPin)
	case KindEndstop:
		return normalizePin(slot.Pins["pin"]) == normalizePin(opts["pin"])
	default:
		return false
	}
}

func normalizePin(p string) string {
	p = strings.TrimSpace(p)
	for len(p) > 0 && (p[0] == '!' || p[0] == '^') {
		p = p[1:]
	}
	return p
}
