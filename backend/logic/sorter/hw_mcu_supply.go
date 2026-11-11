package sorter

import (
	"fmt"
	"math"
	"time"

	"goklippy/backend/mcu/cfgtranslate"
)

// supplyStale: a reading older than this (the MCU reports twice a second) means the
// sensor has gone quiet.
const supplyStale = 3 * time.Second

func (h *MCUHardware) SupplyVoltage() SupplyStatus {
	cfg := h.settings().Supply
	st := SupplyStatus{MinVolts: cfg.MinVolts}
	if cfg.Sensor == "" {
		st.Message = "not measured: no supply sensor is set"
		return st
	}
	s, err := h.session()
	if err != nil {
		st.Message = err.Error()
		return st
	}
	obj := objectBySection(s, cfg.Sensor)
	if obj == nil || obj.Kind != cfgtranslate.KindAnalogIn {
		st.Message = fmt.Sprintf("not measured: add [%s] with its sensor_pin to the board config once the divider is wired", cfg.Sensor)
		return st
	}
	fraction, at, ok := s.AnalogReading(obj.OID)
	if !ok || time.Since(at) > supplyStale {
		st.Message = "no reading from the supply sensor yet"
		return st
	}
	st.Known = true
	st.Volts = math.Round(cfg.Volts(fraction)*100) / 100
	st.Low = cfg.MinVolts > 0 && st.Volts < cfg.MinVolts
	return st
}

// SupplyVoltage in simulation: a steady 12 V, unless SetSupply says otherwise.
func (h *SimHardware) SupplyVoltage() SupplyStatus {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.connected {
		return SupplyStatus{Message: ErrNoBoard.Error()}
	}
	v := 12.0
	if h.supply != nil {
		v = *h.supply
	}
	min := h.settings().Supply.MinVolts
	return SupplyStatus{Known: true, Volts: v, MinVolts: min, Low: min > 0 && v < min}
}

// SetSupply simulates the motors' supply voltage.
func (h *SimHardware) SetSupply(volts float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.supply = &volts
}
