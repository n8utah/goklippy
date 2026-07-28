# RAMPS 1.4 Hardware Setup

Prepare a RAMPS 1.4 board for use with goklippy. This guide covers firmware, wiring, and the board-setup flow in the web UI.

**Prerequisites**

- RAMPS 1.4 shield + Arduino Mega 2560 (or ATmega2560 board with RAMPS form factor)
- 12 V PSU for motors/heaters (if testing steppers or MOSFET outputs — not required for identify-only testing)
- USB cable (Mega 2560 → host PC)
- At least one Pololu-style stepper driver if testing motors (A4988, DRV8825, etc.)

**Related docs**

- [RAMPS 1.4 Capabilities](./ramps14-capabilities.md) — full list of UI connection slots
- [Main PRD](../prds/Main%20PRD.md)

---

## 1. Flash Klipper MCU Firmware

goklippy talks **Klipper MCU firmware** over serial. Marlin and other firmware will not work.

### 1.1 Clone Klipper

```bash
git clone https://github.com/Klipper3d/klipper.git
cd klipper
```

### 1.2 Configure for ATmega2560 (RAMPS)

```bash
make menuconfig
```

Set:

| Setting | Value |
|---------|-------|
| Micro-controller Architecture | Atmega AVR |
| Processor model | atmega2560 |
| Bootloader offset | No bootloader |
| Clock Reference | 16 MHz |

Save and exit.

### 1.3 Build and flash

Connect the Mega 2560 via USB, then:

**Linux / macOS:**

```bash
make flash FLASH_DEVICE=/dev/ttyUSB0   # adjust device path
```

**Windows (PowerShell):**

Klipper's `make flash` uses `avrdude`. Options:

1. **Arduino IDE / avrdude manually** — flash `out/klipper.elf.hex` to the Mega at `0x0000`.
2. **WSL** — build in WSL, flash with the Windows COM port passed through (e.g. `/dev/ttyS3` mapping).

Find your COM port in Device Manager (e.g. `COM5`). Example avrdude command (adjust paths):

```powershell
avrdude -c wiring -p m2560 -P COM5 -b 115200 -D -U flash:w:out/klipper.elf.hex:i
```

Build the hex first with `make` inside the klipper directory.

### 1.4 Verify flash

After reset, the board should enumerate as a USB serial device. No text banner is required — Klipper MCU waits silently for the host protocol.

Note the COM port (Windows) or `/dev/ttyUSB0` / `/dev/ttyACM0` (Linux). You will select this port in goklippy.

---

## 2. USB Drivers (Windows)

Many Mega clones use **CH340** or **CP2102** USB-serial chips.

- If the board does not appear under **Ports (COM & LPT)** in Device Manager, install the manufacturer driver for your USB chip.
- Avoid having two programs open the same COM port at once (PrusaSlicer, serial monitor, old Klipper instance, etc.).

---

## 3. RAMPS Physical Layout

```
                    ┌─────────────────────────────┐
   Power in (12V) ──┤  D8  D9  D10  (MOSFETs)    │
                    │  X   Y   Z   E0  E1 drivers │
                    │  Endstops: X/Y/Z min/max    │
                    │  T0 T1 T2 T3  (thermistors) │
                    │  SERVO  EXP1  EXP2          │
                    └─────────────────────────────┘
                              │
                         Arduino Mega 2560
                              │ USB
                              ▼
                           Host PC
```

### Power notes

| Rail | Use |
|------|-----|
| 5 V USB | Powers Mega logic only — enough for identify + GPIO test **without** motors |
| 12 V RAMPS input | Required for stepper drivers and MOSFET outputs |
| Motor drivers | Install on RAMPS sockets with correct orientation (EN/STEP/DIR) |

**Safety:** Do not enable bed or hotend heaters without proper thermistor feedback and fire-safe testing practices. goklippy caps heater PWM in software, but treat heaters as dangerous.

---

## 4. Wiring Peripherals

Wire only what you plan to use, then declare those connections in **board setup** (see section 6).

### Stepper motor (example: X axis)

| Motor wire | Driver socket |
|------------|---------------|
| 1A / 1B | Coil A |
| 2A / 2B | Coil B |

Enable the X driver socket on RAMPS. Set microstepping jumpers on the driver per your motor datasheet (common: 1/16 for A4988).

### Thermistor (example: T0)

| Thermistor | RAMPS T0 |
|------------|----------|
| Signal | T0 pin |
| Ground | GND on same header |

Use a 100k NTC with 100k pull-up (RAMPS default) unless you configure a different sensor type in board setup.

### Fan or relay (example: D9)

| Device | RAMPS |
|--------|-------|
| Fan + / relay IN | D9 positive side |
| Ground | GND |

Flyback diode required for relay coils. Fans are typically polarized 12 V.

### Endstop (example: X-MIN)

| Switch | RAMPS X-MIN |
|--------|-------------|
| Signal | S |
| Ground | `-` |

Normally-open (NO) microswitch to GND is the most common wiring; mark **invert** in board setup if your switch is NC.

### Servo (example: SERVO0)

| Servo wire | RAMPS SERVO |
|------------|-------------|
| Signal (usually orange) | D11 |
| +5 V | VCC (do not back-power heavy servos from RAMPS 5 V regulator) |
| Ground | GND |

Use a separate 5 V supply for larger servos if needed.

Full slot list: [ramps14-capabilities.md](./ramps14-capabilities.md).

---

## 5. Install Stepper Drivers Safely

1. **Power off** RAMPS 12 V and disconnect USB before inserting drivers.
2. Align driver pins with socket (EN, STEP, DIR, VMOT, GND, VDD).
3. Install heatsinks with airflow if running motors.
4. Set driver current (Vref) with a multimeter **before** first motion test — start low.

---

## 6. Board Setup in goklippy (Web UI)

After firmware is flashed and goklippy is running:

1. **Scan** — find your Klipper MCU on the correct COM port.
2. **New board setup** — choose profile **RAMPS 1.4**.
3. **Capability map** — the UI shows every possible connection slot (steppers, thermistors, heaters, fans, endstops, servos, aux GPIO). See [capabilities doc](./ramps14-capabilities.md).
4. **Mark connected** — for each slot that is physically wired, toggle **Connected** and optionally set:
   - User label (e.g. "bench X motor")
   - Sensor type (thermistors)
   - Output type (fan vs heater vs relay)
   - Invert direction / invert endstop
5. **Save setup** — give the setup a name (e.g. "RAMPS bench v1"). Setups are stored locally in the goklippy database.
6. **Board setup** in the web UI — click pins on the **board diagram** to configure wiring
7. **Config file tab** — manual raw text edit when needed; stays synced with diagram
8. **Connect** → live diagram shows readings on configured pins

You can duplicate a setup and change connections as your bench wiring evolves without re-flashing firmware.

---

## 7. Pre-Flight Checklist

Before connecting in goklippy:

- [ ] Klipper MCU firmware flashed (not Marlin)
- [ ] Correct COM port identified; no other app holding the port
- [ ] 12 V PSU connected if testing motors or MOSFET outputs
- [ ] Stepper drivers installed and current limited
- [ ] Thermistors wired before enabling any heater output
- [ ] Board setup saved with only actually-connected slots marked
- [ ] Fire-safe workspace for heater tests

---

## 8. Troubleshooting

| Symptom | Likely cause |
|---------|--------------|
| Port not listed | USB driver, bad cable, wrong board |
| Port listed, not Klipper | Marlin/other firmware still on board — re-flash |
| Connect fails at config | Board setup references a pin conflict — review connected slots |
| Stepper buzzes, no motion | Enable line, driver Vref, or 12 V not present |
| Thermistor reads 0 / 999 | Wrong sensor type in setup or wiring to wrong T header |
| Access denied on COM port | Another program using the port; close serial monitors |

Enable backend debug logging (`mcu.debug: true` in config when implemented) to inspect serial protocol traffic.

---

## 9. Next Steps

1. Complete [M1 protocol spike](../implementation-plan.md#m1--protocol-spike-backend-only) — verify identify on your COM port.
2. Create your first board setup with minimal connections (one stepper + one thermistor recommended).
3. Expand connected slots as you add hardware.
