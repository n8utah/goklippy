import { useEffect, useRef, useState } from 'react';

import { getCollection, getPlan, listParts, listPieces, watchCollection } from '@/helpers/api';

export const TABS = [
  { id: 'live', label: 'Live' },
  { id: 'plan', label: 'Plan' },
  { id: 'parts', label: 'Parts' },
  { id: 'unknowns', label: 'Unknowns' },
];

const LIVE_SHOWN = 40;
// Pushes come in bursts while sorting: refetch at most this often.
const SETTLE_MS = 1500;

/**
 * One collection, kept current: the sorter's pushes arrive as server-sent events. A new
 * piece goes straight onto the live list; everything else refetches once things settle.
 */
export function useCollectionPage(id, tab) {
  const [collection, setCollection] = useState(null);
  const [error, setError] = useState('');
  const [live, setLive] = useState([]);
  const [plan, setPlan] = useState(null);
  const [parts, setParts] = useState(null);
  const [unknowns, setUnknowns] = useState(null);
  const [version, setVersion] = useState(0); // bumps when something changed
  const [connected, setConnected] = useState(false);
  const settle = useRef(null);

  // Events: new pieces now, the rest after a moment.
  useEffect(() => {
    const changed = () => {
      clearTimeout(settle.current);
      settle.current = setTimeout(() => setVersion((v) => v + 1), SETTLE_MS);
    };
    const stop = watchCollection(id, (e) => {
      setConnected(true);
      if (e.type === 'piece' && e.piece) {
        setLive((list) =>
          [e.piece, ...list.filter((p) => p.id !== e.piece.id)].slice(0, LIVE_SHOWN),
        );
      }
      if (e.type === 'unknowns' || e.type === 'photos') {
        setLive([]); // refetched below
      }
      changed();
    });
    return () => {
      stop();
      clearTimeout(settle.current);
    };
  }, [id]);

  // The collection's totals.
  useEffect(() => {
    let cancelled = false;
    getCollection(id).then(
      (c) => {
        if (cancelled) return;
        setCollection(c);
        setError('');
      },
      (err) => !cancelled && setError(err.status === 404 ? 'notfound' : err.message),
    );
    return () => {
      cancelled = true;
    };
  }, [id, version]);

  // The open tab's data.
  useEffect(() => {
    let cancelled = false;
    const done = (set) => (data) => !cancelled && set(data);
    if (tab === 'live') {
      listPieces(id, { limit: LIVE_SHOWN }).then(
        done((d) => setLive(d.pieces ?? [])),
        () => {},
      );
    } else if (tab === 'plan') {
      getPlan(id).then(done(setPlan), () => {});
    } else if (tab === 'parts') {
      listParts(id).then(
        done((d) => setParts(d.parts ?? [])),
        () => {},
      );
    } else if (tab === 'unknowns') {
      listPieces(id, { unknown: true, limit: 200 }).then(done(setUnknowns), () => {});
    }
    return () => {
      cancelled = true;
    };
  }, [id, tab, version]);

  return { collection, error, live, plan, parts, unknowns, connected };
}

/** A piece's name for people. */
export function pieceName(p) {
  if (!p.part_id) return 'Unknown piece';
  return p.part_name || `Part ${p.part_id}`;
}

/** Where a piece went: "Bin 1-4", "Bucket", or nothing yet. */
export function pieceBin(p) {
  if (!p.sorted_at) return '';
  if (p.bin_layer) return `Bin ${p.bin_layer}-${p.bin_number}`;
  return 'Bucket';
}
