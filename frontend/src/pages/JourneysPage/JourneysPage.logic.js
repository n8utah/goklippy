import { useEffect, useState } from 'react';

import { getSorterJourney, listSorterJourneys } from '../../helpers/api';

const POLL_MS = 2000;

// Why a frame wasn't kept, in the order they're checked, with a color for each.
export const REASONS = [
  {
    id: 'cut off',
    label: 'Cut off',
    color: '#94a3b8',
    hint: 'Touches the frame’s edge or reaches past the ring.',
  },
  {
    id: 'not alone',
    label: 'Not alone',
    color: '#f97316',
    hint: 'Another piece within the isolation margin.',
  },
  { id: 'settling', label: 'Settling', color: '#a855f7', hint: 'Its first frames after landing.' },
  {
    id: 'moving',
    label: 'Moving',
    color: '#0ea5e9',
    hint: 'Moved too far since the last frame to be sharp.',
  },
  {
    id: 'size jump',
    label: 'Size jump',
    color: '#ef4444',
    hint: 'Box far off its usual size: touching something or hidden.',
  },
  {
    id: 'blurry',
    label: 'Blurry',
    color: '#64748b',
    hint: 'Not enough detail (sharpness below the minimum).',
  },
  {
    id: 'a sharper one nearby',
    label: 'Sharper one nearby',
    color: '#cbd5e1',
    hint: 'Good, but a sharper picture from the same stretch was kept.',
  },
  {
    id: 'over the limit',
    label: 'Over the limit',
    color: '#e2e8f0',
    hint: 'Good, but more than the channel’s limit; the least sharp were let go.',
  },
];
export const KEPT_COLOR = '#16a34a';

export const reasonInfo = (id) =>
  REASONS.find((r) => r.id === id) ?? { id, label: id, color: '#94a3b8' };

export function useJourneysPageLogic() {
  const [list, setList] = useState([]);
  const [error, setError] = useState('');
  const [selectedId, setSelectedId] = useState(null);
  const [journey, setJourney] = useState(null);
  const [filter, setFilter] = useState('all'); // all | kept | a reason id
  const [frame, setFrame] = useState(null); // { track, index } shown large

  // The list, live.
  useEffect(() => {
    let cancelled = false;
    const poll = () =>
      listSorterJourneys(100).then(
        (data) => {
          if (cancelled) return;
          setList(data.journeys ?? []);
          setError('');
        },
        (err) => {
          if (!cancelled) setError(err.message);
        },
      );
    poll();
    const timer = setInterval(poll, POLL_MS);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, []);

  // The selected journey, refreshed while it's still riding.
  const selectedDone = list.find((j) => j.id === selectedId)?.done;
  useEffect(() => {
    if (!selectedId) return undefined;
    let cancelled = false;
    const load = () =>
      getSorterJourney(selectedId).then(
        (data) => {
          if (!cancelled) setJourney(data);
        },
        (err) => {
          if (!cancelled) setError(err.message);
        },
      );
    load();
    const timer = selectedDone ? null : setInterval(load, POLL_MS);
    return () => {
      cancelled = true;
      if (timer) clearInterval(timer);
    };
  }, [selectedId, selectedDone]);

  const select = (id) => {
    setSelectedId(id);
    setJourney(null);
    setFrame(null);
  };

  const showFrame = (f) => {
    if (filter === 'all') return true;
    if (filter === 'kept') return f.kept;
    return !f.kept && f.reason === filter;
  };

  return {
    list,
    error,
    selectedId,
    journey: journey?.id === selectedId ? journey : null,
    select,
    filter,
    setFilter,
    showFrame,
    frame,
    setFrame,
  };
}
