import { useEffect, useState } from 'react';

import {
  clearSorterCatalog,
  clearSorterPhotos,
  fillSorterCatalog,
  getSorterCatalogFacets,
  getSorterPhotos,
  listSorterCatalog,
  listSorterCollections,
  listSorterPieces,
  listSorterRuns,
  sorterPieceImageURL,
} from '@/helpers/api';
import { formatBytes } from '@/helpers/format';

export const PAGE_SIZE = 60;
const PREFS_KEY = 'goklippy.pieces.prefs';
const EMPTY_FILTERS = { q: '', color: '', category: '', collection: '', run: '', status: '' };

/** The parts' sorts, each with the way it reads best first. */
export const SORTS = [
  { value: 'seen', label: 'Most seen', dir: 'desc' },
  { value: 'last', label: 'Last seen', dir: 'desc' },
  { value: 'name', label: 'Name', dir: 'asc' },
  { value: 'part', label: 'Part number', dir: 'asc' },
  { value: 'color', label: 'Color', dir: 'asc' },
];

function readPrefs() {
  try {
    const p = JSON.parse(localStorage.getItem(PREFS_KEY) || '{}');
    return {
      view: p.view === 'list' ? 'list' : 'grid',
      picture: p.picture === 'camera' ? 'camera' : 'official',
    };
  } catch {
    return { view: 'grid', picture: 'official' };
  }
}

/** A part's pictures: a kept camera photo of it, and Brickognize's. */
export const partPictures = (part) => ({
  camera: part.photo_piece_id
    ? sorterPieceImageURL(part.photo_piece_id, part.photo_index ?? 0)
    : '',
  official: part.img_url,
});

/** A piece's pictures: its first kept photo, and Brickognize's picture of its part. */
export const piecePictures = (piece) => ({
  camera:
    piece.status !== 'tracking' && piece.images?.length
      ? sorterPieceImageURL(piece.id, piece.images[0].index)
      : '',
  official: piece.part_img_url,
});

/**
 * How many of a part were seen: in the run or collection filtered to (`scope`, null for
 * all), or in all.
 */
export function seenIn(part, scope) {
  if (scope?.run) return part.runs.find((r) => String(r.run_id) === String(scope.run))?.seen ?? 0;
  if (scope?.collection) {
    return part.runs
      .filter((r) => String(r.collection_id) === String(scope.collection))
      .reduce((n, r) => n + r.seen, 0);
  }
  return part.seen;
}

/** The file name's scope: "-run-4", "-collection-2" or nothing. */
const scopeName = ({ run, collection }) =>
  run ? `-run-${run}` : collection ? `-collection-${collection}` : '';

const csvCell = (v) => {
  const s = String(v ?? '');
  return /[",\n]/.test(s) ? `"${s.replaceAll('"', '""')}"` : s;
};

function download(name, head, rows) {
  const text = [head, ...rows].map((r) => r.map(csvCell).join(',')).join('\r\n');
  const url = URL.createObjectURL(new Blob([text], { type: 'text/csv' }));
  const a = document.createElement('a');
  a.href = url;
  a.download = `${name}-${new Date().toISOString().slice(0, 10)}.csv`;
  a.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

function exportParts(parts, scope) {
  download(
    `lego-parts${scopeName(scope)}`,
    [
      'part_id',
      'part_name',
      'color_id',
      'color_name',
      'category',
      scope.run ? `seen_in_run_${scope.run}` : scope.collection ? 'seen_in_collection' : 'seen',
      'seen_total',
      'first_seen',
      'last_seen',
      'runs',
      'image',
    ],
    parts.map((p) => [
      p.part_id,
      p.part_name,
      p.color_id,
      p.color_name,
      p.category,
      seenIn(p, scope),
      p.seen,
      p.first_seen,
      p.last_seen,
      p.runs.map((r) => `${r.run_id}:${r.seen}`).join(' '),
      p.img_url,
    ]),
  );
}

async function exportPieces(filters) {
  const all = [];
  for (let offset = 0; ; offset += 500) {
    const { pieces, total } = await listSorterPieces({ ...filters, limit: 500, offset });
    all.push(...pieces);
    if (pieces.length === 0 || all.length >= total) break;
  }
  download(
    `lego-pieces${scopeName(filters)}`,
    [
      'piece_id',
      'run',
      'status',
      'seen_at',
      'part_id',
      'part_name',
      'part_score',
      'color_id',
      'color_name',
      'category',
      'bin_layer',
      'bin',
      'virtual_bin',
      'image',
    ],
    all.map((p) => [
      p.id,
      p.run_id ?? '',
      p.status,
      p.seen_at,
      p.part_id,
      p.part_name,
      p.part_score ?? '',
      p.color_id,
      p.color_name,
      p.category_name,
      p.bin_layer ?? '',
      p.bin_number ?? '',
      p.virtual_bin ? 'yes' : '',
      p.part_img_url,
    ]),
  );
}

function useLoad(fn, deps) {
  const [state, setState] = useState({ data: null, error: '' });
  useEffect(() => {
    let cancelled = false;
    fn().then(
      (data) => !cancelled && setState({ data, error: '' }),
      (err) => !cancelled && setState((s) => ({ ...s, error: err.message })),
    );
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);
  return state;
}

/**
 * Pieces: every part found across runs (tab "parts") and every piece seen (tab "all"),
 * with shared filters, a grid or a list, camera or official pictures, export, and
 * clearing the catalog or the photos. `linkedRun` (?run=) filters to one run,
 * `linkedCollection` (?collection=) to one collection.
 */
export function usePiecesPageLogic(tab, linkedRun, linkedCollection) {
  const [filters, setFilters] = useState({
    ...EMPTY_FILTERS,
    run: linkedRun || '',
    collection: linkedCollection || '',
  });
  const [query, setQuery] = useState(filters); // filters, with the search settled
  const [sort, setSortState] = useState({ sort: 'seen', dir: 'desc' });
  const [prefs, setPrefs] = useState(readPrefs);
  const [offset, setOffset] = useState(0);
  const [reload, setReload] = useState(0);
  const [busy, setBusy] = useState('');
  const [message, setMessage] = useState('');
  const [actionError, setActionError] = useState('');
  const [openPart, setOpenPart] = useState(null); // { part_id, color_id }

  // A link to a run or collection while already here (from a journey) moves the filter.
  const [seenLink, setSeenLink] = useState(`${linkedRun}|${linkedCollection}`);
  if (`${linkedRun}|${linkedCollection}` !== seenLink) {
    setSeenLink(`${linkedRun}|${linkedCollection}`);
    if (linkedRun || linkedCollection) {
      setFilters({ ...EMPTY_FILTERS, run: linkedRun || '', collection: linkedCollection || '' });
      setOffset(0);
    }
  }

  // Typing settles for a moment before the search is asked.
  useEffect(() => {
    const t = setTimeout(() => setQuery(filters), filters.q === query.q ? 0 : 250);
    return () => clearTimeout(t);
  }, [filters, query.q]);

  const parts = useLoad(
    () =>
      tab === 'parts'
        ? listSorterCatalog({ ...query, status: '', ...sort }).then((r) => r.parts ?? [])
        : Promise.resolve(null),
    [tab, query, sort, reload],
  );
  const pieces = useLoad(
    () =>
      tab === 'all'
        ? listSorterPieces({ ...query, limit: PAGE_SIZE, offset })
        : Promise.resolve(null),
    [tab, query, offset, reload],
  );
  const facets = useLoad(getSorterCatalogFacets, [reload]);
  const runs = useLoad(() => listSorterRuns().then((r) => r.runs ?? []), [reload]);
  const collections = useLoad(
    () => listSorterCollections().then((r) => r.collections ?? []),
    [reload],
  );
  const photos = useLoad(getSorterPhotos, [reload]);

  const savePrefs = (next) => {
    setPrefs(next);
    try {
      localStorage.setItem(PREFS_KEY, JSON.stringify(next));
    } catch {
      // remembered for this visit only
    }
  };

  const act = async (name, fn) => {
    setBusy(name);
    setMessage('');
    setActionError('');
    try {
      setMessage(await fn());
      setReload((n) => n + 1);
    } catch (err) {
      setActionError(err.message);
    } finally {
      setBusy('');
    }
  };

  const filtered = Boolean(
    filters.q ||
    filters.color ||
    filters.category ||
    filters.collection ||
    filters.run ||
    filters.status,
  );
  const allRuns = runs.data ?? [];

  return {
    filters,
    filtered,
    setFilter: (key, value) => {
      setFilters((prev) => {
        const next = { ...prev, [key]: value };
        // A run from another collection doesn't fit the collection picked.
        if (key === 'collection' && value && prev.run) {
          const run = allRuns.find((r) => String(r.id) === String(prev.run));
          if (run && String(run.collection_id) !== String(value)) next.run = '';
        }
        return next;
      });
      setOffset(0);
    },
    resetFilters: () => {
      setFilters(EMPTY_FILTERS);
      setOffset(0);
    },
    sort,
    setSort: (value) =>
      setSortState({ sort: value, dir: SORTS.find((s) => s.value === value)?.dir ?? 'asc' }),
    flipDir: () => setSortState((s) => ({ ...s, dir: s.dir === 'asc' ? 'desc' : 'asc' })),
    view: prefs.view,
    setView: (view) => savePrefs({ ...prefs, view }),
    picture: prefs.picture,
    setPicture: (picture) => savePrefs({ ...prefs, picture }),
    /** The run or collection the shown list is filtered to (settled with the list), or null. */
    shownScope:
      query.run || query.collection ? { run: query.run, collection: query.collection } : null,
    parts: parts.data,
    pieces: pieces.data,
    offset,
    setOffset,
    facets: facets.data,
    /** Runs for the run filter: the picked collection's, or every one. */
    runs: filters.collection
      ? allRuns.filter((r) => String(r.collection_id) === String(filters.collection))
      : allRuns,
    collections: collections.data ?? [],
    photos: photos.data,
    error: actionError || parts.error || pieces.error || facets.error,
    busy,
    message,
    refresh: () => setReload((n) => n + 1),
    openPart,
    setOpenPart,
    exportCsv: () =>
      tab === 'parts'
        ? exportParts(parts.data ?? [], query)
        : act('export', async () => {
            await exportPieces(query);
            return '';
          }),
    fill: () =>
      act('fill', async () => {
        const { added } = await fillSorterCatalog();
        return added
          ? `Added ${added} piece${added === 1 ? '' : 's'} from the records to the parts.`
          : 'Every identified piece in the records is already counted in the parts.';
      }),
    clearCatalog: () =>
      act('clearCatalog', async () => {
        await clearSorterCatalog();
        return 'The parts were cleared. The piece records and photos are kept.';
      }),
    clearPhotos: () =>
      act('clearPhotos', async () => {
        const { freed } = await clearSorterPhotos();
        return `Removed ${freed.files.toLocaleString()} photos (${formatBytes(freed.bytes)}). The records and parts are kept.`;
      }),
  };
}
