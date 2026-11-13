import { useCallback, useEffect, useState, useSyncExternalStore } from 'react';
import { useLocation, useNavigate } from 'react-router';
import { toast } from 'sonner';

import { useMCUSession } from '../../context/MCUSessionContext';
import {
  blinkSorterLights,
  cancelSorterJob,
  getMCUPorts,
  setSorterLights,
  sorterCommand,
} from '../../helpers/api';
import { pickBoardPort, rememberBoardPort } from '../../helpers/boardPort';
import { BOARD_PROFILE_ID } from '../../helpers/useKlipperWorkspace';
import { findLocation } from '../../helpers/navigation';
import { pushRecent } from '../../helpers/recent';
import { useSorterState } from '../../helpers/sorterState';
import { useTheme } from '../../helpers/theme';

/** True when a key press is going into a text field. */
const isTyping = (el) =>
  el?.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(el?.tagName);

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
  const navigate = useNavigate();
  const { resolved: resolvedTheme, setTheme } = useTheme();
  const { group, page, part } = findLocation(location.pathname);
  const { state, connected } = useSorterState();
  const mcu = useMCUSession();
  const [busy, setBusy] = useState(false);
  const [searchOpen, setSearchOpen] = useState(false);
  const [statusOpen, setStatusOpen] = useState(false);

  const isDesktop = useMediaQuery('(min-width: 1024px)');
  const isPhone = useMediaQuery('(max-width: 639px)');

  // Ctrl+K (⌘K on a Mac) opens search from anywhere, and so does "/" when not typing.
  useEffect(() => {
    const onKey = (e) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault();
        setSearchOpen((open) => !open);
      } else if (e.key === '/' && !e.ctrlKey && !e.metaKey && !e.altKey && !isTyping(e.target)) {
        e.preventDefault();
        setSearchOpen(true);
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, []);

  // A new page closes the panels that led there, and goes on search's recent list.
  const pagePath = page?.path;
  useEffect(() => {
    setStatusOpen(false);
    setSearchOpen(false);
    if (pagePath) pushRecent('page', location.pathname, pagePath);
  }, [location.pathname, pagePath]);

  // The quick editor's setting, or null.
  const [editKey, setEditKey] = useState(null);
  const openSetting = useCallback((key) => {
    pushRecent('setting', key);
    setSearchOpen(false);
    setEditKey(key);
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

  const canHome = !busy && s && s !== 'running' && s !== 'homing' && !job;
  const canStart = !busy && s === 'ready' && !job;
  const canPause = !busy && s === 'running';
  const canResume = !busy && s === 'paused' && !job;
  const canStop = !busy && (sorting || Boolean(job) || handMoving);
  const lightsOK = connected && !state?.lights?.message;
  const boardConnected = Boolean(state?.hardware?.connected);
  // The top bar's Connect: shown while the backend answers but has no board.
  const canConnect = connected && !boardConnected && !mcu.connected;

  /**
   * Connects the board in one click: on the port it was last connected on, else the only
   * one found. When it isn't clear which port, Machine › Board lists them to choose.
   */
  const connectBoard = async () => {
    if (mcu.connecting) return;
    let port;
    try {
      port = pickBoardPort((await getMCUPorts()).ports || []);
    } catch (err) {
      toast.error(err.message);
      return;
    }
    if (!port) {
      toast('Choose the board’s port');
      navigate('/machine/board/connection');
      return;
    }
    const id = toast.loading(`Connecting to ${port}…`);
    try {
      await mcu.connect(port, { useWorkspace: true, profileId: BOARD_PROFILE_ID });
      rememberBoardPort(port);
      toast.success(`Connected to ${port}`, { id });
    } catch (err) {
      toast.error(`Couldn’t connect to ${port}: ${err.message}`, {
        id,
        action: { label: 'Board', onClick: () => navigate('/machine/board/connection') },
      });
    }
  };

  /**
   * What search can do, in order of how likely it's wanted: the first allowed ones are the
   * suggestions when search is empty. `moves` says it moves the machine; `why` is shown
   * when it isn't allowed now.
   */
  const actions = [
    job && {
      id: 'cancel-task',
      label: `Cancel ${job.name}`,
      keywords: 'cancel task job abort',
      allowed: !busy,
      run: cancelJob,
    },
    !boardConnected && {
      id: 'connect-board',
      label: 'Connect the board',
      keywords: 'board serial port usb ramps',
      allowed: canConnect && !mcu.connecting,
      why: mcu.connecting ? 'Connecting now' : 'No backend',
      run: connectBoard,
    },
    {
      id: 'start',
      label: 'Start sorting',
      keywords: 'run go begin',
      moves: true,
      allowed: canStart,
      why: s === 'standby' ? 'Home first' : 'Only when ready',
      run: () => command('start'),
    },
    {
      id: 'resume',
      label: 'Resume sorting',
      keywords: 'continue unpause',
      moves: true,
      allowed: canResume,
      why: 'Only when paused',
      run: () => command('resume'),
    },
    {
      id: 'pause',
      label: 'Pause sorting',
      keywords: 'hold wait',
      allowed: canPause,
      why: 'Only while sorting',
      run: () => command('pause'),
    },
    {
      id: 'stop',
      label: 'Stop everything',
      keywords: 'halt emergency end run',
      allowed: canStop,
      why: 'Nothing is moving',
      run: stop,
    },
    {
      id: 'home',
      label: 'Home the machine',
      keywords: 'chute endstop start hardware',
      moves: true,
      allowed: canHome,
      why: 'Not while sorting or busy',
      run: () => command('home'),
    },
    {
      id: 'light',
      label: state?.lights?.on ? 'Turn the light off' : 'Turn the light on',
      keywords: 'lamp leds d10 switch',
      allowed: !busy && lightsOK,
      why: state?.lights?.message || 'No backend',
      run: toggleLights,
    },
    {
      id: 'blink',
      label: 'Blink the light for 30 s',
      keywords: 'lamp leds d10 test wiring flash',
      allowed: !busy && lightsOK && !sorting,
      why: sorting ? 'Not while sorting' : state?.lights?.message || 'No backend',
      run: () => run(() => blinkSorterLights(30)),
    },
    {
      id: 'theme',
      label: resolvedTheme === 'dark' ? 'Switch to the light theme' : 'Switch to the dark theme',
      keywords: 'dark mode light mode appearance colors',
      allowed: true,
      run: () => setTheme(resolvedTheme === 'dark' ? 'light' : 'dark'),
    },
  ].filter(Boolean);

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
    canStop,
    canConnect,
    connecting: mcu.connecting,
    connectBoard,
    command,
    cancelJob,
    toggleLights,
    canHome,
    canStart,
    canPause,
    canResume,
    actions,
    editKey,
    openSetting,
    closeSetting: () => setEditKey(null),
  };
}
