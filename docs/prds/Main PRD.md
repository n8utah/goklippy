# Product Requirements Document (PRD)

**Product Name:** goklippy  
**Version:** 0.4 (Draft)  
**Last Updated:** 2026-07-28

---

## 1. Summary

goklippy is a **visually driven** web workbench for Klipper MCU boards. A board diagram lets you hover pins, click to configure what's connected, and edit options in a panel — while still supporting full manual Klipper config editing. It discovers devices over serial, loads existing configs, connects to the MCU, tests hardware live, and saves changes back to disk.

The first supported board profile is **RAMPS 1.4** (ATmega2560). Future board profiles (SKR, BTT, etc.) will plug in via the same profile system without rewriting core protocol logic.

This is **not** a full Klipper host replacement. goklippy does not run print jobs, parse G-code, or perform motion planning. It is a config editor, bench tester, and direct hardware control tool for development, bring-up, and workshop use.

---

## 2. Problem Statement

Klipper MCU firmware is powerful and well-tested, but interacting with a bare MCU outside of a full Klipper printer setup is awkward:

- You need to wire up klippy or write custom scripts to talk the Klipper serial protocol.
- There is no friendly UI for probing "what's plugged in?" and exercising individual pins/peripherals.
- Board pin maps live in Klipper config files that are not easy to browse or drive interactively.
- Editing `printer.cfg` in a text editor gives no live feedback — you can't easily test whether a pin change works without restarting klippy.

goklippy fills that gap: load an existing Klipper config (or start from a board profile), view and edit it in the UI, connect to a RAMPS board flashed with Klipper MCU firmware, test hardware live, and save the config back out.

---

## 3. Goals

| Goal | Description |
|------|-------------|
| **Discover** | Detect serial devices and identify which ones are running Klipper MCU firmware |
| **Connect** | Establish a session, complete the Klipper identify/config handshake, and track connection state |
| **Control** | Send commands to run steppers, set servo positions, read thermistors, and toggle digital outputs (relays, fans) |
| **Monitor** | Stream live sensor readings (temperature, endstop state, stepper position where available) to the UI |
| **Profile boards** | Ship a RAMPS 1.4 profile listing every possible connection point on the board |
| **Configure wiring** | Let users define a saved **board setup** that marks what's connected where; only connected peripherals are configured and controllable |
| **Visual board setup** | Interactive board diagram — hover pins, click to connect, configure peripherals in a modal |
| **Edit Klipper configs** | Load existing `.cfg` files; visual and manual editors stay in sync |
| **Test configs live** | Connect to an MCU using the current config and exercise hardware from the board view |

---

## 4. Non-Goals (v1)

- Full Klipper host (`klippy`) parity: G-code execution, kinematics, bed mesh, input shaping, etc.
- Marlin or other non-Klipper firmware support
- Firmware build/flash tooling (document manual flash steps instead)
- Multi-MCU coordinated motion (single MCU session per connection in v1)
- Slicer integration or print job management
- Remote access over the internet (**local machine / LAN only for v1**)
- Mobile-native apps (responsive web is sufficient)

---

## 5. Target Users

| Persona | Use Case |
|---------|----------|
| **Maker / hobbyist** | Bench-testing a RAMPS board after flashing Klipper MCU firmware |
| **Hardware developer** | Verifying stepper wiring, thermistor readings, and relay logic before integrating into a larger system |
| **Workshop operator** | Quick manual control of attached hardware without SSH scripts |
| **Klipper tinkerer** | Iterating on `printer.cfg` with a visual editor and immediate hardware test loop |

---

## 6. User Stories

### Discovery & Connection

- As a user, I can open goklippy and see a list of attached serial devices so I know what's available.
- As a user, I can see which devices are identified as Klipper MCUs (vs unknown serial devices) so I pick the right port.
- As a user, I can connect to a Klipper MCU and see connection status (connecting, ready, error, disconnected).
- As a user, I can disconnect cleanly without leaving the MCU in a bad state.

### Visual Board Setup

- As a user, I see a **picture/diagram of my board** (RAMPS 1.4 first) as the primary setup view.
- As a user, I can **hover** over pins and headers to see their name, purpose, and connection status.
- As a user, I can **click** an pin/header to open a panel where I choose **what is connected** (stepper, thermistor, fan, heater, endstop, servo, relay, etc.).
- As a user, after selecting a type, I see **all relevant options** for that connection (sensor type, invert, label, PWM limits, etc.) in the same panel.
- As a user, connected pins are **visually distinct** on the diagram; unconfigured pins appear neutral.
- As a user, on the live dashboard, connected pins show **real-time values** (temperature, endstop state, on/off) on or near the diagram.
- As a user, I can click a connected pin during a session to open **inline controls** (jog, toggle, slider).

### Board Setup (list / sync)

- As a user, I can select a board profile (RAMPS 1.4) and see all connection points — **diagram first**, list view optional.
- As a user, I can create a named **board setup** by clicking pins on the diagram or importing from a Klipper config.
- As a user, I can save, edit, and duplicate board setups.
- As a user, when I connect, I pick a saved setup or loaded config — dashboard reflects what's on the diagram.

### Klipper Config — Manual Edit

- As a user, I can switch to a **Config file** tab to view and edit Klipper config manually (structured form + raw text).
- As a user, manual config edits **sync back** to the board diagram (pin states update after save/reload).
- As a user, I can jump from a pin's connection panel to the **raw config section** for advanced edits.

### Klipper Config Load / Save

- As a user, I can point goklippy at a Klipper config file and load it — the **board diagram populates** from parsed sections.
- As a user, I can edit section parameters in the visual panel **or** manual config editor.
- As a user, I can save my edits back to the original config file (with automatic backup before overwrite).
- As a user, loading a config **populates** the board setup / dashboard view so configured hardware matches what's in the file.
- As a user, I can connect to an MCU using the **currently loaded config** and test whether my edits work.
- As a user, I can export a board setup to Klipper config sections (and vice versa) so visual setup and file editing stay in sync.
- As a user, `[include]` directives are resolved so included files appear in the UI and save goes to the correct source file.

### RAMPS 1.4 Hardware Control

- As a user, I can jog X/Y/Z/E stepper axes individually with configurable distance and speed.
- As a user, I can enable/disable stepper drivers so axes hold position or release.
- As a user, I can read hotend and heated-bed thermistor temperatures in real time.
- As a user, I can toggle fan outputs and relay/digital outputs mapped on the RAMPS board.
- As a user, I can set a servo position on the configured servo pin (typically RAMPS SERVO0 on pin D11).
- As a user, I can read endstop states for homing verification.

### Safety & Feedback

- As a user, I see clear warnings before enabling heater outputs or other high-energy peripherals.
- As a user, I see error messages when a command fails or the MCU disconnects unexpectedly.
- As a user, live readings update automatically without manual refresh.

---

## 7. Functional Requirements

### 7.1 Device Discovery

| ID | Requirement |
|----|-------------|
| FR-D1 | Backend scans available serial ports on a configurable interval or on-demand |
| FR-D2 | Backend probes candidate ports with the Klipper `identify` handshake |
| FR-D3 | Discovered devices expose: port path, USB description (if available), MCU identifier string, firmware version/build info |
| FR-D4 | Non-Klipper serial devices may appear in the list but are marked as unsupported |
| FR-D5 | Frontend displays discovery results in a device list with last-scan timestamp |

### 7.2 Connection Management

| ID | Requirement |
|----|-------------|
| FR-C1 | User selects a discovered Klipper MCU and either a **saved board setup** or a **loaded Klipper config** as the connect source |
| FR-C2 | Backend opens serial connection, sends the resolved Klipper config (from setup generation or loaded file), and waits for `ready` |
| FR-C3 | Only one active session per MCU port at a time |
| FR-C4 | Disconnect shuts down outputs safely where possible (disable steppers, turn off heater PWM) |
| FR-C5 | Connection state is exposed via REST and pushed to UI via WebSocket |

### 7.3 Board Profiles & Setups

| ID | Requirement |
|----|-------------|
| FR-B1 | **Board profiles** define slots, pins, Klipper templates, **diagram SVG**, and **hotspot map** ([spec](../ui-visual-board.md)) |
| FR-B2 | RAMPS 1.4 profile exposes all standard slots — 5 steppers, 4 thermistors, 3 MOSFET outputs, 6 endstops, 2 servos, aux GPIO — per [capabilities doc](../hardware/ramps14-capabilities.md) |
| FR-B3 | Profiles are data-driven (YAML + SVG) and loaded by the backend without recompilation |
| FR-B4 | **Board setups** (user-created, persisted in Postgres) reference a profile and store per-slot: `connected` bool, user label, and slot-specific options |
| FR-B5 | **Visual setup UI** renders board diagram; hover → tooltip; click → connection modal with peripheral type + options |
| FR-B5a | Connection modal filters types by what the pin accepts (stepper socket vs TEMP vs MOSFET, etc.) |
| FR-B5b | Diagram shows connected / unconfigured / live / error states with distinct styling |
| FR-B6 | Connect flow requires a saved setup or loaded config; backend rejects invalid slot ids |
| FR-B7 | Klipper config generation includes only connected slots — disconnected slots emit no config objects |
| FR-B8 | **Live dashboard** reuses board diagram with value overlays and inline controls on connected pins |
| FR-B9 | CRUD API for setups: create, list, get, update, duplicate, delete |
| FR-B10 | Import: parsing a Klipper config maps sections to hotspots/slots (diagram populates) |
| FR-B11 | Export: board setup generates valid Klipper config section text for connected slots |

### 7.4 Klipper Config Files

| ID | Requirement |
|----|-------------|
| FR-K1 | Backend parses Klipper INI-style config: sections, key=value pairs, comments, continuation lines |
| FR-K2 | Backend resolves `[include]` directives (at least one level deep in v1; recursive in v2) |
| FR-K3 | Config root path is set in goklippy YAML config; load/save paths must stay within allowed directories |
| FR-K4 | Load returns structured sections with source file path per section (for includes) |
| FR-K5 | UI offers **Visual** and **Config file** tabs; both edit the same underlying config |
| FR-K6 | Visual tab uses board diagram; Config tab provides section list + structured form |
| FR-K7 | Config tab provides **raw text editor** for full file or per-section manual edits |
| FR-K7a | "View in config" link from connection modal jumps to corresponding raw section |
| FR-K7b | Visual ↔ config bi-directional sync on save/reload |
| FR-K8 | Save writes changes to the correct source file; creates timestamped `.bak` backup before overwrite |
| FR-K9 | Unsaved changes are tracked; UI warns before navigating away or connecting with stale edits |
| FR-K10 | Connect uses the in-memory resolved config (all includes merged for MCU `[mcu]` + hardware sections) |
| FR-K11 | Only hardware-relevant sections are sent to the MCU at connect — not `[virtual_sdcard]`, `[display]`, etc. |
| FR-K12 | Config validation on save: unknown sections warn; invalid pin syntax errors before save |

### 7.5 Hardware Commands

| ID | Requirement |
|----|-------------|
| FR-H1 | **Steppers:** jog (relative move), stop, enable/disable driver |
| FR-H2 | **Servos:** set angle or pulse width within safe bounds |
| FR-H3 | **Thermistors:** read temperature via ADC on a polling interval |
| FR-H4 | **Digital outputs:** set on/off for fans and relay channels |
| FR-H5 | **Heaters:** optional PWM set with explicit user confirmation and max duty cap in v1 |
| FR-H6 | **Endstops:** read triggered/open state |
| FR-H7 | All commands validate against the active session config (section must exist in loaded/generated config) |

### 7.6 Web Interface

| ID | Requirement |
|----|-------------|
| FR-U1 | Primary flow: **board diagram** → connect → live board dashboard |
| FR-U2 | **Visual board page** — SVG/image diagram, hotspot hover/click, connection modal ([spec](../ui-visual-board.md)) |
| FR-U3 | **Config editor** — tab toggle Visual \| Config file; manual raw text always available |
| FR-U4 | Live dashboard overlays readings on diagram; inline controls on pin click |
| FR-U5 | Live values via WebSocket (fallback: polling) |
| FR-U6 | Local-only v1; default dev login on localhost OK |
| FR-U7 | Responsive layout; diagram usable on tablet at bench (pinch-zoom optional) |
| FR-U8 | Optional compact list view for accessibility / power users |

### 7.7 Logging & Diagnostics

| ID | Requirement |
|----|-------------|
| FR-L1 | Backend logs serial traffic at debug level (configurable) |
| FR-L2 | UI shows last N MCU error/responses for troubleshooting |
| FR-L3 | REST endpoint returns MCU stats (uptime, configured objects, last clock) when connected |

---

## 8. RAMPS 1.4 v1 Board Profile (Reference)

Based on standard Klipper RAMPS configurations. Exact pin names will match Klipper config conventions.

| Peripheral | Typical RAMPS 1.4 Mapping |
|------------|---------------------------|
| X stepper | Step D54, Dir D55, Enable D38 |
| Y stepper | Step D60, Dir D61, Enable D56 |
| Z stepper | Step D46, Dir D48, Enable D62 |
| E0 stepper | Step D26, Dir D28, Enable D24 |
| Hotend thermistor | Analog A13 (D65/TEMP_0) |
| Bed thermistor | Analog A15 (D69/TEMP_1) |
| Hotend heater | D10 |
| Bed heater | D8 |
| Part cooling fan | D9 |
| Aux fan / relay candidate | D4 or D5 |
| X endstop | D3 (X-MIN) |
| Y endstop | D14 (Y-MIN) |
| Z endstop | D18 (Z-MIN) |
| Servo | D11 |

> **Note:** Users can start from a **board profile** (blank bench) or an existing **Klipper config file** (migrated printer). Both paths converge at connect + dashboard.

Hardware setup guide: [docs/hardware/ramps14-setup.md](../hardware/ramps14-setup.md)

### Config ↔ UI mapping (examples)

| Klipper section | UI slot / control |
|-----------------|-------------------|
| `[stepper_x]` | X stepper — jog, enable |
| `[extruder]` | E0 stepper + hotend heater + thermistor |
| `[heater_bed]` | Bed heater + thermistor |
| `[fan]` / `[heater_fan]` | Fan / output toggle |
| `[servo]` | Servo slider |
| `[stepper_z]` + endstop pin | Z stepper + endstop read |

---

## 9. Technical Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     React Frontend (Vite)                   │
│  BoardDiagram │ ConnectionModal │ ConfigEditor │ Devices │ LiveBoard   │
└──────────────────────────┬──────────────────────────────────┘
                           │ REST + WebSocket (JWT auth)
┌──────────────────────────▼──────────────────────────────────┐
│                     Go Backend (Gin)                        │
│  routes/  →  logic/  →  models/ (Postgres for sessions/logs)│
│              │                                              │
│              ├── mcu/        Klipper msgproto + serial I/O  │
│              ├── discovery/  Port scan + identify probe     │
│              └── profiles/   RAMPS 1.4 board definitions  │
└──────────────────────────┬──────────────────────────────────┘
                           │ USB Serial
┌──────────────────────────▼──────────────────────────────────┐
│              Klipper MCU Firmware (RAMPS / ATmega2560)    │
│              Steppers │ ADC │ PWM │ GPIO │ Endstops         │
└─────────────────────────────────────────────────────────────┘
```

### Backend Modules (new)

| Package | Responsibility |
|---------|----------------|
| `backend/mcu/` | Klipper serial protocol: encode/decode messages, session lifecycle, command dispatch |
| `backend/logic/discovery/` | Serial port enumeration and Klipper identify probing |
| `backend/logic/mcu_manager/` | Owns active connections, routes commands, publishes events |
| `backend/profiles/` | Static board profile definitions (RAMPS 1.4 first) |
| `backend/models/board_setup.go` | Persist user board setups (Postgres) |
| `backend/logic/klipper_config/` | Parse, merge includes, validate, serialize Klipper `.cfg` files |
| `backend/logic/board_setup/` | Validate setups, merge profile + setup → Klipper config; import/export with config parser |
| `backend/models/klipper_config.go` | Track open config workspace, unsaved edits, file paths |
| `backend/routes/mcu_routes.go` | REST + WebSocket endpoints for discovery, configs, setups, and control |

### Frontend Pages (new)

| Page | Purpose |
|------|---------|
| `BoardSetupPage` | **Visual board diagram** (primary) + Config file tab; connection modal |
| `ConfigEditorPage` | Load/save configs; Visual \| Config tabs share same workspace |
| `DevicesPage` | Scan/list MCUs, connect |
| `LiveBoardPage` | Diagram with live overlays + inline hardware controls |

Board profile ships `diagram` image + `hotspots` map — see [ui-visual-board.md](../ui-visual-board.md).

### Communication

| Channel | Use |
|---------|-----|
| REST | Discovery scan, connect/disconnect, one-shot commands |
| WebSocket | Live telemetry (temps, endstops, connection events) |

---

## 10. Klipper Protocol Notes

goklippy implements the **host-side Klipper MCU protocol** (same family klippy uses). Primary Go reference: [ANYCUBIC-3D/Klipper-go](https://github.com/ANYCUBIC-3D/Klipper-go) — see [klipper-go-reference.md](../klipper-go-reference.md).

1. Open serial at 250000 baud (Klipper default; configurable per profile).
2. Send `identify` to retrieve `data dictionary` (message definitions).
3. Build encoder/decoder from dictionary.
4. Send `config` lines from loaded Klipper config or generated from board setup (hardware sections only).
5. Send `finalize_config` and wait for `ready`.
6. Issue runtime commands: `queue_step`, `set_digital_out`, `query_adc`, etc.
7. Handle `shutdown` and `is_shutdown` states gracefully.

Reference implementation: [Klipper-go](https://github.com/ANYCUBIC-3D/Klipper-go) (`project/msgproto.go`, `project/Mcu.go`, `project/serialhdl.go`). Original: [Klipper host `klippy/mcu.py`](https://github.com/Klipper3d/klipper).

---

## 11. Safety Requirements

| ID | Requirement |
|----|-------------|
| SR-1 | Heater outputs require explicit arming in the UI; default is disarmed |
| SR-2 | Heater PWM capped (e.g. 50% max in v1 unless user overrides with confirmation) |
| SR-3 | Disconnect and shutdown paths disable heaters and steppers when possible |
| SR-4 | Command rate limiting to prevent flooding the MCU |
| SR-5 | UI displays prominent "manual control — not for unattended operation" disclaimer |
| SR-6 | Local-only deployment for v1; no public internet exposure assumed |

---

## 12. Success Metrics (MVP)

| Metric | Target |
|--------|--------|
| Discovery | Klipper MCU on RAMPS identified within 5 s of scan |
| Connect | RAMPS 1.4 connects to `ready` using loaded `sample-ramps` config |
| Config round-trip | Load cfg → T0 hotspot shows connected → edit in modal → save → file updated |
| Visual setup | Click D9 on diagram → pick Fan → save → fan control on dashboard |
| Manual override | Raw edit `pin: ar9` → diagram updates D9 on reload |
| Live overlay | T0 shows live temp on diagram during session |
| Steppers | All four axes jog correctly in both directions |
| Thermistors | Temperature readings within 2 °C of expected at room temp |
| Digital outputs | Fan/relay toggles visible on hardware |
| Servo | Servo sweeps to commanded position |
| UI latency | Telemetry updates ≤ 500 ms behind hardware |

---

## 13. Milestones

### M0 — Foundation (Done)
Go/React scaffold, JWT auth, database, mage tasks.

### M1 — Protocol Spike
Serial open, identify handshake, decode data dictionary, print MCU info.

### M2 — RAMPS Profile + Board Setups + Config Parser
RAMPS profile, setup CRUD, Klipper config load/parse/save, config handshake.

### M3 — Visual Board UI + Config Editor + Discovery
RAMPS SVG diagram, hotspots, connection modal, Visual \| Config tabs, device scan.

### M4 — Live Board Dashboard + Basic I/O
Diagram live overlays, inline controls, thermistors/fans/endstops.

### M5 — Steppers & Servos
Jog from diagram, enable/disable, servo slider.

### M6 — Safety & Polish
Heater guardrails, WebSocket telemetry, error on diagram pins.

### M7 — MVP Release
Docs for flashing Klipper to RAMPS, manual test checklist, demo video.

---

## 14. Open Questions

| # | Question | Proposed Default |
|---|----------|------------------|
| 1 | Store connection history / command logs in Postgres? | Yes — audit log table for sessions and commands |
| 2 | Support Windows serial port naming (`COM3`)? | Yes — primary dev OS is Windows |
| 3 | Custom pin overrides per slot? | Defer to v2; v1 uses profile default pins, user picks connected/not |
| 4 | Protocol implementation source? | Port minimal subset from [Klipper-go](https://github.com/ANYCUBIC-3D/Klipper-go); pure Go serial, no CGO in v1 |
| 5 | WebSocket library choice? | `gorilla/websocket` or Gin-compatible WS |
| 6 | Auth for local-only v1? | Keep JWT scaffold; default dev user on localhost is acceptable |
| 7 | Config editor scope in v1? | Visual diagram primary; Config tab with structured form + raw text |
| 8 | Config file access model? | User sets `config_root` in YAML; backend restricts read/write to that tree |
| 9 | Board diagram asset? | SVG top-view RAMPS 1.4 + hotspot map in profile ([spec](../ui-visual-board.md)) |

---

## 15. Dependencies & Prerequisites

- RAMPS 1.4 (or compatible) board with **Klipper MCU firmware** flashed (not Marlin)
- USB connection to the host running goklippy
- PostgreSQL for auth, board setups, and config workspace metadata
- Hardware setup: [docs/hardware/ramps14-setup.md](../hardware/ramps14-setup.md)
- Existing Klipper config directory (optional) — e.g. `~/printer_data/config/` or a local copy of `printer.cfg`
- Go 1.22+ and Node 20+ for development

---

## 16. References

- [ANYCUBIC-3D/Klipper-go](https://github.com/ANYCUBIC-3D/Klipper-go) — Go klippy port; primary protocol reference ([notes](../klipper-go-reference.md))
- [Klipper3d/klipper](https://github.com/Klipper3d/klipper) — MCU firmware and original host
- [Klipper Config Reference](https://www.klipper3d.org/Config_Reference.html)
- [Klipper MCU Commands](https://www.klipper3d.org/MCU_Commands.html)
- Klipper sample RAMPS config: `config/sample-ramps-*.cfg`
