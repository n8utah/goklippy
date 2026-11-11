package mcu

import (
	"sync"
	"time"
)

// The MCU reports each analog input on its own schedule (twice a second). The session
// keeps the newest of each, whoever happened to read it off the line, so a reader such
// as the supply voltage needn't poll the serial port itself.

type analogLatest struct {
	mu     sync.Mutex
	values map[int]analogValue
}

type analogValue struct {
	fraction float64 // of the ADC's range, 0–1
	at       time.Time
}

// noteMessage is the connection's message hook. It runs with the connection locked,
// so it touches only its own state.
func (s *Session) noteMessage(params map[string]interface{}) {
	if name, _ := params["#name"].(string); name != "analog_in_state" {
		return
	}
	oid, ok := params["oid"].(int64)
	if !ok {
		return
	}
	v := extractAnalogValue(params)
	s.analog.mu.Lock()
	defer s.analog.mu.Unlock()
	if s.analog.values == nil {
		s.analog.values = make(map[int]analogValue)
	}
	s.analog.values[int(oid)] = analogValue{fraction: v, at: time.Now()}
}

// AnalogReading is an analog input's newest reading as a share of the ADC's range
// (0–1), when it came, and false if none has.
func (s *Session) AnalogReading(oid int) (float64, time.Time, bool) {
	s.analog.mu.Lock()
	defer s.analog.mu.Unlock()
	v, ok := s.analog.values[oid]
	return v.fraction, v.at, ok
}
