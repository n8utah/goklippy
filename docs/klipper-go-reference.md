# Klipper-go Reference

**Repository:** [ANYCUBIC-3D/Klipper-go](https://github.com/ANYCUBIC-3D/Klipper-go)  
**License:** GPLv3 (same lineage as Klipper / klippy)

Anycubic's Klipper-go is a Go port of **klippy** (Klipper's Python host). It is the best existing Go reference for talking to Klipper MCU firmware over serial. goklippy does **not** need the full printer host — we extract and adapt the MCU protocol layer only.

---

## What Klipper-go Provides

| Area | Key files | Relevance to goklippy |
|------|-----------|------------------------|
| Message protocol | `project/msgproto.go` | **Critical** — identify, CRC, encode/decode, data dictionary |
| Serial I/O | `project/serialhdl.go` | Reference — uses CGO `chelper/serialqueue` |
| MCU session | `project/Mcu.go` | **High** — identify handshake, `_send_config`, `finalize_config`, command wrappers |
| Clock sync | `project/clocksync.go` | **Medium** — needed for timed commands (steppers, PWM) |
| Pin resolution | `project/pins.go` | **Medium** — AVR pin name parsing (`ar54`, `^ar3`, etc.) |
| Debug console | `project/console.go` | **High for M1** — standalone MCU probe / command REPL pattern |
| Config files | `project/configfile.go`, `common/configparser/ConfigParser.go` | **High for M2** — parse, read/write sections, includes |
| Steppers | `project/stepper.go` | Reference — ties into CGO stepcompress |
| Manual stepper | `project/extras_manual_stepper.go` | Reference for jog UX |
| ADC / temps | `project/extras_query_adc.go`, `extras_adc_temperature.go` | Reference for thermistor reads |
| Digital out | `project/extras_digital_output.go` | Reference for fans/relays |
| C helpers | `chelper/msgblock.c`, `serialqueue.c`, stepcompress | Used by full host; optional for goklippy |

---

## Recommended Strategy for goklippy

### Do port/adapt (into `backend/mcu/`)

1. **`msgproto.go` → `backend/mcu/msgproto/`**  
   Pure Go. Handles `identify` / `identify_response`, zlib dictionary decompress, message encode/decode. This is the hardest part to rewrite from scratch — porting saves significant time.

2. **`Mcu.go` subsets → `backend/mcu/session.go`**  
   Extract only:
   - Serial connect
   - `_mcu_identify` flow
   - Config command accumulation + `_send_config` + `finalize_config`
   - `get_config` / ready detection
   - `CommandWrapper` send/query helpers
   - Shutdown handling  

   Strip: printer object graph, gcode, toolhead, steppersync, CAN bus, file output.

3. **`pins.go` → `backend/mcu/pins.go`**  
   Pin string parsing for RAMPS AVR names.

4. **`clocksync.go` → `backend/mcu/clocksync.go`** (M5)  
   Required before stepper/heater timed commands work reliably.

5. **`console.go` patterns → `backend/cmd/mcu-probe/`**  
   Use as blueprint for M1 CLI debugging.

6. **`configparser` + `configfile.go` → `backend/klipperconfig/`** (M2)  
   Parse Klipper `.cfg` files, resolve includes, read/write sections, serialize back to disk. Strip printer-object dependencies; keep parser + file I/O.

### Do NOT import wholesale

- Do **not** add `k3c` as a Go module dependency — it pulls the entire printer host (reactor, gcode, kinematics, webhooks, Anycubic-specific extras).
- Do **not** require CGO `chelper` for M1–M4 — it complicates Windows builds and ties into motion planning we don't need yet.

### Defer / simplify

| Klipper-go feature | goklippy approach |
|--------------------|-------------------|
| CGO `serialqueue` | Pure Go serial (`go.bug.st/serial`) + msgproto framing loop |
| CGO `stepcompress` / `steppersync` | M5: simple `queue_step` jog sequences, or port `extras_force_move` logic |
| Full Klipper config host integration | Port `configparser` only; no full `ConfigWrapper` / printer object graph |
| Reactor / greenlet scheduler | Go contexts + goroutines per MCU session |
| G-code dispatch | Not needed |

---

## M1 Spike Workflow (Using Klipper-go as Guide)

```
1. Open serial port (250000 8N1)
2. Send identify chunks (msgproto DefaultMessages)
3. Process_identify() → build message dictionary
4. Print version/build_versions from dictionary
5. (M2) Add config lines → finalize_config → wait for "ready"
```

Reference flow in Klipper-go:

```
serialhdl.go   SerialReader.Connect_uart()
msgproto.go    MessageParser.Process_identify()
Mcu.go         MCU._mcu_identify() → MCU._send_config()
console.go     KeyboardReader.connect() — good end-to-end minimal example
```

Local clone for inspection (gitignored):

```
private/klipper-go-ref/   # shallow clone, not committed
```

---

## Mapping Klipper-go → goklippy Packages

```
Klipper-go                          goklippy
─────────────────────────────────   ─────────────────────────────
project/msgproto.go                 backend/mcu/msgproto/
project/serialhdl.go (logic only)   backend/mcu/serial.go
project/Mcu.go (subset)             backend/mcu/session.go
project/clocksync.go                backend/mcu/clocksync.go
project/pins.go                     backend/mcu/pins.go
project/console.go                  backend/cmd/mcu-probe/
project/configfile.go               backend/klipperconfig/
common/configparser/                backend/klipperconfig/parser.go
profiles/*.yaml (ours)              backend/profiles/ramps14.yaml
board setup (ours)                  backend/models/board_setup.go
```

---

## License Considerations

Klipper-go is **GPLv3**. Klipper / klippy source it derives from is also GPL.

| Approach | License impact |
|----------|----------------|
| **Port/adapt** msgproto and MCU code into goklippy | Derived work — goklippy likely needs to be GPLv3-compatible for those files; document attribution |
| **Use as reference only** — rewrite from Klipper protocol docs | More work, no GPL coupling from copied code |
| **Import `k3c` module** | Full GPL dependency chain + massive unused surface |

**Recommendation:** Port the minimal protocol files with GPL headers preserved and add a `LICENSE` note / `docs/third-party.md` entry. Confirm license choice before public release.

---

## Key Protocol Constants (from msgproto.go)

Already implemented in Klipper-go — use when porting:

```go
MESSAGE_SYNC         = 0x7e
MESSAGE_MIN          = 5
MESSAGE_MAX          = 64
MESSAGE_PAYLOAD_MAX  = MESSAGE_MAX - MESSAGE_MIN
DefaultMessages:
  "identify offset=%u count=%c"
  "identify_response offset=%u data=%.*s"
```

---

## Peripherals → Klipper-go Patterns

| goklippy control | Klipper-go reference | MCU config object |
|------------------|---------------------|-------------------|
| Thermistor read | `MCU_adc`, `extras_adc_temperature` | `adc_temperature` / `adc` |
| Fan / relay toggle | `MCU_digital_out`, `extras_digital_output` | `digital_out` |
| Heater PWM | `MCU_pwm`, `extras_heaters` | `pwm` |
| Endstop read | `MCU_endstop`, `extras_query_endstops` | `endstop` |
| Stepper jog | `extras_manual_stepper`, `extras_force_move` | `stepper` + `manual_stepper` |
| Servo | `MCU_pwm` or servo config | `servo` / `pwm` |

Our board profile + setup generates the Klipper config sections; Klipper-go shows how those objects translate to runtime commands after `ready`.

---

## What Klipper-go Does NOT Cover

## What Klipper-go Does NOT Cover

- RAMPS 1.4 bench-control UI — we build this
- Klipper config editor (load/edit/save `.cfg` in browser) — we build this
- Board setup / "what's connected" model — goklippy-specific
- Windows-first serial discovery UX — we build this
- WebSocket telemetry — we build this

---

## Links

- [Klipper-go GitHub](https://github.com/ANYCUBIC-3D/Klipper-go)
- [Klipper MCU Commands](https://www.klipper3d.org/MCU_Commands.html)
- [Klipper Config Reference](https://www.klipper3d.org/Config_Reference.html)
- [goklippy Implementation Plan](../implementation-plan.md)
