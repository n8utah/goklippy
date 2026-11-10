import { useEffect, useState } from 'react';

import { getSorterPiece, getSorterStats, listSorterIncidents, listSorterPieces, listSorterRuns } from '../../helpers/api';

export const PAGE_SIZE = 50;

export function useRecordsPageLogic(initialPieceId) {
  const [tab, setTab] = useState('pieces');
  const [filters, setFilters] = useState({ run: '', status: '', q: '' });
  const [offset, setOffset] = useState(0);
  const [pieces, setPieces] = useState({ list: [], total: 0 });
  const [runs, setRuns] = useState([]);
  const [incidents, setIncidents] = useState([]);
  const [stats, setStats] = useState(null);
  const [pieceId, setPieceId] = useState(initialPieceId || null);
  const [piece, setPiece] = useState(null);
  const [error, setError] = useState('');
  const [reload, setReload] = useState(0);

  useEffect(() => {
    let cancelled = false;
    const done = (fn) => (data) => {
      if (!cancelled) fn(data);
    };
    const fail = (err) => {
      if (!cancelled) setError(err.message);
    };
    listSorterPieces({ ...filters, limit: PAGE_SIZE, offset }).then(
      done(({ pieces: list, total }) => setPieces({ list, total })),
      fail,
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
        setRuns(r.runs);
        setIncidents(i.incidents);
        setStats(s);
      },
      (err) => {
        if (!cancelled) setError(err.message);
      },
    );
    return () => {
      cancelled = true;
    };
  }, [reload]);

  useEffect(() => {
    if (!pieceId) return undefined;
    let cancelled = false;
    getSorterPiece(pieceId).then(
      (p) => {
        if (!cancelled) setPiece(p);
      },
      (err) => {
        if (!cancelled) setError(err.message);
      },
    );
    return () => {
      cancelled = true;
    };
  }, [pieceId]);

  const setFilter = (key, value) => {
    setFilters((prev) => ({ ...prev, [key]: value }));
    setOffset(0);
  };

  const showRun = (runId) => {
    setFilters({ run: String(runId), status: '', q: '' });
    setOffset(0);
    setTab('pieces');
  };

  const openPiece = (id) => {
    setPiece(null);
    setPieceId(id);
  };

  return {
    tab,
    setTab,
    filters,
    setFilter,
    offset,
    setOffset,
    pieces,
    runs,
    incidents,
    stats,
    piece: pieceId ? piece : null,
    pieceId,
    openPiece,
    closePiece: () => setPieceId(null),
    showRun,
    refresh: () => setReload((n) => n + 1),
    error,
  };
}
