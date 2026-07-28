package cfgtranslate

// ObjectKind identifies a configured MCU object type.
type ObjectKind string

const (
	KindStepper    ObjectKind = "stepper"
	KindAnalogIn   ObjectKind = "analog_in"
	KindDigitalOut ObjectKind = "digital_out"
	KindPWMOut     ObjectKind = "pwm_out"
	KindEndstop    ObjectKind = "endstop"
)

// ObjectSpec describes one OID allocated during config translation.
type ObjectSpec struct {
	Section string            `json:"section"`
	OID     int               `json:"oid"`
	Kind    ObjectKind        `json:"kind"`
	SlotID  string            `json:"slot_id,omitempty"`
	Label   string            `json:"label,omitempty"`
	Meta    map[string]string `json:"meta,omitempty"`
}

// BuildResult holds MCU config commands and the object registry.
type BuildResult struct {
	Commands     []string
	InitCommands []string
	Objects      []ObjectSpec
}
