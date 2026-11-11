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
should be off.

Board Setup treats the MOSFET outputs (D8, D9, D10) as plain on/off IO: **Mark connected** writes
an `[output_pin <slot>]` (keeping the name of one already on that pin, such as `leds`) and removes
any old `[extruder]`, `[heater_bed]` or `[fan]` on the same pin, since two outputs on one pin is a
config error. The slot then shows on/off buttons, which is the quickest wiring check.

For safety the light starts off (`value: 0`) and goes off whenever the machine stops: on Stop,
when an error stops it, when the backend exits, and on an MCU shutdown (`shutdown_value: 0`). A
board reset also puts it back to `value`.

`maximum_mcu_duration: 5` covers a backend that dies without cleaning up (killed, crashed, USB
cable out). While the light is on, the backend resends its state every third of that time; if
the board hears nothing for 5 s it shuts down, which turns the light off and stops every motor.
Reconnect or reset the board on Board Setup to clear it. The option takes 0–5 s (Klipper's
limit), and `value` must equal `shutdown_value` when it's set. Board Setup asks for it as
**Safety timeout** on each MOSFET output, 5 s by default.

The **Light** button on the Dashboard switches it, at any time, sorting included. Starting or
resuming a sort turns it on if **Turn on when sorting starts** is ticked on the Tuning page (the
default). Pausing leaves it as it is. The Tuning page also holds the section name,
`output_pin leds` by default.

The mapping, gear ratios, speeds and index settings are saved on the server
(`sorter_settings`, document `machine`; routines are document `routines`), so every browser and
the sorter's control loop see the same values. The first time a browser opens the page against an
empty server, its older per-browser settings are uploaded. If the server can't be reached, the page
says so and keeps changes in that browser only, without overwriting the server later.

C4 indexing has an **Accel** setting (rpm per second, default 60; 0 turns the ramp off). Each index
move speeds up and slows down on a trapezoid instead of starting and stopping at full speed.

## Cameras (Cameras page)

**Setup** lists every camera Windows has. **Show picture** gives a live view (wave a hand in front
of it to see which one it is), and **Watches** puts it over C2, C3 or C4. Leave the laptop's own
camera on Not used. A camera watches one channel; choosing a camera for a channel that has one
replaces it. The **Channels** list says how each assigned camera is doing, and a red note names
any assigned camera that isn't plugged in. Each channel's tab then draws its zones and captures
the empty channel.

**How cameras are recognized.** Identical cameras share a name ("USB Camera"), so they're saved by
their DirectShow device path, such as
`@device_pnp_\\?\usb#vid_0bda&pid_5844&mi_00#7&3390689c&0&0000#{…}\global`. These cameras have no
serial number, so Windows builds the `7&3390689c&…` part from the USB port: **keep each camera in
the same USB port** (label the cables). Moved to another port it shows up as a new camera, and its
channel reports it missing until it's assigned again. The card shows the short form,
`0bda:5844 · 3390689c` (USB vendor:product · port instance).

**ffmpeg.** Device paths need a current ffmpeg; the 2013 one from a Python package (first on
PATH here) only lists names. A current one is installed with `winget install Gyan.FFmpeg`, and
with the ffmpeg setting empty the backend uses winget's
(`%LOCALAPPDATA%\Microsoft\WinGet\Links\ffmpeg.exe`) before PATH. The Setup view shows which
ffmpeg and version it found.

**Sizes.** Every camera opens at the size under All cameras, picked from 320×240 to 1280×720
(1280×720 by default, to start sharp and lower as needed). It must be one each camera offers:
**Check sizes** on a camera's card lists them and warns if it isn't. The frame rate isn't asked
of the camera (most offer only 30 fps); ffmpeg drops frames down to the FPS setting. The USB
camera here offers 160×120 to 1280×720 in MJPEG at 30 fps, and runs a steady 10 fps at
1280×720 using about a fifth of a CPU core each for ffmpeg and the backend.

The detector's defaults scale with the size: the smallest piece by pixel count (120 px at
640×480, 360 at 1280×720) and its sampling step by width (every 2nd pixel at 640, every 4th at
1280), so a large frame costs about what a small one does. Changing the size needs a new
empty-channel picture on each channel; the channel says so until it has one.

**Find which camera is where** (Setup view) does the assigning for you. With the board connected
and the sorter not running, it starts the ticked cameras (the laptop's own is left unticked) and
measures how much each picture changes with nothing moving. Then it turns C1, C2, C3 and C4 in
turn a few degrees (15 by default) and back, comparing each camera's picture before and after.
Only the middle half of each picture (width and height, outlined on the result's pictures) is
compared, so a neighboring channel at a camera's edge doesn't count. A camera saw a channel move
when at least 0.3% of that middle changed and three times its own noise. The strongest
camera–channel pairs are proposed, one camera per channel, with before/after pictures and the
full table of changes. **Use these** applies them. Notes say when no camera saw a channel move,
when two cameras saw it about equally, and when a camera saw only C1 (which has no camera). If
no camera saw anything move, it says to check motor power instead of guessing. Each channel needs
something visible that turns with it: a piece or a strip of tape. It runs as a sorter job, so
Cancel stops it, and every channel is turned back.

**Patterned rotors.** C1–C3's cones have 10 facets and C4's platter 5 sectors, and they turn
with the rotor, so a single empty-channel picture only matches where it was taken: elsewhere the
facet edges look like pieces. **Capture empty across a section** (each channel's tab) turns the
clear channel across one section (36° on C1–C3, 72° on C4, from each channel's **sections**
setting) and captures it every degree. The detector compares each frame with whichever of those
pictures it looks most like, chosen by picture rather than motor position, so it survives a board
reset. A pixel counts as changed only if it differs from every pixel within a few pixels of the
matched picture, which covers the rotor standing between two pictures; real pieces differ from
all of them. Against 180 pictures, detection takes about 7 ms a frame at 640×480. On C3 on
2026-11-11 this took the facets from several false pieces a turn to none over 12 positions.

A camera is opened once and shared: a channel using it and a preview of it watch the same
capture, so previewing an assigned camera doesn't disturb it. A preview stops 15 s after the page
stops watching it.

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

## At the machine next (2026-11-11)

1. **Motor power.** Check 12 V on the RAMPS **5 A** input (steppers, D9, D10). On 2026-11-11 the
   board took every command but the light stayed dark and the camera saw no channel turn; only a
   C1 move nudged the blue brick, so C1 may be powered, or that was vibration.
2. **Light (D10).** With power, switch it on and off (Dashboard) and compare C2's pictures; check
   D10's LED on the board and the strip's polarity. Leave it on a minute to confirm the 5 s
   safety timeout never trips.
3. **Gear ratios.** Done: all four set to 130/12. Your saved mapping is C1 stepper_y, C2
   stepper_z, C3 stepper_x, C4 stepper_e0; the 2026-11-11 C1 nudge was stepper_x, so your C3.
4. **Find which camera is where.** Put a piece or tape on each channel and run it from the
   Cameras page, Setup view.
5. **Supply voltage sensing (wiring).** A divider from the 12 V 5 A rail to a free analog pin
   (AUX-2 A5 or A9–A12): 10 kΩ from 12 V to the pin, 4.7 kΩ from the pin to ground (12 V →
   3.8 V; 14 V stays under 5 V). The software is built: add `[voltage_sensor supply]` with
   `sensor_pin: analog5` (or your pin) to the board config. The Dashboard then shows motor
   power, and homing and sorting are refused below 10.5 V (Tuning, Motor power). Set the
   calibration from a meter reading.
6. **Piece detection.** Take the brick off, capture C2 empty, drop pieces on and check they're
   found and tracked as the channel turns.

## Open questions

- Every channel (C1–C4) is geared the same: a 12t gear on the motor drives a 24t idler, which
  drives the 130t gear on the rotor. The idler only reverses direction, so the ratio is
  130/12 ≈ 10.833 motor turns per rotor turn (the default; C4 was earlier set to 10.8–10.9 by
  eye). The stage speeds (C1 5 rpm, C2 7.5 rpm, C3 11.25 rpm) are still placeholders to tune.
- C4 indexing ramps (see Accel above). Feeder runs and the distributor's bin moves still start at
  full speed.
