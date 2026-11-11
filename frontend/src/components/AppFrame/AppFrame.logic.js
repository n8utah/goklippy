import { useCallback, useEffect, useState, useSyncExternalStore } from 'react';
import { useLocation } from 'react-router';
import { toast } from 'sonner';

import { useMCUSession } from '../../context/MCUSessionContext';
import { cancelSorterJob, setSorterLights, sorterCommand } from '../../helpers/api';
import { findLocation } from '../../helpers/navigation';
import { useSorterState } from '../../helpers/sorterState';

/** True while the media query matches, following changes. */
function useMediaQuery(query) {
  return useSyncExternalStore(
    (onChange) => {
      const mq = window.matchMedia(query);
      mq.addEventListener('change', onChange);
      return () => mq.removeEventListener('change', onChange);
    },
    () => window.matchMedia(query).matches,
  );
}

/**
 * The status pill's words and color. `tone` is one of running, ready, warning, error,
 * idle; the dot and text both carry it so color is never the only signal.
 */
export function describeStatus(state, connected) {
  if (!connected) return { label: 'No backend', tone: 'error' };
  if (!state) return { label: 'Connecting', tone: 'idle' };
  const run = state.run_id ? ` · run ${state.run_id}` : '';
  if (state.state === 'error') return { label: 'Error', tone: 'error' };
  if (state.incident) return { label: 'Needs attention', tone: 'warning' };
  if (state.supply?.low) return { label: 'Low motor power', tone: 'warning' };
  switch (state.state) {
    case 'running':
      return { label: `Sorting${run}`, tone: 'running' };
    case 'paused':
      return { label: `Paused${run}`, tone: 'warning' };
    case 'homing':
      return { label: 'Homing', tone: 'running' };
    case 'ready':
      return { label: state.job ? state.job.name : 'Ready', tone: 'ready' };
    default:
      if (!state.hardware?.connected) return { label: 'No board', tone: 'idle' };
      return { label: state.job ? state.job.name : 'Standby', tone: 'idle' };
  }
}

/** The running task's progress, 0–1, or null when it has no steps to count. */
export function jobProgress(job) {
  if (!job || !job.total) return null;
  return Math.min(1, Math.max(0, job.step / job.total));
}

export function useAppFrameLogic() {
  const location = useLocation();
  const { group, page, part } = findLocation(location.pathname);
  const { state, connected } = useSorterState();
  const mcu = useMCUSession();
  const [busy, setBusy] = useState(false);
  const [searchOpen, setSearchOpen] = useState(false);
  const [statusOpen, setStatusOpen] = useState(false);

  const isDesktop = useMediaQuery('(min-width: 1024px)');
  const isPhone = useMediaQuery('(max-width: 639px)');

  // Ctrl+K (⌘K on a Mac) opens search from anywhere.
  useEffect(() => {
    const onKey = (e) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault();
        setSearchOpen((open) => !open);
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, []);

  // A new page closes the panels that led there.
  useEffect(() => {
    setStatusOpen(false);
    setSearchOpen(false);
  }, [location.pathname]);

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
  const job = state?.job || null;
  const sorting = s === 'running' || s === 'paused';
  const handMoving =
    Object.keys(mcu.stepperJogState).length > 0 || Object.keys(mcu.stepperRoutineOids).length > 0;

  /**
   * Stop everything that moves, in one click: the run (which also cancels its task), else
   * the running task (homing, a capture), and any motor turned by hand from Board or the
   * bench. Each is tried even if another fails.
   */
  const stop = () =>
    run(async () => {
      const tries = [];
      if (sorting) tries.push(sorterCommand('stop'));
      else if (job) tries.push(cancelSorterJob());
      if (handMoving) tries.push(mcu.stopAllSteppers());
      const failed = (await Promise.allSettled(tries)).filter((r) => r.status === 'rejected');
      if (failed.length) throw failed[0].reason;
      toast('Stopped');
    });

  const command = (name) => run(() => sorterCommand(name));
  const cancelJob = () => run(() => cancelSorterJob());
  const toggleLights = () => run(() => setSorterLights(!state?.lights?.on));

  return {
    group,
    page,
    part,
    state,
    connected,
    status: describeStatus(state, connected),
    progress: jobProgress(job),
    job,
    busy,
    isDesktop,
    isPhone,
    searchOpen,
    setSearchOpen,
    statusOpen,
    setStatusOpen,
    stop,
    canStop: !busy && (sorting || Boolean(job) || handMoving),
    command,
    cancelJob,
    toggleLights,
    canHome: !busy && s && s !== 'running' && s !== 'homing' && !job,
    canStart: !busy && s === 'ready' && !job,
    canPause: !busy && s === 'running',
    canResume: !busy && s === 'paused' && !job,
  };
}
