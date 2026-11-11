# LEGO sorter test bench

Branch `lego-sorter`. A page for running and measuring a [Sorter V2](https://docs.basically.website)
style C-channel feed from a RAMPS 1.4 board instead of the project's custom electronics.

The autonomous sorting loop built around this page is planned in
[lego-sorter-server-plan.md](./lego-sorter-server-plan.md).

## Default motor mapping

| Channel | Role | RAMPS socket | Motion |
|---|---|---|---|
| C1 | Feeder, slowest | X (`stepper_x`) | Continuous |
| C2 | Feeder, faster than C1 | Y (`stepper_y`) | Continuous |
| C3 | Feeder, faster than C2 | Z (`stepper_z`) | Continuous |
| C4 | Classification carousel | E0 (`stepper_e0`) | 72° index moves, 5 slots |
| Distributor | Rotates the chute over the bins | E1 (`stepper_e1`) | Homed moves to a bin, clockwise from home |

Each distribution layer has its own servo. Selecting a layer opens its servo and closes the others.
Layer count, servo assignment, bin count (18 or 12) and open/closed angles are page settings.

## The chute (Chute page)

The chute is laid out like Sorter V2's tower: each layer is 6 sections, with 3 bins per section on
an 18-bin layer and 2 on a 12-bin layer. Bins are numbered per layer from 1, clockwise from home.
The **Chute** page homes, calibrates and aims it, through the backend:

- **Home** turns the chute counter-clockwise until its home switch fires; that is 0°. It then
  closes every layer and parks at bin 1. From home the chute only ever turns clockwise, up to the
  **travel limit** (350°), so it never crosses the switch. Bins past the limit are shown red and
  are never used. If the switch already reads triggered, the chute first turns 20° off it.
- **Set 0° here** uses the current position instead of the switch, for working without one.
- **Calibrate the aim**: jog to the middle of the first bin of a section and capture it, then the
  last bin of the same section, then **Work it out** and **Use these**. That sets the section width
  and where the first section starts, and every bin on every layer follows from them.
- Clicking a bin selects its layer and aims the chute at it. **Test every bin** visits each
  reachable bin on a layer in turn.

The chute's angle is read from the MCU's step count from home, so jogging the distributor
elsewhere doesn't lose it. A board reset or reconnect does, and the chute must be homed again.

### Wiring the home switch

Plug the switch into a free RAMPS endstop socket and add its pin to the distributor's stepper
section on Board Setup (or in the config). For example, X-min is `ar3`:

```
[stepper_e1]
endstop_pin: ^ar3
```

`^` turns on the pull-up. Add `!` (`^!ar3`) if the switch reads triggered when it isn't pressed.
The other sockets are X-max `ar2`, Y-min `ar14`, Y-max `ar15`, Z-min `ar18` and Z-max `ar19`. The
endstop shows up as `stepper_e1_endstop`, the Chute page's default **Home switch**. The Chute page
shows whether it reads Open or Triggered, which is the quickest wiring check.

If homing turns the chute clockwise, untick **Motor forward turns the chute clockwise**.

## The light (D10)

The LED light runs off the RAMPS D10 MOSFET (the hotend heater output, 12 V switched on the
negative side). Add it to the board config as an on/off output:

```
[output_pin leds]
pin: ar10
value: 0
shutdown_value: 0
```

`value` is whether it's on once the board is configured, and `shutdown_value` is its state after
an MCU shutdown (both 0 or 1; on/off only, no PWM). Add `!` (`!ar10`) if the light is on when it
should be off. Don't also configure an `[extruder]` on D10: two outputs on one pin is a config
error. Board Setup shows the section on the D10 slot with on/off buttons, which is the quickest
wiring check.

The **Light** button on the Dashboard switches it, at any time, sorting included. Starting or
resuming a sort turns it on if **Turn on when sorting starts** is ticked on the Tuning page (the
default). The Tuning page also holds the section name, `output_pin leds` by default. A board reset
puts the light back to `value`.

The mapping, gear ratios, speeds and index settings are saved on the server
(`sorter_settings`, document `machine`; routines are document `routines`), so every browser and
the sorter's control loop see the same values. The first time a browser opens the page against an
empty server, its older per-browser settings are uploaded. If the server can't be reached, the page
says so and keeps changes in that browser only, without overwriting the server later.

C4 indexing has an **Accel** setting (rpm per second, default 60; 0 turns the ramp off). Each index
move speeds up and slows down on a trapezoid instead of starting and stopping at full speed.

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
- C4 indexing ramps (see Accel above). Feeder runs and the distributor's bin moves still start at
  full speed.
