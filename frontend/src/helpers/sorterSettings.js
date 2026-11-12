import { createContext, useContext, useEffect, useRef, useState } from 'react';

import { getSorterSettings, putSorterSettings } from './api';

// The sorter's settings documents, shared by the LEGO Sorter and Chute pages. The
// server keeps them ("machine", "routines"); the browser copies are a backup and the
// source of the first upload.
export const MACHINE_STORAGE_KEY = 'goklippy_sorter_settings';
const SAVE_DEBOUNCE_MS = 400;

export const DEFAULT_SORTER_SETTINGS = {
  feederRpm: { c1: 5, c2: 7.5, c3: 11.25 },
  indexDegrees: 72,
  indexRpm: 10,
  indexAccel: 60,
  dwellMs: 1000,
  showSettings: true,
  distributor: {
    leftOut: false,
    stepper: 'stepper_e1',
    gear: 5,
    rpm: 5,
    accel: 30,
    // The chute: forward turns it clockwise from above; it homes counter-clockwise
    // onto `endstop`. Geometry defaults are Sorter V2's until calibrated.
    forward: true,
    endstop: 'stepper_e1_endstop',
    homeRpm: 2,
    sections: 6,
    sectionWidth: 360 / 6 - 8.25,
    firstSectionOffset: 8.25,
    maxAngle: 350,
    doorSettleMs: 300,
    // Let a category share a bin once every bin is taken, instead of going to misc.
    sharedBins: false,
    // Each layer may also carry disabled, disabledSections (from 1) and maxPieceMm.
    layers: [
      { servo: '', bins: 18, openAngle: 90, closedAngle: 0 },
      { servo: '', bins: 18, openAngle: 90, closedAngle: 0 },
      { servo: '', bins: 18, openAngle: 90, closedAngle: 0 },
      { servo: '', bins: 12, openAngle: 90, closedAngle: 0 },
    ],
  },
  // Every channel: a 12t motor gear drives the rotor's 130t gear through a 24t idler,
  // so 130/12 motor turns per rotor turn.
  channels: {
    c1: { stepper: 'stepper_x', forward: true, gear: 130 / 12, sections: 10 },
    c2: { stepper: 'stepper_y', forward: true, gear: 130 / 12, sections: 10 },
    c3: { stepper: 'stepper_z', forward: true, gear: 130 / 12, sections: 10 },
    c4: { stepper: 'stepper_e0', forward: true, gear: 130 / 12 },
  },
  routines: {},
  // The pulsing feeder (C1–C3) and the classification platter (C4); see System › Settings (helpers/settingsRegistry.js).
  feeder: {
    dropPulseDeg: 30,
    dropPauseMs: 100,
    exitPulseDeg: 2,
    exitPauseMs: 100,
    exitApproach: 0.5,
    holdAfterFallMs: 800,
    minMoveDeg: 0.1,
    maxMoveDeg: 120,
    bulkPulseDeg: 10,
    bulkPauseMs: 1500,
    staleMs: 1000,
    carryRpm: { c1: 5, c2: 12, c3: 12 },
    carryAccel: 30,
    carryPulseDeg: 90,
    maxPieces: { c2: 6, c3: 3 },
    maxCover: 0.2,
    starveMs: 3000,
    starveBulkDeg: 30,
    backupMs: 1000,
    workAroundStuck: true,
    pushPastStuck: true,
    pushSeconds: 3,
    shakeBeforePush: false,
    pushBumpDeg: 45,
    clearSeconds: 30,
    clearRpm: 12,
    clearAccel: 20,
  },
  platter: {
    sectors: 5,
    exitSector: 2,
    identifyTimeoutMs: 20000,
    settleMs: 250,
    burstFrames: 6,
    stallMs: 30000,
    idleIndexMs: 1500,
    clearExitFirst: true,
  },
  // Which camera watches each channel (c2, c3, c4) and where its zones are.
  // devices maps a channel to its camera's id (device path); deviceNames keeps a label
  // for each, to name a camera that's unplugged.
  cameras: {
    devices: {},
    deviceNames: {},
    zones: {},
    width: 1280,
    height: 720,
    fps: 10,
    ffmpeg: '',
    threshold: 0,
    minArea: 0,
  },
  // The machine's light: an [output_pin] in the board config (RAMPS D10).
  lights: { output: 'output_pin leds', onWhenSorting: true },
  // The motors' 12 V supply read through a divider (10 kΩ over 4.7 kΩ); see System › Settings (helpers/settingsRegistry.js).
  supply: {
    sensor: 'voltage_sensor supply',
    topOhms: 10000,
    bottomOhms: 4700,
    referenceV: 5,
    calibration: 1,
    minVolts: 10.5,
  },
  // Which frames of a tracked piece are good pictures of it; see System › Settings (helpers/settingsRegistry.js).
  pictures: {
    isolationPx: 12,
    settleFrames: 3,
    maxDegPerFrame: 6,
    minSharpness: 40,
    sizeFactor: 1.6,
    keepEveryDeg: 15,
    maxKept: 12,
    cropMarginPct: 20,
    cropMaxPx: 512,
    keepJourneys: 200,
    handoffMs: 1500,
  },
};

/** Older saves stored C2 and C3 as multiples of C1. */
function feederRpmFrom(saved) {
  if (saved.feederRpm) return saved.feederRpm;
  if (saved.baseRpm == null) return DEFAULT_SORTER_SETTINGS.feederRpm;
  const c2 = saved.baseRpm * (saved.c2Factor ?? 1.5);
  return { c1: saved.baseRpm, c2, c3: c2 * (saved.c3Factor ?? 1.5) };
}

/** Fills in defaults for anything a saved machine settings document lacks. */
export function normalizeSorterSettings(saved) {
  if (!saved?.channels) return DEFAULT_SORTER_SETTINGS;
  // eslint-disable-next-line no-unused-vars
  const { baseRpm, c2Factor, c3Factor, ...rest } = saved;
  return {
    ...DEFAULT_SORTER_SETTINGS,
    ...rest,
    feederRpm: feederRpmFrom(saved),
    distributor: { ...DEFAULT_SORTER_SETTINGS.distributor, ...saved.distributor },
    // Per channel, so a field added later (sections) gets its default in old saves.
    channels: Object.fromEntries(
      Object.entries({ ...DEFAULT_SORTER_SETTINGS.channels, ...saved.channels }).map(([ch, c]) => [
        ch,
        { ...DEFAULT_SORTER_SETTINGS.channels[ch], ...c },
      ]),
    ),
    cameras: { ...DEFAULT_SORTER_SETTINGS.cameras, ...saved.cameras },
    feeder: { ...DEFAULT_SORTER_SETTINGS.feeder, ...saved.feeder },
    platter: { ...DEFAULT_SORTER_SETTINGS.platter, ...saved.platter },
    lights: { ...DEFAULT_SORTER_SETTINGS.lights, ...saved.lights },
    pictures: { ...DEFAULT_SORTER_SETTINGS.pictures, ...saved.pictures },
    supply: { ...DEFAULT_SORTER_SETTINGS.supply, ...saved.supply },
  };
}

export function readLocal(key) {
  try {
    return JSON.parse(localStorage.getItem(key));
  } catch {
    return null;
  }
}

function writeLocal(key, value) {
  try {
    localStorage.setItem(key, JSON.stringify(value));
  } catch {
    // the server copy is the one that matters
  }
}

/**
 * Keeps one settings document in sync with the server. It starts from this browser's
 * copy, then takes the server's. When the server has none yet, the browser's copy is
 * saved to it, which moves older per-browser settings onto the server. If the server
 * can't be read, nothing is saved to it (so an old browser copy can't overwrite it)
 * and changes stay in this browser.
 *
 * Returns [value, setValue, syncError, savedCount]; savedCount goes up after each
 * successful save, for refetching anything the server derives from the document.
 */
export function useServerDocument(name, storageKey, initial, normalize) {
  const [value, setValue] = useState(initial);
  const [loaded, setLoaded] = useState(false);
  const [syncError, setSyncError] = useState('');
  const [savedCount, setSavedCount] = useState(0);
  const pending = useRef(null);

  useEffect(() => {
    let cancelled = false;
    getSorterSettings(name).then(
      ({ saved, data }) => {
        if (cancelled) return;
        if (saved) setValue(normalize(data));
        setLoaded(true);
      },
      (err) => {
        if (!cancelled) setSyncError(`Settings are only saved in this browser: ${err.message}`);
      },
    );
    return () => {
      cancelled = true;
    };
  }, [name, normalize]);

  useEffect(() => {
    writeLocal(storageKey, value);
    if (!loaded) return undefined;
    pending.current = value;
    const timer = setTimeout(() => {
      pending.current = null;
      putSorterSettings(name, value).then(
        () => {
          setSyncError('');
          setSavedCount((n) => n + 1);
        },
        (err) => setSyncError(`Settings not saved to the server: ${err.message}`),
      );
    }, SAVE_DEBOUNCE_MS);
    return () => clearTimeout(timer);
  }, [name, storageKey, value, loaded]);

  // Leaving the page inside the debounce window still saves the last change.
  useEffect(
    () => () => {
      if (pending.current) putSorterSettings(name, pending.current).catch(() => {});
    },
    [name],
  );

  return [value, setValue, syncError, savedCount];
}

const localMachineSettings = () => normalizeSorterSettings(readLocal(MACHINE_STORAGE_KEY));

const MachineSettingsContext = createContext(null);

/**
 * One copy of the "machine" settings document for the whole app, so a page, the
 * Settings page and the quick editor never hold different copies and overwrite each
 * other's changes.
 */
export function MachineSettingsProvider({ children }) {
  const doc = useServerDocument(
    'machine',
    MACHINE_STORAGE_KEY,
    localMachineSettings,
    normalizeSorterSettings,
  );
  return <MachineSettingsContext.Provider value={doc}>{children}</MachineSettingsContext.Provider>;
}

/**
 * The sorter's "machine" settings document: [value, setValue, syncError, savedCount].
 * Needs MachineSettingsProvider above it.
 */
export function useMachineSettings() {
  const ctx = useContext(MachineSettingsContext);
  if (!ctx) throw new Error('useMachineSettings must be used within MachineSettingsProvider');
  return ctx;
}
