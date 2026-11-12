import { useEffect, useState } from 'react';

import {
  getSorterStats,
  listSorterIncidents,
  listSorterPieces,
  listSorterRuns,
} from '../../helpers/api';

export const PAGE_SIZE = 50;

/**
 * Run › Records: pieces (filtered and paged), runs, incidents and lifetime totals. A
 * piece opens in the journey modal. `initialRun` filters pieces to one run (?run=).
 */
export function useRecordsPageLogic(initialRun) {
  const [filters, setFilters] = useState({ run: initialRun || '', status: '', q: '' });
  const [offset, setOffset] = useState(0);
  const [pieces, setPieces] = useState({ list: [], total: 0, loading: true });
  const [runs, setRuns] = useState([]);
  const [incidents, setIncidents] = useState([]);
  const [stats, setStats] = useState(null);
  const [error, setError] = useState('');
  const [reload, setReload] = useState(0);

  // A link to another run's pieces while already here (from a journey) moves the filter.
  const [linkedRun, setLinkedRun] = useState(initialRun);
  if (initialRun !== linkedRun) {
    setLinkedRun(initialRun);
    if (initialRun) {
      setFilters({ run: initialRun, status: '', q: '' });
      setOffset(0);
    }
  }

  useEffect(() => {
    let cancelled = false;
    listSorterPieces({ ...filters, limit: PAGE_SIZE, offset }).then(
      ({ pieces: list, total }) => !cancelled && setPieces({ list, total, loading: false }),
      (err) => !cancelled && setError(err.message),
    );
    return () => {
      cancelled = true;
    };
  }, [filters, offset, reload]);

  useEffect(() => {
    let cancelled = false;
    Promise.all([listSorterRuns(), listSorterIncidents(), getSorterStats()]).then(
      ([r, i, s]) => {
        if (cancelled) return;
        setRuns(r.runs ?? []);
        setIncidents(i.incidents ?? []);
        setStats(s);
      },
      (err) => !cancelled && setError(err.message),
    );
    return () => {
      cancelled = true;
    };
  }, [reload]);

  const setFilter = (key, value) => {
    setFilters((prev) => ({ ...prev, [key]: value }));
    setOffset(0);
  };

  return {
    filters,
    setFilter,
    /** Shows one run's pieces. */
    showRun: (runId) => {
      setFilters({ run: String(runId), status: '', q: '' });
      setOffset(0);
    },
    offset,
    setOffset,
    pieces,
    runs,
    incidents,
    stats,
    refresh: () => setReload((n) => n + 1),
    error,
  };
}
