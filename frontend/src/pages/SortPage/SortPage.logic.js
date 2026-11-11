import { useCallback, useEffect, useState } from 'react';
import { toast } from 'sonner';

import {
  getSorterBins,
  listSorterRuns,
  listSorterShakes,
  moveSorterChannel,
  resolveSorterIncident,
  shakeSorterChannel,
  sorterCommand,
} from '../../helpers/api';
import { useMachineSettings } from '../../helpers/sorterSettings';
import { useSorterState } from '../../helpers/sorterState';

const NUMBERS_POLL_MS = 5000;

export const CAMERA_CHANNELS = [
  { id: 'c2', label: 'C2', name: 'C-channel 2' },
  { id: 'c3', label: 'C3', name: 'C-channel 3' },
  { id: 'c4', label: 'C4', name: 'Classification' },
];

/** The run's numbers and the bins, refreshed every few seconds. */
function useRunNumbers(runId) {
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
  const minutes = run
    ? Math.max(
        1 / 60,
        ((run.ended_at ? new Date(run.ended_at) : new Date()) - new Date(run.started_at)) / 60000,
      )
    : 0;
  const allBins = (bins?.layers ?? [])
    .filter((l) => !l.disabled)
    .flatMap((l) => l.sections.filter((s) => !s.disabled).flatMap((s) => s.bins));

  return {
    run,
    current: Boolean(runId && run?.id === runId),
    perMinute: run ? run.distributed / minutes : 0,
    binsFull: allBins.filter((b) => b.full).length,
    binsTotal: allBins.length,
    profile: bins?.profile ?? null,
  };
}

export function useSortPageLogic() {
  const { state, connected } = useSorterState();
  const [settings] = useMachineSettings();
  const [busy, setBusy] = useState(false);
  const [shakes, setShakes] = useState([]);
  const numbers = useRunNumbers(state?.run_id || 0);

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
    command: (name) => run(() => sorterCommand(name)),
    resolve: (action) => run(() => resolveSorterIncident(action)),
    /** Turn by hand at the channel's own speed. */
    turn: (channel, degrees) => {
      const rpm = channel === 'c4' ? settings.indexRpm : settings.feederRpm?.[channel] || 5;
      return run(() => moveSorterChannel(channel, degrees, rpm));
    },
    shake: (channel, profile) => run(() => shakeSorterChannel(channel, profile)),
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
