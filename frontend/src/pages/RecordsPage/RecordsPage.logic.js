import { useEffect, useState } from 'react';

import {
  clearSorterIncidentPictures,
  clearSorterIncidents,
  getSorterIncidentPictures,
  getSorterStats,
  listSorterIncidents,
  listSorterRuns,
} from '../../helpers/api';
import { useSorterState } from '../../helpers/sorterState';

/**
 * Run › Records: runs, incidents (each with every channel's picture and state from when
 * it opened) and lifetime totals, and clearing incidents or their pictures. (Pieces have
 * their own group, Pieces.)
 */
export function useRecordsPageLogic() {
  const [runs, setRuns] = useState([]);
  const [incidents, setIncidents] = useState([]);
  const [stats, setStats] = useState(null);
  const [pictures, setPictures] = useState(null);
  const [error, setError] = useState('');
  const [message, setMessage] = useState('');
  const [busy, setBusy] = useState(false);
  const [reload, setReload] = useState(0);
  const { state: sorter } = useSorterState();

  useEffect(() => {
    let cancelled = false;
    Promise.all([
      listSorterRuns(),
      listSorterIncidents(),
      getSorterStats(),
      getSorterIncidentPictures(),
    ]).then(
      ([r, i, s, p]) => {
        if (cancelled) return;
        setRuns(r.runs ?? []);
        setIncidents(i.incidents ?? []);
        setStats(s);
        setPictures(p);
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
    /** The incident the machine is stopped for now (kept when clearing), or 0. */
    heldIncidentId: sorter?.incident?.id ?? 0,
    runs,
    incidents,
    stats,
    pictures,
    error,
    message,
    busy,
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
