import { useEffect, useRef, useState } from 'react';

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
    layers: [
      { servo: '', bins: 18, openAngle: 90, closedAngle: 0 },
      { servo: '', bins: 18, openAngle: 90, closedAngle: 0 },
      { servo: '', bins: 18, openAngle: 90, closedAngle: 0 },
      { servo: '', bins: 12, openAngle: 90, closedAngle: 0 },
    ],
  },
  channels: {
    c1: { stepper: 'stepper_x', forward: true, gear: 1 },
    c2: { stepper: 'stepper_y', forward: true, gear: 1 },
    c3: { stepper: 'stepper_z', forward: true, gear: 1 },
    c4: { stepper: 'stepper_e0', forward: true, gear: 10.8 },
  },
  routines: {},
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
    channels: { ...DEFAULT_SORTER_SETTINGS.channels, ...saved.channels },
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

/** The sorter's "machine" settings document. */
export function useMachineSettings() {
  return useServerDocument('machine', MACHINE_STORAGE_KEY, localMachineSettings, normalizeSorterSettings);
}
