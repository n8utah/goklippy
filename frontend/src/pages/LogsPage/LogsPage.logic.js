import { useCallback, useEffect, useRef, useState } from 'react';

import { clearLogs, listLogs, logsDownloadURL } from '@/helpers/api';

const LIVE_MS = 3000;
const PAGE = 300;

export const LEVELS = [
  { value: 'info', label: 'Everything' },
  { value: 'warn', label: 'Warnings and errors' },
  { value: 'error', label: 'Errors' },
];

export const RANGES = [
  { value: '15m', label: 'Last 15 min', ms: 15 * 60_000 },
  { value: '1h', label: 'Last hour', ms: 60 * 60_000 },
  { value: '6h', label: 'Last 6 hours', ms: 6 * 60 * 60_000 },
  { value: '24h', label: 'Last 24 hours', ms: 24 * 60 * 60_000 },
];

/**
 * Diagnose › Logs: the backend's kept logs (24 h, Info and up), newest first, filtered
 * by level, text and time, following live, paged back, downloaded or cleared.
 */
export function useLogsPageLogic() {
  const [filters, setFilters] = useState({ level: 'info', q: '', range: '1h' });
  const [query, setQuery] = useState(filters); // with the search settled
  const [entries, setEntries] = useState([]);
  const [more, setMore] = useState(false);
  const [loadedKey, setLoadedKey] = useState(''); // which query the entries are for
  const [error, setError] = useState('');
  const [live, setLive] = useState(true);
  const [reload, setReload] = useState(0);
  const newest = useRef(null);

  useEffect(() => {
    const t = setTimeout(() => setQuery(filters), filters.q === query.q ? 0 : 300);
    return () => clearTimeout(t);
  }, [filters, query.q]);

  const params = useCallback(
    (extra = {}) => ({ level: query.level, q: query.q, limit: PAGE, ...extra }),
    [query],
  );

  // A new filter loads the range from the newest back.
  const key = JSON.stringify([query, reload]);
  const loading = loadedKey !== key;
  useEffect(() => {
    let cancelled = false;
    const ms = RANGES.find((r) => r.value === query.range)?.ms ?? RANGES[1].ms;
    const since = new Date(Date.now() - ms).toISOString();
    listLogs(params({ since })).then(
      (res) => {
        if (cancelled) return;
        setEntries(res.entries);
        setMore(res.more);
        newest.current = res.entries[0]?.time ?? since;
        setError('');
        setLoadedKey(key);
      },
      (err) => {
        if (cancelled) return;
        setError(err.message);
        setLoadedKey(key);
      },
    );
    return () => {
      cancelled = true;
    };
  }, [params, query.range, key]);

  // Live: newer entries are added on top every few seconds.
  useEffect(() => {
    if (!live || loading) return undefined;
    const timer = setInterval(() => {
      if (!newest.current) return;
      listLogs(params({ since: newest.current })).then(
        (res) => {
          if (!res.entries.length) return;
          newest.current = res.entries[0].time;
          setEntries((prev) => [...res.entries, ...prev]);
        },
        () => {},
      );
    }, LIVE_MS);
    return () => clearInterval(timer);
  }, [live, loading, params]);

  return {
    filters,
    setFilter: (key, value) => setFilters((prev) => ({ ...prev, [key]: value })),
    entries,
    more,
    loading,
    error,
    live,
    setLive,
    refresh: () => setReload((n) => n + 1),
    /** The next page back, older than what's shown (the range is only where it starts). */
    loadOlder: async () => {
      const oldest = entries[entries.length - 1]?.time;
      if (!oldest) return;
      try {
        const res = await listLogs(params({ before: oldest }));
        setEntries((prev) => [...prev, ...res.entries]);
        setMore(res.more);
      } catch (err) {
        setError(err.message);
      }
    },
    downloadURL: logsDownloadURL({ level: query.level, q: query.q }),
    clear: async () => {
      try {
        await clearLogs();
        setEntries([]);
        setMore(false);
        newest.current = new Date().toISOString();
      } catch (err) {
        setError(err.message);
      }
    },
  };
}
