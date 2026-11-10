import { useCallback, useState } from 'react';

import { resolveSorterIncident, sorterCommand } from '../../helpers/api';
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
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

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

  const s = state?.state;
  return {
    state,
    connected,
    error,
    busy,
    command,
    resolve,
    canHome: s && s !== 'running' && s !== 'homing' && !state.job,
    canStart: s === 'ready' && !state.job,
    canPause: s === 'running',
    canResume: s === 'paused' && !state.job,
    canStop: s === 'running' || s === 'paused',
  };
}
