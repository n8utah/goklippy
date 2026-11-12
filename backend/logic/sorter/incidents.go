package sorter

import "errors"

// Incident kinds. Two sorts of problem:
//
//   - ones the sort stops for until the operator deals with them: the board lost,
//     the motors' supply low, the chute losing its home (stopForError), and a stalled
//     classification channel that turning it round didn't clear (IncidentPlatterStuck),
//     since pieces on C4 can't be told apart without it moving them on;
//   - ones it works around and only notes: a piece stuck where pieces land on C2 or C3
//     that shaking didn't free is left out of what that camera reports
//     (IncidentLandingWorkedAround, ignored_spots.go) while everything else carries on;
//     a piece that didn't fall off C4 is shaken off; a failed identification goes to
//     the bucket. With Feeder.WorkAroundStuck off, a stuck piece stops the sort
//     (IncidentLandingStuck).
const (
	IncidentPlatterStuck        = "platter_stuck"
	IncidentBoardLost           = "board_lost"
	IncidentLandingStuck        = "landing_stuck"
	IncidentLandingWorkedAround = "landing_worked_around"
)

// Ways an incident is resolved.
const (
	ResolveDone         = "done"          // the operator dealt with it by hand
	ResolveWorkedAround = "worked_around" // the machine carried on without it
)

var ErrNoIncident = errors.New("there's no open incident")

// IncidentHold is what the machine keeps about the open incident.
type IncidentHold struct {
	Incident
	Actions []string `json:"actions"`
}
