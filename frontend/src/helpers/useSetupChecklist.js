import { useEffect, useState } from 'react';

import { useMCUSession } from '../context/MCUSessionContext';
import { getSorterBins, getSorterCameras } from './api';
import { useMachineSettings } from './sorterSettings';
import { useSorterState } from './sorterState';
import { sentence } from './text';

const POLL_MS = 5000;
const CAMERA_CHANNELS = ['c2', 'c3', 'c4'];

/**
 * The machine's setup, as a checklist in the order to do it (docs/ui-ux-spec.md, Machine ›
 * Overview). Each item is { id, label, state: 'done' | 'todo' | 'unknown', detail, to,
 * optional }: 'unknown' when it can't be checked yet (no board). Home shows the progress;
 * Machine › Overview shows the list.
 */
export function useSetupChecklist() {
  const { state } = useSorterState();
  const { connected: mcuConnected, stepperObjects } = useMCUSession();
  const [settings] = useMachineSettings();
  const [cameras, setCameras] = useState(null);
  const [profile, setProfile] = useState(undefined);

  useEffect(() => {
    let cancelled = false;
    const load = () => {
      getSorterCameras().then(
        (c) => !cancelled && setCameras(c),
        () => {},
      );
      getSorterBins().then(
        (b) => !cancelled && setProfile(b.profile ?? null),
        () => {},
      );
    };
    load();
    const timer = setInterval(load, POLL_MS);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, []);

  const board = Boolean(state?.hardware?.connected);
  const items = [];

  items.push({
    id: 'board',
    label: 'Board connected',
    state: board ? 'done' : 'todo',
    detail: board
      ? `${state.hardware.simulated ? 'Simulated' : 'On'} ${state.hardware.port ?? ''}`.trim()
      : sentence(state?.hardware?.message) || 'Connect the RAMPS board by USB.',
    to: '/machine/board/connection',
  });

  // Every motor the settings name is in the connected config.
  const wanted = [
    ...['c1', 'c2', 'c3', 'c4'].map((c) => [c.toUpperCase(), settings.channels?.[c]?.stepper]),
    ['the chute', settings.distributor?.stepper],
  ].filter(([, s]) => s);
  const missing = wanted.filter(([, s]) => !stepperObjects.some((o) => o.section === s));
  items.push({
    id: 'motors',
    label: 'Motors wired',
    state: !mcuConnected ? 'unknown' : missing.length === 0 ? 'done' : 'todo',
    detail: !mcuConnected
      ? 'Checked once the board is connected.'
      : missing.length === 0
        ? `C1–C4 and the chute: ${wanted.map(([, s]) => s).join(', ')}`
        : `Not in the connected config: ${missing.map(([n, s]) => `${n} (${s})`).join(', ')}.`,
    to: missing.length ? '/machine/board/wiring' : '/machine/channels',
  });

  const chute = state?.chute;
  items.push({
    id: 'chute',
    label: 'Chute homed',
    state: !board ? 'unknown' : chute?.homed ? 'done' : 'todo',
    detail: !board
      ? 'Checked once the board is connected.'
      : chute?.left_out
        ? 'Left out for now: sorting runs without it. Untick “Leave the chute out” on Machine › Chute once it’s wired.'
        : chute?.homed
          ? `At ${chute.angle?.toFixed(1)}°. Calibrate its aim under Chute › Calibrate.`
          : sentence(chute?.message) || 'Home the machine to find the chute’s 0°.',
    to: '/machine/chute',
  });

  const camStatus = (c) => cameras?.channels?.[c];
  const camDone = CAMERA_CHANNELS.filter((c) => {
    const st = camStatus(c);
    return (
      settings.cameras?.devices?.[c] && st?.camera?.running && st?.zones_set && st?.has_background
    );
  });
  const camNext = CAMERA_CHANNELS.find((c) => !camDone.includes(c));
  items.push({
    id: 'cameras',
    label: 'Cameras set up',
    state: !cameras ? 'unknown' : camDone.length === CAMERA_CHANNELS.length ? 'done' : 'todo',
    detail:
      camDone.length === CAMERA_CHANNELS.length
        ? 'C2, C3 and C4: camera, zones and empty picture.'
        : `${camDone.length} of 3 ready (camera running, zones drawn, empty picture). Next: ${camNext.toUpperCase()}.`,
    to:
      camNext && settings.cameras?.devices?.[camNext]
        ? `/machine/cameras/${camNext}`
        : '/machine/cameras',
  });

  const lights = state?.lights;
  items.push({
    id: 'light',
    label: 'Light switches',
    state: !board ? 'unknown' : lights && !lights.message ? 'done' : 'todo',
    detail: !board
      ? 'Checked once the board is connected.'
      : lights?.message
        ? sentence(lights.message)
        : `On ${settings.lights?.output}; switch it from the status panel.`,
    to: '/system/settings/light',
  });

  const supply = state?.supply;
  items.push({
    id: 'power',
    label: 'Motor power',
    optional: true,
    state: !board ? 'unknown' : supply?.known ? (supply.low ? 'todo' : 'done') : 'unknown',
    detail: supply?.known
      ? `${supply.volts.toFixed(1)} V${supply.low ? `, below ${supply.min_volts} V` : ''}`
      : 'Not measured. Optional: a voltage divider to an analog pin holds sorting back when power is low.',
    to: '/system/settings/power',
  });

  items.push({
    id: 'profile',
    label: 'Sorting profile active',
    state: profile === undefined ? 'unknown' : profile ? 'done' : 'todo',
    // Not needed while the chute is left out: sorting doesn't use one then.
    optional: Boolean(chute?.left_out),
    detail: chute?.left_out
      ? 'Not used while the chute is left out.'
      : profile
        ? profile.name
        : 'Upload a Hive profile and activate it.',
    to: '/rules/profiles',
  });

  const required = items.filter((i) => !i.optional);
  return {
    items,
    done: required.filter((i) => i.state === 'done').length,
    total: required.length,
    next: required.find((i) => i.state !== 'done') ?? null,
  };
}
