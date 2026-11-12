import { DEFAULT_SORTER_SETTINGS } from './sorterSettings';

/**
 * Every setting a person tunes, described once (docs/ui-ux-spec.md, "Settings registry").
 * The Settings page, inline SettingFields, the quick editor and search all read from here.
 *
 * A setting's key is its path in the "machine" settings document ("feeder.exitPulseDeg"),
 * except `local` ones, which live in this browser (the theme). Its default comes from
 * DEFAULT_SORTER_SETTINGS, which mirrors the backend's DefaultMachineSettings.
 *
 * Fields: label, help, unit, kind (number | boolean | enum | text), min, max, step,
 * options (enum: [{ value, label }]), keywords (for search), and `stopped: true` for
 * settings that shouldn't change mid-run. Wiring (which stepper drives what) and
 * calibrated geometry stay on their pages, where they're measured.
 */
export const SETTING_SECTIONS = [
  {
    id: 'feeder',
    label: 'Feeder',
    about: 'C1–C3: how fast they turn, and the pulses that carry pieces to each exit.',
    settings: [
      {
        key: 'feederRpm.c1',
        label: 'C1 speed',
        unit: 'rpm',
        step: 0.25,
        min: 0.25,
        max: 30,
        help: 'How fast C1’s rotor turns when it pulses.',
        keywords: 'bulk speed rotor',
      },
      {
        key: 'feederRpm.c2',
        label: 'C2 speed',
        unit: 'rpm',
        step: 0.25,
        min: 0.25,
        max: 30,
        help: 'How fast C2’s rotor turns when it pulses.',
        keywords: 'speed rotor',
      },
      {
        key: 'feederRpm.c3',
        label: 'C3 speed',
        unit: 'rpm',
        step: 0.25,
        min: 0.25,
        max: 30,
        help: 'How fast C3’s rotor turns when it pulses.',
        keywords: 'speed rotor',
      },
      {
        key: 'feeder.dropPulseDeg',
        label: 'Drop-zone pulse',
        unit: '°',
        step: 1,
        min: 0,
        max: 180,
        help: 'How far a piece short of the exit is nudged each pulse. It always stops at the precise zone.',
      },
      {
        key: 'feeder.dropPauseMs',
        label: 'Pause after a drop-zone pulse',
        unit: 'ms',
        step: 10,
        min: 0,
        max: 10000,
        help: 'Lets the camera look again before the next nudge.',
      },
      {
        key: 'feeder.exitPulseDeg',
        label: 'Exit pulse',
        unit: '°',
        step: 0.5,
        min: 0,
        max: 90,
        help: 'How far a piece at the exit is nudged. Smaller is gentler and less likely to push two through together.',
      },
      {
        key: 'feeder.exitPauseMs',
        label: 'Pause after an exit pulse',
        unit: 'ms',
        step: 10,
        min: 0,
        max: 10000,
      },
      {
        key: 'feeder.holdAfterFallMs',
        label: 'Hold after a fall',
        unit: 'ms',
        step: 50,
        min: 0,
        max: 10000,
        help: 'After a piece falls off an exit, that exit pushes nothing for this long, so the next channel sees it land.',
      },
      {
        key: 'feeder.bulkPulseDeg',
        label: 'C1 bulk pulse',
        unit: '°',
        step: 1,
        min: 0,
        max: 180,
        help: 'C1 has no camera: it turns this far whenever C2’s drop zone is clear.',
      },
      {
        key: 'feeder.bulkPauseMs',
        label: 'Pause after a C1 pulse',
        unit: 'ms',
        step: 100,
        min: 0,
        max: 30000,
      },
      {
        key: 'feeder.minMoveDeg',
        label: 'Smallest move',
        unit: '°',
        step: 0.1,
        min: 0,
        max: 10,
        help: 'Moves smaller than this are skipped as noise.',
      },
      {
        key: 'feeder.maxMoveDeg',
        label: 'Largest move',
        unit: '°',
        step: 1,
        min: 1,
        max: 360,
        help: 'A cap on any single pulse.',
      },
      {
        key: 'feeder.staleMs',
        label: 'Camera frame too old after',
        unit: 'ms',
        step: 100,
        min: 100,
        max: 10000,
        help: 'An older frame counts as no information: the channel waits.',
      },
    ],
  },
  {
    id: 'platter',
    label: 'Platter',
    about:
      'C4: pieces land in a sector, are photographed and identified, then drop into the chute.',
    settings: [
      {
        key: 'platter.sectors',
        label: 'Sectors',
        step: 1,
        min: 1,
        max: 12,
        stopped: true,
        help: 'Equal sectors; one index turns one sector.',
      },
      {
        key: 'platter.exitSector',
        label: 'Indexes to the exit',
        step: 1,
        min: 1,
        max: 11,
        stopped: true,
        help: 'A piece drops into the chute this many indexes after it lands.',
      },
      {
        key: 'indexRpm',
        label: 'Index speed',
        unit: 'rpm',
        step: 0.5,
        min: 0.5,
        max: 60,
        help: 'How fast C4 turns from one sector to the next.',
        keywords: 'c4 speed',
      },
      {
        key: 'indexAccel',
        label: 'Index acceleration',
        unit: 'rpm/s',
        step: 5,
        min: 0,
        max: 1000,
        help: 'How quickly C4 gets up to speed. 0 starts at full speed.',
        keywords: 'c4 ramp',
      },
      {
        key: 'platter.settleMs',
        label: 'Settle after landing or an index',
        unit: 'ms',
        step: 50,
        min: 0,
        max: 5000,
      },
      {
        key: 'platter.burstFrames',
        label: 'Photos per piece',
        step: 1,
        min: 1,
        max: 8,
        help: 'Sent to Brickognize together.',
        keywords: 'brickognize identify',
      },
      {
        key: 'platter.identifyTimeoutMs',
        label: 'Brickognize timeout',
        unit: 'ms',
        step: 1000,
        min: 1000,
        max: 120000,
        help: 'A piece still waiting when it reaches the exit goes to misc.',
        keywords: 'identify',
      },
      {
        key: 'platter.idleIndexMs',
        label: 'Index anyway after',
        unit: 'ms',
        step: 100,
        min: 0,
        max: 60000,
        help: 'With pieces aboard and nothing landing, index so they still get delivered.',
      },
      {
        key: 'platter.stallMs',
        label: 'Stuck after',
        unit: 'ms',
        step: 1000,
        min: 1000,
        max: 600000,
        help: 'No progress this long with pieces aboard: turn the channel round to clear it, then ask.',
        keywords: 'jam incident',
      },
    ],
  },
  {
    id: 'pictures',
    label: 'Pictures',
    about:
      'Which frames of a tracked piece are good pictures of it for identifying. A frame must be whole, alone, settled, steady, its usual size and sharp. Diagnose › Piece journeys shows each decision.',
    settings: [
      {
        key: 'pictures.isolationPx',
        label: 'Alone: no other piece within',
        unit: 'px',
        step: 1,
        min: 0,
        max: 200,
        help: 'Another piece’s box closer than this makes a frame “not alone”.',
      },
      {
        key: 'pictures.settleFrames',
        label: 'Settling: skip the first',
        unit: 'frames',
        step: 1,
        min: 0,
        max: 30,
        help: 'While a piece lands and bounces.',
      },
      {
        key: 'pictures.maxDegPerFrame',
        label: 'Moving: at most',
        unit: '°/frame',
        step: 0.5,
        min: 0.5,
        max: 90,
        help: 'Faster than this, the piece is moving too much for a sharp picture.',
      },
      {
        key: 'pictures.minSharpness',
        label: 'Sharp: at least',
        step: 5,
        min: 0,
        max: 5000,
        help: 'The crop’s detail (variance of its Laplacian). Look at the values on Piece journeys to set it.',
        keywords: 'blurry focus',
      },
      {
        key: 'pictures.sizeFactor',
        label: 'Size: within a factor of',
        unit: '×',
        step: 0.1,
        min: 1.1,
        max: 10,
        help: 'Of the piece’s usual box area. A jump means it’s touching something or partly hidden.',
      },
      {
        key: 'pictures.keepEveryDeg',
        label: 'Keep one per',
        unit: '°',
        step: 1,
        min: 1,
        max: 180,
        help: 'Kept pictures this far apart along the channel show the piece turned different ways; the sharpest in each stretch wins.',
      },
      {
        key: 'pictures.maxKept',
        label: 'Keep at most',
        unit: 'per channel',
        step: 1,
        min: 1,
        max: 50,
      },
      {
        key: 'pictures.cropMarginPct',
        label: 'Crop margin',
        unit: '%',
        step: 5,
        min: 0,
        max: 200,
        help: 'Around the piece’s box, on each side.',
      },
      {
        key: 'pictures.cropMaxPx',
        label: 'Crop size at most',
        unit: 'px',
        step: 32,
        min: 64,
        max: 2048,
        help: 'The crop’s longer side.',
      },
      {
        key: 'pictures.keepJourneys',
        label: 'Keep debug pictures for',
        unit: 'pieces',
        step: 50,
        min: 10,
        max: 5000,
        help: 'The newest pieces’ journeys; older ones are deleted.',
      },
      {
        key: 'pictures.handoffMs',
        label: 'Falling takes at most',
        unit: 'ms',
        step: 100,
        min: 200,
        max: 10000,
        help: 'A piece that lands is matched to the one that left the channel above within this time, so its pictures from both join up. Too long and pieces get mixed up; too short and they aren’t joined.',
      },
    ],
  },
  {
    id: 'cameras',
    label: 'Cameras',
    about:
      'How the cameras capture and how pieces are found in their pictures. Which camera watches which channel, the picture size and the zones are on Machine › Cameras.',
    settings: [
      {
        key: 'cameras.fps',
        label: 'Frames per second',
        unit: 'fps',
        step: 1,
        min: 1,
        max: 30,
        help: 'More follows pieces better but costs the laptop more. A change restarts the cameras.',
      },
      {
        key: 'cameras.threshold',
        label: 'Piece threshold',
        step: 5,
        min: 0,
        max: 765,
        help: 'How different from the empty channel a pixel must be (|ΔR|+|ΔG|+|ΔB|) to count as a piece. 0 uses the default, 90.',
        keywords: 'detect sensitivity',
      },
      {
        key: 'cameras.minArea',
        label: 'Smallest piece',
        unit: 'px',
        step: 10,
        min: 0,
        max: 100000,
        help: 'Smaller spots are noise. 0 scales the default to the picture size.',
        keywords: 'detect noise',
      },
      {
        key: 'cameras.ffmpeg',
        label: 'ffmpeg',
        kind: 'text',
        help: 'The ffmpeg program. Empty finds it (the winget install first, then PATH).',
        keywords: 'capture program path',
      },
    ],
  },
  {
    id: 'chute',
    label: 'Chute',
    about:
      'How the chute moves and shares bins. Its wiring, layers and calibration are on Machine › Chute.',
    settings: [
      {
        key: 'distributor.leftOut',
        label: 'Leave the chute out for now',
        kind: 'boolean',
        help: 'Run without it, before its motor and home switch are wired: Home skips the chute and the layer doors, and sorting still feeds and identifies each piece but uses no profile and aims nowhere, so pieces drop off C4 where it lets them go.',
        keywords: 'distributor disable skip endstop homing without',
      },
      {
        key: 'distributor.rpm',
        label: 'Speed',
        unit: 'rpm',
        step: 0.5,
        min: 0.5,
        max: 60,
        keywords: 'distributor',
      },
      {
        key: 'distributor.accel',
        label: 'Acceleration',
        unit: 'rpm/s',
        step: 5,
        min: 0,
        max: 1000,
        help: '0 starts at full speed.',
        keywords: 'distributor ramp',
      },
      {
        key: 'distributor.homeRpm',
        label: 'Homing speed',
        unit: 'rpm',
        step: 0.5,
        min: 0.5,
        max: 30,
        help: 'Slower finds the home switch more exactly.',
        keywords: 'distributor endstop',
      },
      {
        key: 'distributor.doorSettleMs',
        label: 'Wait after the layer doors move',
        unit: 'ms',
        step: 50,
        min: 0,
        max: 5000,
        keywords: 'servo layer',
      },
      {
        key: 'distributor.sharedBins',
        label: 'Share bins when full up',
        kind: 'boolean',
        help: 'Once every bin is taken, let a category share a bin instead of going to misc.',
        keywords: 'misc categories',
      },
    ],
  },
  {
    id: 'light',
    label: 'Light',
    about:
      'The light runs off the RAMPS D10 output, set up in the board config as an [output_pin] with pin: ar10. Switch it from the status panel.',
    settings: [
      {
        key: 'lights.onWhenSorting',
        label: 'Turn on when sorting starts',
        kind: 'boolean',
        help: 'The cameras need it. Starting or resuming a sort switches it on; stopping, an error or the backend exiting always switches it off.',
        keywords: 'leds lamp',
      },
      {
        key: 'lights.output',
        label: 'Board config section',
        kind: 'text',
        help: 'The [output_pin] the light is wired to.',
        keywords: 'leds lamp d10 pin',
      },
    ],
  },
  {
    id: 'power',
    label: 'Motor power',
    about:
      'The 12 V supply to the RAMPS 5 A input, read through a resistor divider into a free analog pin: a [voltage_sensor] section in the board config with sensor_pin: analog5. Until it’s there, power shows as not measured and holds nothing back.',
    settings: [
      {
        key: 'supply.minVolts',
        label: 'Hold back below',
        unit: 'V',
        step: 0.5,
        min: 0,
        max: 24,
        help: 'Homing and sorting are refused, and a run stops, when the supply drops under this. 0 turns that off.',
        keywords: 'voltage supply low',
      },
      {
        key: 'supply.calibration',
        label: 'Calibration',
        unit: '×',
        step: 0.005,
        min: 0.5,
        max: 2,
        help: 'Meter reading ÷ shown reading, to correct the resistors’ tolerance.',
        keywords: 'voltage meter',
      },
      {
        key: 'supply.topOhms',
        label: 'Resistor from 12 V to the pin',
        unit: 'Ω',
        step: 100,
        min: 0,
        max: 1000000,
        keywords: 'voltage divider',
      },
      {
        key: 'supply.bottomOhms',
        label: 'Resistor from the pin to ground',
        unit: 'Ω',
        step: 100,
        min: 1,
        max: 1000000,
        keywords: 'voltage divider',
      },
      {
        key: 'supply.referenceV',
        label: 'ADC reference',
        unit: 'V',
        step: 0.01,
        min: 1,
        max: 5.5,
        help: 'The Mega’s 5 V. Measure it for a closer reading.',
        keywords: 'voltage',
      },
      {
        key: 'supply.sensor',
        label: 'Board config section',
        kind: 'text',
        help: 'The [voltage_sensor] that reads the supply.',
        keywords: 'voltage sensor',
      },
    ],
  },
  {
    id: 'appearance',
    label: 'Appearance',
    about: 'How goklippy looks in this browser.',
    settings: [
      {
        key: 'theme',
        local: true,
        label: 'Theme',
        kind: 'enum',
        options: [
          { value: 'system', label: 'Follow the system' },
          { value: 'light', label: 'Light' },
          { value: 'dark', label: 'Dark' },
        ],
        default: 'system',
        help: 'Dark is easier on the eyes at the machine at night.',
        keywords: 'dark mode light colors',
      },
    ],
  },
];

/** Reads a dotted path ("feeder.exitPulseDeg") from an object. */
export function getPath(obj, path) {
  return path.split('.').reduce((o, k) => (o == null ? undefined : o[k]), obj);
}

/** A copy of obj with the dotted path set to value. */
export function setPath(obj, path, value) {
  const [head, ...rest] = path.split('.');
  if (rest.length === 0) return { ...obj, [head]: value };
  return { ...obj, [head]: setPath(obj?.[head] ?? {}, rest.join('.'), value) };
}

// Every setting, flat, with its section and default filled in.
export const ALL_SETTINGS = SETTING_SECTIONS.flatMap((section) =>
  section.settings.map((s) => ({
    kind: 'number',
    ...s,
    section,
    default: s.default ?? getPath(DEFAULT_SORTER_SETTINGS, s.key),
  })),
);

const BY_KEY = Object.fromEntries(ALL_SETTINGS.map((s) => [s.key, s]));

/** The setting with this key, or undefined. */
export const findSetting = (key) => BY_KEY[key];

/** A value with its unit, for lists and search ("2 °", "On", "Follow the system"). */
export function formatSettingValue(setting, value) {
  if (value == null || value === '') return setting.kind === 'text' ? 'Not set' : '';
  switch (setting.kind) {
    case 'boolean':
      return value ? 'On' : 'Off';
    case 'enum':
      return setting.options.find((o) => o.value === value)?.label ?? String(value);
    case 'number':
      return setting.unit ? `${value} ${setting.unit}` : String(value);
    default:
      return String(value);
  }
}

/** Why a value can't be saved, or '' when it can. */
export function checkSettingValue(setting, value) {
  if (setting.kind !== 'number') return '';
  if (typeof value !== 'number' || Number.isNaN(value)) return 'Enter a number';
  if (setting.min != null && value < setting.min) return `At least ${setting.min}`;
  if (setting.max != null && value > setting.max) return `At most ${setting.max}`;
  return '';
}
