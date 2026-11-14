import { useEffect, useRef, useState } from 'react';
import { useSearchParams } from 'react-router';

import {
  getCollection,
  getPlan,
  getSet,
  listParts,
  listPieces,
  listSets,
  watchCollection,
} from '@/helpers/api';

export const TABS = [
  { id: 'live', label: 'Live' },
  { id: 'plan', label: 'Plan' },
  { id: 'parts', label: 'Parts' },
  { id: 'unknowns', label: 'Unknowns' },
  { id: 'sets', label: 'Sets' },
];

/** The smallest set shown, so 3-piece polybags don't crowd the top. */
export const SET_SIZES = [
  { min: 1, label: 'Any size' },
  { min: 25, label: '25+ parts' },
  { min: 100, label: '100+' },
  { min: 500, label: '500+' },
];
const SETS_PAGE = 60;
const SETS_MAX = 200;

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

/**
 * The Sets tab: sets the collection covers best, filtered and ordered on the server, and
 * the one opened (?set=<number> in the address, so it can be linked). Scoring takes a
 * moment on a big collection, so it isn't redone on every piece the sorter sends; Refresh
 * does it on demand.
 */
export function useSets(id, active) {
  const [q, setQ] = useState('');
  const [query, setQuery] = useState(''); // q once typing pauses
  const [min, setMin] = useState(25);
  const [any, setAny] = useState(false);
  const [limit, setLimit] = useState(SETS_PAGE);
  const [reload, setReload] = useState(0);
  // Each answer is kept with the request it answers (key), so loading is "the latest
  // answer is for an older request".
  const [answer, setAnswer] = useState({ key: '', result: null, error: '' });
  const [params, setParams] = useSearchParams();
  const open = params.get('set') || '';
  const [opened, setOpened] = useState({ key: '', data: null });

  useEffect(() => {
    const t = setTimeout(() => {
      setQuery(q.trim());
      setLimit(SETS_PAGE);
    }, 250);
    return () => clearTimeout(t);
  }, [q]);

  const key = JSON.stringify([id, query, min, any, limit, reload]);
  useEffect(() => {
    if (!active) return undefined;
    let cancelled = false;
    listSets(id, { q: query, min, any, limit }).then(
      (result) => !cancelled && setAnswer({ key, result, error: '' }),
      (err) => !cancelled && setAnswer((a) => ({ ...a, key, error: err.message })),
    );
    return () => {
      cancelled = true;
    };
  }, [active, key, id, query, min, any, limit]);

  const openKey = JSON.stringify([id, open, reload]);
  useEffect(() => {
    if (!active || !open) return undefined;
    let cancelled = false;
    getSet(id, open).then(
      (data) => !cancelled && setOpened({ key: openKey, data }),
      (err) => !cancelled && setOpened({ key: openKey, data: { error: err.message } }),
    );
    return () => {
      cancelled = true;
    };
  }, [active, openKey, id, open]);

  const { result, error } = answer;
  const loading = active && answer.key !== key;
  let detail = null;
  if (open) detail = opened.key === openKey ? opened.data : { loading: true };
  const total = result?.total ?? 0;
  return {
    q,
    setQ,
    min,
    setMin: (m) => {
      setMin(m);
      setLimit(SETS_PAGE);
    },
    any,
    setAny: (a) => {
      setAny(a);
      setLimit(SETS_PAGE);
    },
    result,
    error,
    loading,
    canShowMore: Boolean(result) && result.sets.length < Math.min(total, SETS_MAX),
    showMore: () => setLimit((l) => Math.min(l + SETS_PAGE, SETS_MAX)),
    refresh: () => setReload((r) => r + 1),
    open,
    detail,
    openSet: (setNum) =>
      setParams((p) => {
        const next = new URLSearchParams(p);
        if (setNum) next.set('set', setNum);
        else next.delete('set');
        return next;
      }),
  };
}

/** A share of a set as a whole percentage, rounded down so 99.6% doesn't read 100%. */
export function percent(have, need) {
  return need ? Math.floor((100 * have) / need) : 0;
}

/** How much of a set there is, as ranked: in the right colours, or (any) in any colour. */
export function setShare(set, any) {
  return percent(any ? set.have_any_colour : set.have, set.need);
}

/**
 * "Missing 10 of 50 · 92% in any colour", or counting any colour:
 * "Missing 4 of 50 in any colour · 80% in the right colours".
 */
export function coverText(set, any) {
  const have = any ? set.have_any_colour : set.have;
  const missing = set.need - have;
  const suffix = any ? ' in any colour' : '';
  const parts = [
    missing === 0 ? `All ${set.need} parts${suffix}` : `Missing ${missing} of ${set.need}${suffix}`,
  ];
  if (set.have_any_colour > set.have) {
    parts.push(
      any
        ? `${percent(set.have, set.need)}% in the right colours`
        : `${percent(set.have_any_colour, set.need)}% in any colour`,
    );
  }
  return parts.join(' · ');
}
