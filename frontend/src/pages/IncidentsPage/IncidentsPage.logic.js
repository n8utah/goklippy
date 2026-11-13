import { useEffect, useState } from 'react';

import {
  clearSorterIncidentPictures,
  clearSorterIncidents,
  getSorterIncidentPictures,
  listSorterIncidents,
} from '@/helpers/api';
import { useSorterState } from '@/helpers/sorterState';

const WIDE = '(min-width: 768px)';

/** Whether the screen is wide enough to show incidents' cameras in the list. */
function useWide() {
  const [wide, setWide] = useState(() => window.matchMedia(WIDE).matches);
  useEffect(() => {
    const mq = window.matchMedia(WIDE);
    const on = () => setWide(mq.matches);
    mq.addEventListener('change', on);
    return () => mq.removeEventListener('change', on);
  }, []);
  return wide;
}

/**
 * Run › Incidents: every incident, each with every channel's picture and state from when
 * it opened (in the list on a wide screen, in a modal on a phone), found by number, and
 * clearing incidents or their pictures.
 */
export function useIncidentsPageLogic() {
  const [incidents, setIncidents] = useState([]);
  const [pictures, setPictures] = useState(null);
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState('');
  const [message, setMessage] = useState('');
  const [busy, setBusy] = useState(false);
  const [reload, setReload] = useState(0);
  const [openId, setOpenId] = useState(null); // shown in the modal (phones)
  const { state: sorter } = useSorterState();
  const wide = useWide();

  useEffect(() => {
    let cancelled = false;
    Promise.all([listSorterIncidents(), getSorterIncidentPictures()]).then(
      ([i, p]) => {
        if (cancelled) return;
        setIncidents(i.incidents ?? []);
        setPictures(p);
        setLoaded(true);
      },
      (err) => !cancelled && setError(err.message),
    );
    return () => {
      cancelled = true;
    };
  }, [reload]);

  const act = async (fn) => {
    setBusy(true);
    setMessage('');
    setError('');
    try {
      setMessage(await fn());
      setReload((n) => n + 1);
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  };

  return {
    incidents,
    pictures,
    loaded,
    error,
    message,
    busy,
    wide,
    /** The incident the machine is stopped for now (kept when clearing), or 0. */
    heldIncidentId: sorter?.incident?.id ?? 0,
    open: incidents.find((i) => i.id === openId) ?? null,
    setOpenId,
    refresh: () => setReload((n) => n + 1),
    clearPictures: () =>
      act(async () => {
        const { freed } = await clearSorterIncidentPictures();
        return `Removed ${freed.files.toLocaleString()} incident pictures. The incidents are kept.`;
      }),
    clearIncidents: () =>
      act(async () => {
        const { removed } = await clearSorterIncidents();
        return removed
          ? `Removed ${removed} incident${removed === 1 ? '' : 's'} and their pictures.`
          : 'No incidents to remove.';
      }),
  };
}
