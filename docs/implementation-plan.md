# goklippy Implementation Plan

**Based on:** [Main PRD](./prds/Main%20PRD.md)  
**Last Updated:** 2026-07-28  
**Hardware docs:** [docs/hardware/](../hardware/) · **Visual UI spec:** [ui-visual-board.md](./ui-visual-board.md)

This plan picks up after the framework scaffold (M0) and sequences work through an MVP that supports RAMPS 1.4 bring-up, **Klipper config editing**, and bench control.

---

## Current State (M0 — Complete)

- Go backend with Gin, JWT auth, Postgres via sqlx
- React/Vite frontend with auth flow
- Atlas migrations, mage build tasks, Playwright smoke tests
- PRD defined; hardware setup docs added; no MCU/serial code yet

---

## Recommended Approach

Build from the wire protocol up, then add config parsing, then expose through API and UI. Avoid parallel frontend work until M3 when backend APIs exist—use CLI or integration tests to validate M1–M2.

### Config + board setup: two paths, one connect pipeline

```
Path A — Klipper config file          Path B — Visual board setup
────────────────────────────          ────────────────────────────
Load printer.cfg                      Pick RAMPS 1.4 profile
Parse sections → UI                   Mark connected slots
Edit in form / raw text               Export → Klipper sections
Save back to disk                     Save setup to DB
         │                                      │
         └──────────────┬───────────────────────┘
                        ▼
              Resolve hardware sections
                        ▼
              Connect MCU → test → dashboard
```

Both paths produce the same internal representation: **resolved hardware config** sent to the MCU at connect time.

### Why Klipper-go (not from scratch)

- [ANYCUBIC-3D/Klipper-go](https://github.com/ANYCUBIC-3D/Klipper-go) is a Go port of klippy with working `msgproto`, serial, and MCU session code.
- Port a **minimal subset** into `backend/mcu/` rather than importing the full `k3c` printer host.
- See [klipper-go-reference.md](./klipper-go-reference.md) for file mapping, license notes, and what to skip (CGO stepcompress, gcode, reactor).

### Risk: msgproto complexity

**Mitigation:** M1 ports `msgproto.go` and `console.go` patterns from Klipper-go. Pure Go serial (`go.bug.st/serial`) replaces CGO `serialqueue` for Windows-friendly builds.

---

## Phase Breakdown

### M1 — Protocol Spike (Backend only)

**Goal:** Connect to a real Klipper MCU on RAMPS and print its identify info.

**Primary reference:** [Klipper-go](https://github.com/ANYCUBIC-3D/Klipper-go) — port/adapt `msgproto.go`, use `console.go` + `Mcu._mcu_identify` as flow guide.

| Task | Details |
|------|---------|
| Study Klipper-go | Clone to `private/klipper-go-ref/` (gitignored); read `msgproto.go`, `serialhdl.go`, `console.go` |
| Port msgproto | `backend/mcu/msgproto/` from Klipper-go `project/msgproto.go` — identify, CRC, dictionary, encode/decode |
| Pure Go serial | `backend/mcu/serial.go` via `go.bug.st/serial` — no CGO chelper in v1 |
| Create `backend/mcu/session.go` | Identify-only session (connect → identify → print version); config in M2 |
| Create `backend/cmd/mcu-probe/` | CLI modeled on Klipper-go `console.go` |
| Config | Add `mcu.default_baud`, `mcu.scan_timeout` to YAML config |
| License | Preserve GPL headers on ported files; note in docs |
| Test | Unit tests with captured identify dictionary fixture; manual test against real board |

**Exit criteria:** `go run ./backend/cmd/mcu-probe --port COM5` prints MCU identifier and dictionary message count.

**Suggested layout:**

```
backend/
  mcu/
    serial.go
    session.go
    msgproto/
      msgproto.go      # ported from Klipper-go
      ...
  cmd/
    mcu-probe/
      main.go
```

---

### M2 — Profiles, Config Parser, Board Setups, MCU Connect

**Goal:** Load/edit/save Klipper configs, board setup CRUD, connect using config file OR setup; MCU reaches `ready`.

**Klipper-go reference:** `project/configfile.go`, `common/configparser/ConfigParser.go`

| Task | Details |
|------|---------|
| Port config parser | `backend/klipperconfig/` from Klipper-go configparser — parse sections, get/set keys, serialize |
| Config service | `backend/logic/klipper_config/` — load file, resolve `[include]`, merge view, save with `.bak` |
| Config YAML | Add `klipper.config_root` path; restrict read/write to that directory |
| Section classifier | Map sections → UI category + hardware vs non-hardware (exclude `[virtual_sdcard]`, etc. from MCU send) |
| Import to setup | `[stepper_x]` etc. → board profile slots (connected + options from file) |
| Export from setup | Connected slots → valid Klipper section text |
| Create `backend/profiles/ramps14.yaml` | Slots + **diagram hotspots** per [ui-visual-board.md](./ui-visual-board.md) |
| Create `backend/profiles/ramps14-diagram.svg` | Top-view RAMPS 1.4 board illustration |
| Profile API | `GET /api/board-profiles/:id/diagram` → image URL, viewBox, hotspots, slot metadata |
| DB migration | `board_setups`, `board_setup_slots`, optional `config_workspace` metadata |
| Create `backend/mcu/config.go` | Send resolved hardware sections; port `_send_config` from Klipper-go `Mcu.go` |
| Port `pins.go` | AVR pin parsing |
| `backend/logic/mcu_manager/` | Connect accepts `{ port, setup_id }` OR `{ port, config_path }` |
| Disconnect safety | Reset/disable on teardown |

**Exit criteria:**
- Load `sample-ramps` cfg → see sections in API response
- Edit `[stepper_x]` step_pin → save → reload → change persisted
- Connect with loaded config → MCU `ready`

**Config API (M2):**

```
GET    /api/klipper-config/list?dir=           # list .cfg files under config_root
POST   /api/klipper-config/load               # { path } → sections[], includes[], dirty=false
GET    /api/klipper-config/workspace          # current in-memory workspace
PUT    /api/klipper-config/sections/:name     # update section keys (structured)
PUT    /api/klipper-config/raw                # replace full file text (advanced)
POST   /api/klipper-config/save               # write to disk + .bak backup
POST   /api/klipper-config/reload             # discard unsaved, reload from disk
POST   /api/klipper-config/import-to-setup    # config → board setup draft
POST   /api/klipper-config/export-from-setup  # setup → config sections
```

**Profile vs setup:**

| Layer | Source | Example |
|-------|--------|---------|
| Profile (static) | `ramps14.yaml` | `stepper_x` exists at pins D54/D55/D38 |
| Setup (user) | Postgres | `stepper_x`: connected=true, label="bench motor" |
| Runtime config | merge | Only connected slots → Klipper config objects |

**RAMPS profile structure (sketch):**

```yaml
id: ramps14
name: RAMPS 1.4
mcu: atmega2560
serial:
  baud: 250000
slots:
  stepper_x:
    type: stepper
    group: steppers
    pins: { step: ar54, dir: ar55, enable: "!ar38" }
    config_template: stepper
  thermistor_t0:
    type: thermistor
    group: thermistors
    pins: { sensor: analog13 }
    defaults: { sensor_type: "NTC 100K beta 3950" }
  # ... all slots from capabilities doc
```

**Board setup API (M2):**

```
GET    /api/board-profiles              # list profiles (ramps14)
GET    /api/board-profiles/:id/slots  # full capability map for setup UI
GET    /api/board-setups
POST   /api/board-setups              # { profile_id, name, slots: [{ id, connected, label, options }] }
GET    /api/board-setups/:id
PUT    /api/board-setups/:id
POST   /api/board-setups/:id/duplicate
DELETE /api/board-setups/:id
```

---

### M3 — Visual Board UI + Config Editor + Discovery

**Goal:** Interactive board diagram as primary UX; manual config tab; scan and connect.

**Visual UI spec:** [ui-visual-board.md](./ui-visual-board.md)

| Task | Details |
|------|---------|
| `BoardDiagram` component | SVG background + transparent hotspot overlays; hover tooltips |
| `ConnectionModal` | Click pin → pick type (stepper/thermistor/fan/heater/endstop/servo) → options form |
| Pin state styling | Unconfigured / connected / live / error CSS states |
| `BoardSetupPage` | **Visual tab (default)** + **Config file tab** (section form + raw text editor) |
| Bi-directional sync | Modal save → config; raw save → diagram state refresh |
| "View in config" | Link from modal to raw section in Config tab |
| `DevicesPage` | Scan, connect modal |
| `backend/logic/discovery/scanner.go` | Port scan + identify probe |
| Unsaved changes | Banner; warn on navigate / connect |

**Exit criteria:** Click T0 on diagram → configure thermistor → save → see hotspot highlighted → switch to Config tab → see `[sensor]` section → connect → live temp on T0.

**Frontend structure:**

```
frontend/src/pages/BoardSetupPage/
  BoardSetupPage.js
  BoardSetupPage.logic.js
  components/
    BoardDiagram/
    ConnectionModal/
    ConfigRawEditor/
    ConfigSectionForm/
```

---

### M4 — Live Board Dashboard + Basic I/O

**Goal:** Same board diagram with live overlays; inline fan/thermistor/endstop controls.

| Task | Details |
|------|---------|
| `LiveBoardPage` | Reuse `BoardDiagram` with `mode=live`; WS pushes values to hotspot badges |
| Click connected pin | Inline control popover (toggle fan, read temp, endstop indicator) |
| Backend commands | digital_out, query_adc, endstops |
| REST + WebSocket | Session telemetry → pin overlay state |

**Exit criteria:** Fan on D9 → D9 hotspot shows ON; T0 shows live °C on diagram.

---

### M5 — Steppers & Servos

**Goal:** Jog axes and move servo from UI.

| Task | Details |
|------|---------|
| Stepper config objects | Ensure profile defines stepper sections correctly in M2 |
| Commands | `extras_manual_stepper` / `extras_force_move` patterns; add `clocksync.go` from Klipper-go |
| REST | `POST .../steppers/:axis/jog`, `POST .../steppers/:axis/enable`, `POST .../servos/:name` |
| Frontend controls | Jog buttons (+/-), distance selector, speed slider, enable toggle |
| Servo UI | Slider 0–180° mapped to pulse width |

**Exit criteria:** X/Y/Z/E0 jog correctly; servo moves; steppers hold when enabled.

---

### M6 — Safety & Polish

**Goal:** Safe enough for bench use; good error UX.

| Task | Details |
|------|---------|
| Heater arming | Two-step enable in UI; cap PWM in backend |
| Disconnect on tab close | WS disconnect handler triggers cleanup |
| Error surfaces | MCU shutdown reason shown in UI |
| Command rate limit | Token bucket per session |
| Logging | Structured slog with session_id, port, command |
| Auth guards | MCU routes behind existing JWT middleware; local dev defaults OK |

**Exit criteria:** Heaters cannot fire without arming; disconnect disables outputs; errors are visible.

---

### M7 — MVP Release

| Task | Details |
|------|---------|
| Manual test script | Step-by-step verification checklist |
| E2E test | Playwright: load config → edit → save → connect (mock) → dashboard smoke |
| README update | Project purpose, quick start, link to [hardware docs](./hardware/) |

---

## Suggested Next Session (Start Here)

When you're ready to code, begin **M1** with these concrete steps:

1. **Flash Klipper MCU firmware** to your RAMPS 1.4 — follow [ramps14-setup.md](./hardware/ramps14-setup.md).
2. **Add serial + mcu-probe CLI** — prove identify works against your board.
3. **Commit the spike** once COM port → MCU name works reliably.

Parallel prep (no code):

- Confirm which COM port Windows assigns to the RAMPS USB adapter
- Copy or symlink a Klipper `printer.cfg` (or Klipper's `config/sample-ramps-*.cfg`) into your `config_root` for M2 testing

---

## API Sketch (Target)

```
# Klipper config
GET    /api/klipper-config/list
POST   /api/klipper-config/load
GET    /api/klipper-config/workspace
PUT    /api/klipper-config/sections/:name
PUT    /api/klipper-config/raw
POST   /api/klipper-config/save
POST   /api/klipper-config/reload
POST   /api/klipper-config/import-to-setup
POST   /api/klipper-config/export-from-setup

# Board profiles & setups
GET    /api/board-profiles
GET    /api/board-profiles/:id/diagram   # SVG + hotspots
GET    /api/board-profiles/:id/slots
GET    /api/board-setups
POST   /api/board-setups
GET    /api/board-setups/:id
PUT    /api/board-setups/:id
DELETE /api/board-setups/:id

# MCU
GET    /api/mcu/ports
POST   /api/mcu/scan
POST   /api/mcu/connect                  # { port, setup_id | config_path }
POST   /api/mcu/disconnect
GET    /api/mcu/sessions/:id
GET    /api/mcu/sessions/:id/ws
POST   /api/mcu/sessions/:id/digital-out
GET    /api/mcu/sessions/:id/adc/:sectionId
...
```

---

## Database Additions (M2)

```sql
-- board_setups: named wiring configuration for a profile
--   id, profile_id, name, created_at, updated_at
-- board_setup_slots: per-slot state
--   setup_id, slot_id, connected, label, options (jsonb)
-- mcu_sessions: active/historical connections (setup_id, port, status)
-- mcu_command_log: optional audit trail
```

Auth tables remain unchanged.

---

## Testing Strategy

| Layer | Approach |
|-------|----------|
| msgproto | Unit tests with captured identify dictionary fixture from real RAMPS |
| Session | Integration test with serial port mock (loopback or recorded transcript) |
| Config parser | Unit tests with sample-ramps.cfg, include files, SAVE_CONFIG block |
| API | testify HTTP tests with mocked mcu_manager |
| Hardware | Manual checklist per milestone |
| Frontend | Playwright against mock API until hardware CI is feasible |

---

## Timeline Estimate (Solo, Part-Time)

| Phase | Effort |
|-------|--------|
| M1 Protocol spike | 3–5 days |
| M2 Profile + config parser + setups | 5–7 days |
| M3 Visual board UI + config editor | 5–7 days |
| M4 Live diagram + basic I/O | 3–4 days |
| M5 Steppers/servos | 3–4 days |
| M6 Safety/polish | 2–3 days |
| M7 Docs/release | 1–2 days |
| **Total MVP** | **~4–5 weeks** |

---

## Decision Log

| Date | Decision | Rationale |
|------|----------|-----------|
| 2026-07-28 | First board: RAMPS 1.4 | Common, well-documented in Klipper, good for bench testing |
| 2026-07-28 | Use Klipper-go as protocol reference | Port msgproto + MCU session subset; avoid full k3c import and CGO for v1 |
| 2026-07-28 | Not a klippy replacement | Scope control; focus on direct hardware control |
| 2026-07-28 | Board setup UX | Show all profile slots; user marks what's connected; config/dashboard scoped to that |
| 2026-07-28 | Local-only v1 | No remote access; JWT scaffold kept with dev defaults |
| 2026-07-28 | Visual-first UI | Board diagram with hover/click hotspots; connection modal; Config tab for manual edit |
