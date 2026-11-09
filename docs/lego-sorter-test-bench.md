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

## What the numbers mean

- All readings come from `stepper_get_position`, sampled every 50 ms and sent to the UI five
  times a second over the session WebSocket (`type: "motion"`).
- **Jitter** on this page is a motion, not a measurement: any saved stepper routine can be run on a
  channel, and one named "jitter" is selected by default.
- The steppers are open-loop. These figures show what the MCU issued; skipped steps on the shaft
  are not visible.

## Open questions

- The Sorter V2 docs give no gear ratios or stage speeds, so the defaults (1:1, C1 at 5 rpm, each
  stage 1.5× the last) are placeholders to tune on the machine.
- Moves have no acceleration ramp. If the C4 carousel skips, indexing needs one.
