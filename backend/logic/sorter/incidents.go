package sorter

import "errors"

// Incident kinds: problems the sort stops for until the operator deals with them.
const (
	IncidentPlatterStuck = "platter_stuck"
	IncidentBoardLost    = "board_lost"
	IncidentLandingStuck = "landing_stuck"
)

// Ways an operator resolves an incident.
const (
	ResolveDone = "done" // dealt with it by hand
)

var ErrNoIncident = errors.New("there's no open incident")

// IncidentHold is what the machine keeps about the open incident.
type IncidentHold struct {
	Incident
	Actions []string `json:"actions"`
}
