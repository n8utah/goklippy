import { useCallback, useEffect, useState } from 'react';

import { listSorterShakes, moveSorterChannel, resolveSorterIncident, shakeSorterChannel, sorterCommand } from '../../helpers/api';
import { useMachineSettings } from '../../helpers/sorterSettings';
import { useSorterState } from '../../helpers/sorterState';

export const STATE_LABELS = {
  standby: 'Standby',
  homing: 'Homing',
  ready: 'Ready',
  running: 'Sorting',
  paused: 'Paused',
  error: 'Error',
};

export const CHANNEL_NAMES = { c2: 'C-channel 2', c3: 'C-channel 3', c4: 'Classification' };

export function useDashboardPageLogic() {
  const { state, connected } = useSorterState();
  const [settings] = useMachineSettings();
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const [shakes, setShakes] = useState([]);

  useEffect(() => {
    let cancelled = false;
    listSorterShakes().then(
      (data) => {
        if (!cancelled) setShakes(data.shakes);
      },
      () => {},
    );
    return () => {
      cancelled = true;
    };
  }, []);

  const run = useCallback(async (fn) => {
    setError('');
    setBusy(true);
    try {
      await fn();
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  }, []);

  const command = (name) => run(() => sorterCommand(name));
  const resolve = (action) => run(() => resolveSorterIncident(action));

  /** Turn by hand at the channel's own speed. */
  const turn = (channel, degrees) => {
    const rpm = channel === 'c4' ? settings.indexRpm : settings.feederRpm?.[channel] || 5;
    return run(() => moveSorterChannel(channel, degrees, rpm));
  };
  const shake = (channel, profile) => run(() => shakeSorterChannel(channel, profile));

  const s = state?.state;
  return {
    state,
    connected,
    error,
    busy,
    command,
    resolve,
    turn,
    shake,
    shakes,
    handsFree: Boolean(state?.hardware?.connected) && s !== 'running' && s !== 'homing' && !state?.job,
    canHome: s && s !== 'running' && s !== 'homing' && !state.job,
    canStart: s === 'ready' && !state.job,
    canPause: s === 'running',
    canResume: s === 'paused' && !state.job,
    canStop: s === 'running' || s === 'paused',
  };
}
