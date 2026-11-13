import { useCallback, useEffect, useState } from 'react';
import { toast } from 'sonner';

import {
  clearSorterChannels,
  forgetSorterIgnored,
  getSorterBins,
  getSorterCameras,
  listSorterRuns,
  listSorterShakes,
  moveSorterChannel,
  keepSorterFeeding,
  resolveSorterIncident,
  shakeSorterChannel,
  sorterCommand,
} from '../../helpers/api';
import { perMinute, sortingMs } from '../../helpers/sorterRuns';
import { useMachineSettings } from '../../helpers/sorterSettings';
import { useSorterState } from '../../helpers/sorterState';

const NUMBERS_POLL_MS = 5000;

export const CAMERA_CHANNELS = [
  { id: 'c2', label: 'C2', name: 'C-channel 2' },
  { id: 'c3', label: 'C3', name: 'C-channel 3' },
  { id: 'c4', label: 'C4', name: 'Classification' },
];

/** The run's numbers and the bins, refreshed every few seconds. finishing: the run is
 * finishing on its own now, so its sorting ended with its last piece. */
function useRunNumbers(runId, finishing) {
  const [runs, setRuns] = useState([]);
  const [bins, setBins] = useState(null);

  useEffect(() => {
    let cancelled = false;
    const load = () =>
      Promise.all([listSorterRuns(), getSorterBins()]).then(
        ([r, b]) => {
          if (cancelled) return;
          setRuns(r.runs ?? []);
          setBins(b);
        },
        () => {},
      );
    load();
    const timer = setInterval(load, NUMBERS_POLL_MS);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, [runId]);

  // This run while sorting, else the last one.
  const run = (runId && runs.find((r) => r.id === runId)) || runs[0] || null;
  const current = Boolean(runId && run?.id === runId);
  const allBins = (bins?.layers ?? [])
    .filter((l) => !l.disabled)
    .flatMap((l) => l.sections.filter((s) => !s.disabled).flatMap((s) => s.bins));

  return {
    run,
    current,
    perMinute: perMinute(run, current && finishing),
    /** How long it really sorted (to its last piece once done or finishing). */
    sortingMs: sortingMs(run, current && finishing),
    binsFull: allBins.filter((b) => b.full).length,
    binsTotal: allBins.length,
    profile: bins?.profile ?? null,
  };
}

const CAMERAS_POLL_MS = 3000;

/** Each channel's camera: its device and, when it isn't seeing, why. */
function useCameraStatuses() {
  const [cameras, setCameras] = useState({});
  useEffect(() => {
    let cancelled = false;
    const load = () =>
      getSorterCameras().then(
        (data) => !cancelled && setCameras(data.channels ?? {}),
        () => {},
      );
    load();
    const timer = setInterval(load, CAMERAS_POLL_MS);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, []);
  return cameras;
}

export function useSortPageLogic() {
  const cameras = useCameraStatuses();
  const { state, connected } = useSorterState();
  const [settings] = useMachineSettings();
  const [busy, setBusy] = useState(false);
  const [shakes, setShakes] = useState([]);
  const [fullscreen, setFullscreen] = useState(null); // the camera channel shown full screen
  const numbers = useRunNumbers(state?.run_id || 0, Boolean(state?.finish && !state.finish.done));

  useEffect(() => {
    let cancelled = false;
    listSorterShakes().then(
      (data) => !cancelled && setShakes(data.shakes),
      () => {},
    );
    return () => {
      cancelled = true;
    };
  }, []);

  const run = useCallback(async (fn) => {
    setBusy(true);
    try {
      await fn();
    } catch (err) {
      toast.error(err.message);
    } finally {
      setBusy(false);
    }
  }, []);

  const s = state?.state;
  const job = state?.job;
  return {
    state,
    connected,
    busy,
    numbers,
    shakes,
    cameras,
    /** The channel whose camera fills the screen, or null. */
    fullscreen: CAMERA_CHANNELS.find((ch) => ch.id === fullscreen) ?? null,
    openFullscreen: setFullscreen,
    closeFullscreen: () => setFullscreen(null),
    command: (name) => run(() => sorterCommand(name)),
    resolve: (action) => run(() => resolveSorterIncident(action)),
    /** Calls off the run finishing on its own, for a refilled hopper. */
    keepFeeding: () => run(() => keepSorterFeeding()),
    /** Turn by hand at the channel's own speed. */
    turn: (channel, degrees) => {
      const rpm = channel === 'c4' ? settings.indexRpm : settings.feederRpm?.[channel] || 5;
      return run(() => moveSorterChannel(channel, degrees, rpm));
    },
    shake: (channel, profile) => run(() => shakeSorterChannel(channel, profile)),
    /** Run C2–C4 for the set time to empty them; with the chute in, into the bucket. */
    clear: () => run(() => clearSorterChannels()),
    /** The stuck piece was taken off: the channel's camera watches its spot again. */
    forgetIgnored: (channel) => run(() => forgetSorterIgnored(channel)),
    clearSeconds: settings.feeder?.clearSeconds ?? 30,
    clearRpm: settings.feeder?.clearRpm ?? 12,
    chuteLeftOut: Boolean(settings.distributor?.leftOut),
    sorting: s === 'running',
    handsFree: Boolean(state?.hardware?.connected) && s !== 'running' && s !== 'homing' && !job,
    handsBlockedWhy: !state?.hardware?.connected
      ? 'Connect the board first'
      : s === 'running'
        ? 'Pause or stop the sort first'
        : job || s === 'homing'
          ? 'Wait for the task to finish'
          : '',
    canHome: !busy && s && s !== 'running' && s !== 'homing' && !job,
    canStart: !busy && s === 'ready' && !job,
    canPause: !busy && s === 'running',
    canResume: !busy && s === 'paused' && !job,
  };
}
