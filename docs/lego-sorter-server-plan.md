# LEGO sorter server: plan for Go + React

Status: draft plan, 2026-11-09. Branch `lego-sorter`.

## Goal

Give goklippy the machine-side behaviour of the [Basically Sorter V2](https://github.com/basicallysource/sorter-v2)
backend (`software/sorter/backend`, about 73k lines of Python plus a SvelteKit UI), running on our
Go backend, React frontend, SQLite, and a RAMPS 1.4 board with Klipper MCU firmware.

We copy **what the machine does**, not how the Python is written. Where their design is driven by
Python limits (the GIL, threads, `numpy`) or by their hardware (RP2040 firmware, TMC2209 StallGuard,
Waveshare bus servos), we do what fits Go and our hardware instead.

The existing [test bench](./lego-sorter-test-bench.md) stays. It is the manual tool for one motor at
a time. This plan adds the autonomous sorting loop around it.

## What the Python server does

The sorter is a pipeline of five rotating stages and a tower of bins:

```
bulk bucket → C1 (bulk) → C2 → C3 → C4 (classification platter) → chute → layer door → bin
                         cam     cam    cam + photos → Brickognize
```

| Area | What it does | Python home |
|---|---|---|
| Lifecycle | `initializing → ready/standby → homing → paused ⇄ running`, plus error. Pause keeps pieces' classifications; stop clears them. | `sorter_controller.py`, `coordinator.py` |
| Coordinator | One control loop. Each tick steps distribution, then classification, then feeder. Stations talk through gates ("C4 can take a piece", "chute is ready") on a small message bus. An active incident holds the whole process. | `coordinator.py`, `subsystems/bus`, `subsystems/shared_variables.py` |
| Feeder | Pulses C1–C3. Per channel per tick: piece in **exit** zone and next channel ready → small exit pulse; exit busy downstream → hold; piece only in **drop** zone → bigger drop pulse; empty → idle. C1 has no camera and advances unless C2's drop zone is busy. Holds after a fall until the piece lands. | `subsystems/feeder/pulse_perception/` |
| Perception | Cameras over C2, C3, C4. A detector gives boxes, boxes are mapped to angles around the ring, an angular tracker keeps ids, and each channel reports which zones (`drop`, `precise`, `exit`) hold a piece. | `vision/`, `perception/`, `zones/` |
| Classification (C4) | A 5-sector platter that only turns forward. A piece lands in the drop zone, is photographed (a burst of crops), sent to Brickognize, carried round, and dropped off the exit once the chute is aimed for it. Up to two pieces on the platter, one cycle ahead of the chute. Watchdog for a stuck channel. | `subsystems/classification_channel/` |
| Identification | Brickognize HTTP API: up to 8 JPEGs → ranked parts and colors. Primo/Duplo filtered. Optional feedback on wrong answers. | `classification/brickognize*.py` |
| Sorting profile | Part + color → category. Hive's compiled JSON: ordered rules, first match wins; part variants (`3010pb176`) fall back to the base part; kits take parts only until the kit is full. Unmatched → `misc`. | `sorting_profile.py` |
| Bins | Layers × sections × bins. Category → bin: existing assignment, else first free bin. Disabled layers/sections, too-big pieces to the bottom bucket, layer size limits. | `irl/bin_layout.py`, `bin_layout_store.py`, `bin_contents.py` |
| Distribution | Chute homes on an endstop, aims at a section by angle, closes the door of the layer that should catch the piece, opens the rest. Positioning → ready → sending. Optional periodic position check against the switch. | `subsystems/distribution/` |
| Incidents & recovery | Typed incidents (stuck on C4, piece circling, stuck at landing, motor stall, chute needs homing, doors not answering, no bin). Each has an operator action, and some have automatic recovery: shake profiles and probe turns. | `incidents/`, `recovery/` |
| Records | Every piece: track history, photos, predictions, category, bin, problems. Runs, lifetime stats, runtime stats, logs. | `piece_records.py`, `sorting_runs.py`, `runtime_stats.py` … |
| UI API | ~40 FastAPI routers plus WebSocket events and MJPEG camera feeds. | `server/` |
| Cloud and platform | Hive sync and models, telemetry, Wi-Fi, Tailscale, firmware flashing, SorterOS, training uploads. | `server/hive_*`, `fleet_telemetry.py` … |

## Hardware differences that change the design

| Theirs | Ours | Consequence |
|---|---|---|
| RP2040 firmware with its own trapezoid motion: host sends "move N steps" | Klipper MCU: host sends `queue_step` timing | We build ramps on the host. A blocking `MoveRotorDeg(ch, deg, rpm, accel)` that returns when the move ends is phase 0 work. The test bench notes C4 has no accel ramp yet. |
| TMC2209 on UART, StallGuard stall detection | RAMPS drivers, no stall feedback | No stall incidents from the drivers. Stalls show up only as "perception says nothing moved". Drop the StallGuard pages. |
| Waveshare bus servos that report position | Hobby servos on PCA9685 over I2C | Doors can't confirm they moved. "Doors not answering" becomes "PCA9685 not answering on I2C" (we already have `CheckPCA9685Boards`). |
| Orange Pi 5, Linux, NPU | One Windows laptop runs the backend, UI and all three cameras | Capture and detection must be light enough to run three cameras plus the loop on a laptop CPU. |
| Chute home switch | E1 has no homing yet ("set position as bin 1") | We add an endstop on the distributor and home via trsync. |

## Architecture in goklippy

### Backend packages

The control loop is business logic, so it lives in `backend/logic/`. Hardware stays in
`backend/mcu/`. SQL stays in `backend/models/`.

```
backend/
  logic/
    sorter/                 lifecycle, coordinator, wiring
      machine.go            state machine: standby/homing/ready/paused/running/error
      coordinator.go        the tick loop and the gates between stations
      feeder/               C1–C3 pulse logic
      platter/              C4 flow and the pieces riding it
      distribution/         chute aiming, doors, bin choice
      profile/              part+color → category, profile import
      bins/                 layout, assignment rules, contents
      incidents/            incident kinds, actions, recovery procedures
      hw.go                 Hardware interface (see below)
      hw_mcu.go             implementation on mcu.Session
      hw_sim.go             simulator for dev and tests
    vision/
      camera/               capture from USB cameras
      zones/                ring + drop/precise/exit arcs, angle math
      detect/               Detector interface + implementations
      track/                angular tracker
      perception.go         per-channel occupancy published to the coordinator
    brickognize/            HTTP client + types
  models/
    sorter_settings.go  bins.go  profiles.go  pieces.go  runs.go  incidents.go
  routes/
    sorter_routes.go  sorter_ws_routes.go  camera_routes.go  bins_routes.go
    profiles_routes.go  records_routes.go
```

### Concurrency model

The Python spends a lot of code working around the GIL and threads. In Go:

- **One goroutine owns all sorter state** (the coordinator). It ticks every 20–50 ms and is the
  only code that changes stations, pieces or bins. HTTP handlers never touch that state. They send
  commands (`Pause`, `Resume`, `HomeChute`, `ResolveIncident`…) on a channel and wait for a reply.
- **Each camera has a goroutine** that captures frames, runs detection and tracking, and publishes
  the latest `ChannelObservation` (zones occupied, tracks, timestamp) through an atomic value. The
  coordinator reads the latest snapshot each tick and never waits on vision.
- **Slow I/O runs off the loop**: Brickognize calls, image writes and DB writes run in worker
  goroutines and report back to the coordinator through channels.
- **Motion runs on the hardware layer**. Moves are started from the loop and finish on their own.
  The loop checks `moving_until` instead of blocking.
- A snapshot of state is published to a fan-out hub for the WebSocket, the same way
  `SubscribeMotion` works today.

### Hardware interface

Everything above `hw.go` works in rotor units (degrees, rpm), as the test bench already does.

```go
// Phase 0 (backend/logic/sorter/hw.go)
type Hardware interface {
    MoveChannel(ch Channel, deg, rpm float64) (done <-chan error, err error)
    SelectLayer(layer int) error // -1 = none: every layer closed, straight to the bottom bucket
    StopAll() error
    Health() HardwareHealth      // board connected, shutdown reason
}
// Added later: HomeChute and AimChute (phase 1), Shake (phase 8), and the PCA9685 check in Health.
```

`hw_mcu.go` maps channels to stepper OIDs and gearing from the sorter settings (today's
per-browser settings, moved server-side). `hw_sim.go` fakes timing and positions so the loop,
tests and the UI can run without the machine. Pair it with recorded camera video (see phase 3)
and most of the pipeline can be built away from the bench.

### Persistence (SQLite, `database/schema.sql`)

Replaces their `machine.toml` and JSON files, and our `localStorage` settings. Like theirs, it is one SQLite file on the machine (`private/goklippy.db`).

| Table | Holds |
|---|---|
| `sorter_settings` | Named JSON documents. `machine`: channel mapping and gearing, speeds, distributor and layer servos (later also feeder tuning, platter, chute calibration, cameras, zones). `routines`: the bench page's rotor routines. JSON keeps tuning changes free of schema changes; the backend reads the fields it needs over its defaults and keeps the rest untouched. |
| `sorting_profiles` | Imported profiles: name, source, version, rules JSON, active flag. |
| `bin_layouts`, `bins` | Layer/section/bin geometry, enabled flags, category assignments per profile. |
| `sort_runs` | Start/stop, profile, counts. |
| `pieces` | uuid, run, timestamps per stage, part/color/category/confidences, bin, status, problems JSON. |
| `piece_images` | Path on disk, channel, burst index, sharpness, used-for-prediction flag. Image files go in a data dir (default `private/data/pieces/`), never in the DB. |
| `piece_predictions` | Raw Brickognize response per request. |
| `incidents` | Kind, subject, opened/resolved, resolution, auto-recovery steps. |

### Real-time to the UI

- `GET /api/sorter/ws`: state, gates, station phases, recent pieces, active incident, perception
  summaries. Same pattern as the MCU session WebSocket.
- `GET /api/cameras/:role/stream`: MJPEG. Overlays (boxes, zones, ring) are drawn by React on top
  of the `<img>` from detection JSON on the WebSocket, so the stream stays a plain camera feed.

### Frontend pages

Feature folders per `AGENTS.md`. Each page builds its own components first.

| Page | Contents | Phase |
|---|---|---|
| `SorterPage` (exists) | Manual test bench, unchanged | — |
| `SorterDashboardPage` | State badge, Home / Pause / Resume, camera tiles with overlay toggle, turn and shake buttons, incident card, recent pieces | 7 |
| `CamerasPage` | Assign devices to C2/C3/C4, preview, draw ring and zones on the frame | 2 |
| `PerceptionPage` | Live boxes, track ids, zone occupancy per channel | 3 |
| `FeederTuningPage` | Pulse distances, pauses, speeds, max moves, hold after a fall, with defaults and "reset to default" | 4 |
| `ChuteCalibrationPage` | Home, aim at first bin, aim at a later bin, compute offsets, test every bin, door angles | 1 |
| `BinsPage` | Layers × sections × bins, enable/disable, assign categories, point the chute at a bin | 6 |
| `ProfilesPage` | Import, list, activate, view rules | 6 |
| `RecordsPage` + `PiecePage` | Runs, pieces with filters, one piece's photos, predictions, problems, review buttons | 8 |

## Phases

Each phase ends with something we can run on the bench. Phases 1 and 2 can go in parallel.

### Where things stand (2026-11-09)

Everything below is built and tested in software; none of it has run on the machine yet.
`sorter.simulate: true` runs the whole sort against pretend LEGO (`logic/sorter/sim_world.go`),
and `models/sort_e2e_test.go` sorts known pieces end to end on SQLite and checks every bin.

| Phase | State |
|---|---|
| 0 Foundations | Built. Bench check: C4 index with acceleration. |
| 1 Distribution | Built. Needs the chute's home switch fitted, then homing, calibration and test-every-bin. |
| 2 Cameras and zones | Built: `logic/camera` (ffmpeg per camera, MJPEG split into frames, device list with DirectShow device paths, sizes each camera offers), one shared capture per camera (`logic/sorter/camera_pool.go`), live streams, and the **Cameras** page: Setup to preview every camera and pick which watches C2/C3/C4, then the zone editor and empty-channel capture per channel. Tested with the USB camera over C2 and ffmpeg 9 (winget). Needs the other two cameras. |
| 3 Perception | Built: `logic/vision` (zones, observations, box merging, angular tracker, background-difference detector) and `sorter.Perception`. Needs real frames to tune the detector's sensitivity. |
| 4 Feeder | Built: `logic/sorter/feeder.go`, tuned on the **Tuning** page. Flags a piece that won't ride for recovery. |
| 5 Platter (C4) | Built: `logic/sorter/platter.go`, a slot model of the 5-sector carousel. Brickognize built and checked live. |
| 6 Profiles and bins | Built. |
| 7 Coordinator and dashboard | Built: the machine runs platter then feeder each tick, with runs and the **Dashboard** page. |
| 8 Records, incidents, recovery | Built: pieces, photos, runs, incidents, reviews and the **Records** page; shakes and Sorter V2's landing-held procedure, clearing a stalled C4, shaking off a piece that didn't fall; bin counts with full bins and "emptied"; kit limits from set inventories. |
| Light | Built: the LED light on D10 as `[output_pin leds]` (on/off, `!`, `value`, `shutdown_value`), switched from the **Dashboard** and on when sorting starts. See the test bench notes. Needs the light wired. |

Not planned: BrickStore export, Hive sync, and sending reviews back to Brickognize as feedback.
Still open: the "no bin" incident (pieces go to the bucket with a problem noted instead), a piece
circling C4, and auto-assigning bins from recent counts.

### Phase 0: Foundations

**Status (2026-11-09): built, waiting on the bench check.** What landed:

- `backend/mcu/motion_profile.go`: trapezoid step times compressed into
  `queue_step interval/count/add` chunks within 25 µs of exact (Klipper's tolerance). A 72° C4
  slot at 10 rpm and 60 rpm/s is 31 commands. `Session.StepperMove` sends the first 40 with
  `reset_step_clock` and appends the rest straight after, and returns when the move ends. C4
  indexing (`IndexMove.AccelHz`, `accel_hz2` on the index route) uses it.
- `sorter_settings` table, `models/sorter_settings.go`, and `logic/sorter/settings.go`.
- `logic/sorter`: `Hardware` with `MCUHardware` (the one connected session, rotor units, step
  remainders carried per channel) and `SimHardware` (`sorter.simulate: true` in the config).
  `SelectLayer(n)` puts layer n's servo at its open angle and the rest at closed; `-1` closes
  them all, so pieces fall through to the bucket.
- `logic/sorter/machine.go`: the loop goroutine, `standby → homing → ready → running ⇄ paused`
  and `error`. Until phase 1 adds the endstop, "home" checks the board is connected and closes
  every layer. Losing the board while homed, running or paused is an error.
- Routes under `/api/v1/sorter`: `GET/PUT settings/:name`, `GET state`, `POST commands`,
  `POST channels/:channel/move`, `GET ws`.
- The bench page reads and writes server settings and has a C4 **Accel** field.

Original scope:

- Move sorter settings from `localStorage` to `sorter_settings` (`GET/PUT /api/sorter/settings`).
  The test bench page reads and writes the same document. Import the browser's existing settings
  once.
- **Accelerated moves**: trapezoid step timing for `queue_step` in `backend/mcu`, and a "move
  rotor degrees, tell me when done" call. Needed by the C4 index and the chute.
- `Hardware` interface, `hw_mcu.go`, `hw_sim.go`.
- Sorter lifecycle skeleton: `standby → homing → ready → running ⇄ paused`, with commands over a
  channel and state on the WebSocket. In this phase "running" does nothing.
- Mage targets stay as they are; add `go test ./backend/logic/sorter/...` to backend test.

**Done when** the bench page uses server settings and a 72° C4 index with acceleration lands on
the next slot repeatedly.

### Phase 1: Distribution on its own (no cameras)

**Status (2026-11-09): built, waiting on the switch to be fitted and the bench check.** What landed:

- `mcu/session_home.go`: `HomeStepper` arms `endstop_home` on the stepper's stop trsync and polls
  `endstop_query_state` until it fires; `EndstopTriggered` reads the switch. Endstop pins now take
  Klipper's `^` (pull-up) and `!` (invert) prefixes.
- `logic/sorter/chute.go`: their geometry (`theta0 + s*60° + (i+0.5)*W/K`, the no-go wedge past
  the travel limit) and the two-capture calibration.
- `MCUHardware` homes the chute (turning off the switch first if it starts on it), reads its angle
  from the MCU step count, measures which way that count runs during homing, and keeps manual
  distributor moves inside the travel once homed.
- The machine runs homing, aim, send-to-bin and test-a-layer as jobs: one at a time, cancellable,
  with progress in the snapshot. Homing means home the chute, close every layer, park at bin 1.
  Losing the chute's home while ready is an error.
- Routes: `GET /sorter/chute`, `POST /sorter/chute/{aim,zero,calibrate}`,
  `POST /sorter/distribution/{send,test}`, `POST /sorter/jobs/cancel`.
- The **Chute** page. The bench page's flat bin map and "Set position as bin 1" are gone.
- Calibration only calculates on the backend; the page saves the result into its settings, so the
  page stays the only writer of the `machine` document.

Original scope:

- Distributor endstop (new hardware, added to the board setup) and homing: counter-clockwise to
  the switch = 0°, then clockwise to the bins. Manual zero stays only as a dev fallback.
- Bin geometry: layers, sections per layer (6), bins per section (2 or 3), section pitch, pillar
  width, first-section offset. Calibration from two captured bins as their docs describe.
- Doors: one servo per layer, close the target layer, open the others; all open = bottom bucket.
  Wait for doors to settle before aiming.
- `POST /api/sorter/distribution/send {layer, bin}` and a "test every bin" routine.
- `ChuteCalibrationPage`.

**Done when** we can send a hand-dropped piece to any bin by number and the test-every-bin pass
looks right.

### Phase 2: Cameras and zones

- Capture: one **ffmpeg** subprocess per camera (`dshow` on the Windows laptop) producing MJPEG,
  decoded in Go with `image/jpeg`. No cgo, and the JPEG bytes go straight to the browser stream.
  ffmpeg must be on the laptop's `PATH` (or set in config); the backend reports it missing clearly.
  On 2026-11-09 the laptop's `PATH` found a 2013 ffmpeg under the Python 3.12 `Scripts` folder,
  which is too old. Install a current build (`winget install Gyan.FFmpeg`) and point config at it.
- Device listing, assigning a device to C2 / C3 / C4, resolution and fps, exposure where the
  driver allows it.
- Zone model from `zones/model.py`: ring center and radii, and `drop`, `precise`, `exit` arcs in
  image angles going forward; pure functions with tests.
- Record a short clip per camera to `private/data/recordings/`. Replaying a clip through
  perception lets us work on phases 3–5 without running the machine.
- `CamerasPage` with a zone editor drawn over the live frame.

### Phase 3: Perception

- `Detector` interface: `Detect(frame) []Box`.
- **First detector: background difference** inside the ring mask against a learned empty-channel
  image, then blobs → boxes. Pure Go, fast at 640×480, and the Python ships an equivalent (`mog2`)
  as a built-in option. Pieces on a plain channel against a lamp should separate well.
- **Later detector: ONNX YOLO/NanoDet**, the kind Hive serves, through onnxruntime
  (`yalue/onnxruntime_go`, needs cgo and the runtime library). Only if background difference
  isn't reliable enough.
- Merge nearby boxes (one piece split into colour regions), box → angle on the ring, zone
  attribution (`perception/arcs.py`).
- Angular tracker: predict angle with ω·dt, greedy nearest match, average-colour gate, short
  re-acquire window (`perception/angular_tracker.py`).
- Publishes `ChannelObservation{Drop, Precise, Exit bool; Tracks []Track; At time.Time}` per
  channel.
- `PerceptionPage`.

**Done when** a recorded clip of pieces moving round C3 gives stable track ids and correct zone
flags.

### Phase 4: Feeder

- The pulse decision per channel each tick (table above), with each channel's `moving_until`.
- C1 blind: advance on a timer unless C2's drop zone is occupied.
- Gates: C3 may only drop into C4 when the platter says it can take a piece.
- Hold after a fall: fixed hold first; follow measured landing times (p90 + 0.3 s, clamped
  0.5–2.5 s, after 20 landings) later.
- Exit move refinements from their tuning page (longest exit move, room kept for the next piece,
  keep the next piece back) come after the basic loop works.
- Settings apply live (re-read on change; no restart). `FeederTuningPage` with defaults.

**Done when** the machine meters pieces one at a time into C4 for 10 minutes with C4 disabled.

### Phase 5: Classification platter (C4)

**Status (2026-11-09): the Brickognize part is built; the platter waits on the cameras.**

- `logic/brickognize`: the client below, tested against a fake server and checked once against
  the live API.
- `sorter.IdentifyAndRoute` and `POST /sorter/identify`: up to 8 photos (JPEG, or PNG converted to
  JPEG) go to Brickognize, and the best match is routed under the active profile as a dry run.
  The Bins page's **Identify from photos** panel uses it, so identification and routing can be
  tried with phone photos before the cameras are in.

Original scope:

- 5 sectors at 72°, forward only. Sector states: free, occupied, handoff, exit.
- When a new track enters the drop zone: take a burst of crops (around 8, sharpest kept), create a
  piece, send the burst to Brickognize on a worker. Reject a multi-drop (two separate boxes in the
  drop zone) straight to misc.
- Index forward when the next sector is free and, for the piece reaching the exit, the chute is
  aimed and its doors are set. A piece still waiting for its answer when it reaches the exit goes
  to misc. Timeout configurable.
- Watchdog: no progress for 30 s with a piece on the channel → try clearing (up to 2 turns, all
  doors open), then raise "stuck on classification channel".
- `brickognize` package: multipart POST of up to 8 JPEGs to
  `https://api.brickognize.com/predict/?predict_color=true`, keep the original rank of each item
  and color, drop Primo/Duplo, timeouts that grow with image count.

**Done when** pieces go from C3 through C4 and land in the bottom bucket with a part and color
recorded.

### Phase 6: Profiles and bins

**Status (2026-11-09): built (done before phases 2–5, which wait on the cameras).** What landed:

- `logic/sorter/profile`: parses Hive's `sorting_profile.json` (schema 1) in both formats, the
  compiled `program` (rules, kits, `fallback` by category or color, `default`) and the flat
  `part_to_category` map. Base-part stripping, and their routing tests ported as table tests.
  Kits take parts with no limit until piece records exist to count them (phase 8).
- `logic/sorter/bins.go`: `ChooseBin` after their `_findOrAssignBinForCategory`: a category's own
  bins (random among several), else the first empty usable bin, else a shared bin if
  `sharedBins`, else a misc bin, else the bucket. Misc never claims a bin by itself; pieces over
  80 mm, or over a layer's limit (18-bin layers 50 mm, 12-bin 80 mm), skip it. Disabled layers and
  sections and unreachable bins are skipped.
- `logic/sorter/sorting.go`: import, list, activate (keep / reset / pre-assign in rule order),
  delete, the bins view, setting a bin's categories (moving them from other bins), and `Route`,
  which saves a newly given bin when committing. Each profile keeps its own bins.
- Tables `sorting_profiles` (file plus a summary, at most one active) and `bin_categories`.
- Routes: `GET/POST /sorter/profiles`, `POST /sorter/profiles/:id/activate`,
  `DELETE /sorter/profiles/:id`, `GET /sorter/bins`, `PUT /sorter/bins/:layer/:bin`,
  `POST /sorter/bins/reset`, `POST /sorter/route` (a dry run: "where would this go?").
- **Profiles** and **Bins** pages. Layer and section switches, per-layer size limits and
  `sharedBins` are machine settings.
- Not done yet: the "no bin" incident (phase 8), full bins (needs piece counts), auto-assign by
  recent counts, high-value and not-in-inventory routing, and local profiles without Hive.

Original scope:

- Profile import: Hive's `sorting_profile.json` (schema version 1, `program.rules` or the older
  flat `part_to_category`), so profiles built in Hive work here. Also a minimal local profile:
  "by BrickLink category" or "by colour", with no Hive needed.
- Lookup: `color-part`, then `any_color-part`, then the same for each base part
  (`assy`, `pb/pr/pat/ps`, `cNN`, a trailing letter); first rule wins; kit rules stop taking a part
  once the kit has enough. Port their tests as table tests.
- Bin assignment: existing assignment, else first unassigned bin in an enabled layer/section;
  several bins for one category → pick one; too big (over 80 mm or over the layer's limit) →
  bottom bucket; no bin left → "no bin" incident with "send to bucket".
- On activate: reset bins, or pre-assign in rule order.
- `BinsPage`, `ProfilesPage`.

### Phase 7: Coordinator and dashboard

- Wire the stations: each tick runs distribution, then platter, then feeder. Incidents hold the
  gates.
- Pause keeps pieces on C4 classified, stop clears them; resume needs the chute homed.
- Sort runs start and end with run/pause.
- `SorterDashboardPage`: camera tiles with overlays, turn ±N° and shake per channel, recent pieces
  (on the channel now, then the last 8 delivered), state and run counters.

**Done when** a handful of known pieces sort into the right bins unattended.

### Phase 8: Records, incidents and recovery

- Save pieces, images, predictions and problems as they happen (worker goroutine, never on the
  loop).
- Records page with filters, piece page, and the review buttons (part correct, colour correct,
  issue flags). Sending feedback to Brickognize is optional.
- Incident kinds that make sense on our hardware: stuck on C4, piece circling C4, stuck at a
  landing, chute needs homing, PCA9685 not answering, no bin. Each with its operator action.
- Recovery as data, from `recovery/procedures.py`: a list of moves (turn, shake profile, probe
  backwards) tried in order, checking perception after each.
- Lifetime and per-run stats: pieces per minute, time spent in each station phase.

### Later, if wanted

- ONNX detector (phase 3 alternative) and fetching Hive's public default detection model.
- BrickStore `.bsx` export of what's in each bin; "not in inventory" bins.
- Set progress / parts checklists for kit profiles.
- 3D tower view.
- Hive account linking and uploads.

## Out of scope

Platform and cloud parts that don't apply to goklippy: Hive sync, telemetry and heartbeat, fleet
naming, Wi-Fi, Tailscale, SorterOS, firmware flashing, macOS camera controls, StallGuard tuning,
power stress tests, training-data collection and training runs, the Gemini/SAM detector, and the
Python's legacy go-to-angle feeder.

## Testing

- Pure logic (zones, arcs, tracker, profile lookup, bin assignment, chute angles, feeder
  decisions) is table-tested with `testify`, with cases taken from the Python tests where they
  exist.
- The coordinator runs against `hw_sim` and recorded clips in tests: "these frames → these moves".
- Bench checks at the end of each phase, as listed above.

## Decisions (confirmed 2026-11-09)

1. **Host and cameras.** Everything runs on the Windows laptop attached to the RAMPS board, with
   three USB cameras on it: C2, C3 and C4.
2. **Capture.** One ffmpeg subprocess per camera (`dshow`), MJPEG decoded in Go. No GoCV.
3. **Detector.** Start with background difference. ONNX stays in "Later, if wanted" unless it
   isn't reliable enough.
4. **Identification.** Brickognize is the identifier. The laptop needs internet while sorting.
5. **Profiles.** Import Hive's `sorting_profile.json` format.
6. **Chute homing.** We add an endstop to the distributor. Phase 1 homes on it; manual zero is
   only a dev fallback.
7. **Bin geometry.** Their tower layout: 6 sections per layer, 2 bins per section on a half-size
   layer (12) and 3 on a third-size layer (18), bins numbered per layer. The bench page's flat bin
   count is replaced by this in phase 1.
