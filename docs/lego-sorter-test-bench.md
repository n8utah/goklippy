# LEGO sorter test bench

Branch `lego-sorter`. A page for running and measuring a [Sorter V2](https://docs.basically.website)
style C-channel feed from a RAMPS 1.4 board instead of the project's custom electronics.

## Default motor mapping

| Channel | Role | RAMPS socket | Motion |
|---|---|---|---|
| C1 | Feeder, slowest | X (`stepper_x`) | Continuous |
| C2 | Feeder, faster than C1 | Y (`stepper_y`) | Continuous |
| C3 | Feeder, faster than C2 | Z (`stepper_z`) | Continuous |
| C4 | Classification carousel | E0 (`stepper_e0`) | 72° index moves, 5 slots |
| Distributor | Rotates the chute over the bins | E1 (`stepper_e1`) | Shortest-path moves to a bin |

Each distribution layer has its own servo. Selecting a layer opens its servo and closes the others.
Layer count, servo assignment, bin count (18 or 12) and open/closed angles are page settings. There
is no homing yet: "Set position as bin 1" defines the reference, and it is lost on a reset.

The mapping, gear ratios, speeds and index settings are per-browser settings on the page.

## Units

Everything on the page is in rotor terms: rpm, degrees, slots and bins. Each motor's **Gearing**
block is the one place that ties those to the motor:

- Enter motor turns per rotor turn, or the motor and rotor gear tooth counts.
- Microsteps come from the board config (16 is assumed if the motor isn't in it).
- The block shows the result as steps per rotor rev, and per slot or bin.
- **Calibrate** turns what the page thinks is one rotor rev. Enter how far the rotor really turned
  and the ratio is corrected (360° means it was already right).

C1–C3 speeds are set as plain rpm. The page still flags them if they don't increase from C1 to C3.

## Routines

The page keeps its own routines (`goklippy_sorter_routines`), made of moves in rotor degrees and
rpm. They repeat in order until stopped. A move's "forward" is the channel's forward direction,
so the same routine is the same physical motion on any channel. The first time the page opens it
copies the Steppers-page routines, reading their steps as a bare 1:1 motor at 16 microsteps, so a
routine like jitter keeps the motion it was tuned with. Later edits on either page don't affect the
other.

## What the numbers mean

- All readings come from `stepper_get_position`, sampled every 50 ms and sent to the UI five
  times a second over the session WebSocket (`type: "motion"`).
- **Jitter** on this page is a motion, not a measurement: any routine can be chosen for a channel.
  Routines are optional and every channel starts with none.
- The steppers are open-loop. These figures show what the MCU issued; skipped steps on the shaft
  are not visible.

## Open questions

- The Sorter V2 docs give no gear ratios or stage speeds, so the feeder defaults (1:1, C1 5 rpm, C2
  7.5 rpm, C3 11.25 rpm) are placeholders to tune on the machine. C4's 10.8 gear ratio was set on
  the machine so one forward move lands on the next slot.
- Moves have no acceleration ramp. If the C4 carousel skips, indexing needs one.
