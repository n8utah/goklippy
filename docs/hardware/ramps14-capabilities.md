# RAMPS 1.4 — Board Capabilities Reference

This document lists every connection point goklippy exposes for the **RAMPS 1.4** board profile. During **board setup** in the web UI, you mark which of these are actually wired on your bench rig. Only marked connections are configured on the MCU and shown as active controls.

Pin names follow [Klipper AVR pin naming](https://www.klipper3d.org/Config_Reference.html#pin-names-for-avr-boards) (`ar54` = Arduino digital 54, `analog13` = A13, etc.).

---

## Stepper Drivers (Pololu sockets)

RAMPS has five driver sockets. Each socket exposes step, direction, enable, and optional UART pins depending on the driver type.

| Profile ID | RAMPS Label | Step | Dir | Enable | Notes |
|------------|-------------|------|-----|--------|-------|
| `stepper_x` | X | D54 | D55 | D38 | Primary X axis |
| `stepper_y` | Y | D60 | D61 | D56 | Primary Y axis |
| `stepper_z` | Z | D46 | D48 | D62 | Primary Z axis |
| `stepper_e0` | E0 | D26 | D28 | D24 | Extruder 0 / aux axis |
| `stepper_e1` | E1 | D36 | D34 | D30 | Extruder 1 / aux axis |

**Setup options per stepper:** connected (yes/no), user label, invert direction, microstep jumpers noted in comments only (hardware).

---

## Endstops

RAMPS endstop headers use mechanical switch or opto wiring. Klipper config uses internal pull-up (`^`) by default.

| Profile ID | RAMPS Header | Pin | Typical Use |
|------------|--------------|-----|-------------|
| `endstop_x_min` | X-MIN | D3 | X minimum |
| `endstop_x_max` | X-MAX | D2 | X maximum |
| `endstop_y_min` | Y-MIN | D14 | Y minimum |
| `endstop_y_max` | Y-MAX | D15 | Y maximum |
| `endstop_z_min` | Z-MIN | D18 | Z minimum |
| `endstop_z_max` | Z-MAX | D19 | Z maximum |

**Setup options per endstop:** connected (yes/no), user label, invert logic, NC vs NO (invert).

---

## Thermistors (TEMP headers)

| Profile ID | RAMPS Header | Pin | Typical Use |
|------------|--------------|-----|-------------|
| `thermistor_t0` | T0 | A13 (D65) | Hotend / probe 0 |
| `thermistor_t1` | T1 | A15 (D69) | Heated bed |
| `thermistor_t2` | T2 | A12 (D66) | Aux thermistor |
| `thermistor_t3` | T3 | A14 (D68) | Aux thermistor |

**Setup options per thermistor:** connected (yes/no), user label, sensor type (NTC 100k beta 3950 default, PT1000, etc.).

---

## Heater MOSFET Outputs

| Profile ID | RAMPS Label | Pin | Max Current (typ.) | Notes |
|------------|-------------|-----|--------------------|-------|
| `heater_hotend` | D10 | D10 | ~5 A | Hotend cartridge |
| `heater_bed` | D8 | D8 | ~15 A | Heated bed (requires PSU) |
| `heater_fan` | D9 | D9 | ~1 A | Often wired as part-cooling fan, not heater |

> D9 is the "Fan" MOSFET on RAMPS silkscreen. Many builds use it for part cooling. Mark it as a **fan** or **heater** in setup depending on your wiring.

**Setup options:** connected (yes/no), user label, output type (heater PWM vs fan PWM), max PWM cap.

---

## Fan & Digital Outputs

| Profile ID | RAMPS Label | Pin | Notes |
|------------|-------------|-----|-------|
| `fan_part` | D9 | D9 | Part cooling (if not used as heater) |
| `fan_aux1` | D4 | D4 | Aux output / relay candidate |
| `fan_aux2` | D5 | D5 | Aux output / relay candidate |
| `output_servos` | SERVO | D11 | Servo signal (SERVO0) |

Additional digital outputs available for relay boards:

| Profile ID | Pin | Notes |
|------------|-----|-------|
| `gpio_d23` | D23 | Exp header |
| `gpio_d25` | D25 | Exp header |
| `gpio_d27` | D27 | Exp header |
| `gpio_d29` | D29 | Exp header |
| `gpio_d31` | D31 | Exp header |

**Setup options:** connected (yes/no), user label, output type (fan, relay, generic digital).

---

## Servos

| Profile ID | Header | Pin | Notes |
|------------|--------|-----|-------|
| `servo_0` | SERVO0 | D11 | Standard RAMPS servo header |
| `servo_1` | SERVO1 | D6 | Secondary (shared with E1 enable on some boards — verify wiring) |

**Setup options:** connected (yes/no), user label, min/max angle.

---

## LCD / Exp Header (informational only for v1)

These pins exist on RAMPS but are **not** in the v1 control UI. Listed for wiring awareness:

| Function | Pins |
|----------|------|
| SD card (SPI) | D53, D50, D52, D51 |
| LCD EXP1/EXP2 | D16, D17, D23, D25, D27, D29, D31, D33, D35, D37 |

---

## Board Setup → Klipper Config

When you save a board setup, goklippy:

1. Loads the RAMPS 1.4 profile (all possible slots above).
2. Filters to **connected** slots only.
3. Generates Klipper `config` commands for those objects at connect time.
4. Shows control widgets only for connected peripherals.

Example: if only X stepper, T0 thermistor, and D9 fan are connected, the dashboard shows three sections — not the full board map.

---

## Suggested Bench-Minimal Setup

For first bring-up with goklippy:

| Slot | Connected | Label |
|------|-----------|-------|
| `stepper_x` | ✓ | X test motor |
| `thermistor_t0` | ✓ | Room temp probe |
| `fan_part` (D9) | ✓ | Test fan |
| Everything else | ✗ | — |

Add peripherals in board setup as you wire them.
