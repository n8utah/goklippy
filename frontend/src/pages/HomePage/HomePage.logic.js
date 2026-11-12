import { useCallback, useEffect, useState } from 'react';
import { toast } from 'sonner';

import { getSorterBins, listSorterIncidents, sorterCommand } from '../../helpers/api';
import { useMachineSettings } from '../../helpers/sorterSettings';
import { useSorterState } from '../../helpers/sorterState';
import { sentence } from '../../helpers/text';
import { useSetupChecklist } from '../../helpers/useSetupChecklist';

const POLL_MS = 10000;

const RECENT_PROBLEMS = 4;

/**
 * What stands between the machine and sorting, in the order to fix them: each
 * { id, text, to, link } where `to` is the page that fixes it.
 */
export function blockers({ state, connected, profile, cameras }) {
  const list = [];
  if (!connected) {
    list.push({ id: 'backend', text: 'The backend isn’t answering. Is goklippy running?' });
    return list;
  }
  if (!state) return list;
  if (!state.hardware?.connected)
    list.push({
      id: 'board',
      text: sentence(state.hardware?.message) || 'No board is connected.',
      to: '/machine/board',
      link: 'Connect the board',
    });
  if (state.supply?.low)
    list.push({
      id: 'power',
      text: sentence(state.supply.message) || 'Motor power is low.',
      to: '/system/settings/power',
      link: 'Motor power settings',
    });
  if (!profile)
    list.push({
      id: 'profile',
      text: 'No sorting profile is active, so every piece would go to misc.',
      to: '/rules/profiles',
      link: 'Choose a profile',
    });
  const watched = ['c2', 'c3', 'c4'].filter((c) => cameras?.devices?.[c]);
  if (watched.length < 3)
    list.push({
      id: 'cameras',
      text:
        watched.length === 0
          ? 'No camera watches a channel yet.'
          : `Only ${watched.map((c) => c.toUpperCase()).join(' and ')} ${watched.length === 1 ? 'has' : 'have'} a camera.`,
      to: '/machine/cameras',
      link: 'Set up the cameras',
    });
  if (state.state === 'error' && state.error)
    list.push({ id: 'error', text: sentence(state.error) });
  return list;
}

export function useHomePageLogic() {
  const { state, connected } = useSorterState();
  const [settings] = useMachineSettings();
  const [profile, setProfile] = useState(undefined); // undefined while loading
  const [incidents, setIncidents] = useState([]);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let cancelled = false;
    const load = () => {
      getSorterBins().then(
        (b) => !cancelled && setProfile(b.profile ?? null),
        () => {},
      );
      listSorterIncidents().then(
        (i) => !cancelled && setIncidents((i.incidents ?? []).slice(0, RECENT_PROBLEMS)),
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

  const command = useCallback(async (name) => {
    setBusy(true);
    try {
      await sorterCommand(name);
    } catch (err) {
      toast.error(err.message);
    } finally {
      setBusy(false);
    }
  }, []);

  const setup = useSetupChecklist();
  const s = state?.state;
  const job = state?.job;
  return {
    setup,
    state,
    connected,
    profile,
    incidents,
    blockers:
      profile === undefined
        ? []
        : blockers({ state, connected, profile, cameras: settings.cameras }),
    busy,
    command,
    canHome: !busy && s && s !== 'running' && s !== 'homing' && !job && state?.hardware?.connected,
    canStart: !busy && s === 'ready' && !job,
    canPause: !busy && s === 'running',
    canResume: !busy && s === 'paused' && !job,
  };
}
