package pins

import "testing"

func TestUpdateCommandStepperPins(t *testing.T) {
	r := NewArduinoMegaResolver()
	cmd := "config_stepper oid=0 step_pin=ar54 dir_pin=ar55 invert_step=-1 step_pulse_ticks=0"
	want := "config_stepper oid=0 step_pin=PF0 dir_pin=PF1 invert_step=-1 step_pulse_ticks=0"
	if got := r.UpdateCommand(cmd); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestUpdateCommandAnalogPin(t *testing.T) {
	r := NewArduinoMegaResolver()
	cmd := "config_analog_in oid=1 pin=analog13"
	want := "config_analog_in oid=1 pin=PK5"
	if got := r.UpdateCommand(cmd); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestResolvePinWithInvert(t *testing.T) {
	r := NewArduinoMegaResolver()
	if got := r.ResolvePin("!ar38"); got != "!PD7" {
		t.Fatalf("got %q want !PD7", got)
	}
}
