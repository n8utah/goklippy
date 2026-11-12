import { createContext, useContext, useEffect, useState } from 'react';

import { getSorterPiece, reviewSorterPiece } from '@/helpers/api';

/** Opens a piece's journey: useJourneyModal().open(pieceId). */
export const JourneyModalContext = createContext({ open: () => {} });

export const useJourneyModal = () => useContext(JourneyModalContext);

const POLL_MS = 2000;
// A piece in one of these won't change any more.
const SETTLED = new Set(['distributed', 'lost']);

/** The piece's record, refreshed while it's still on its way. */
export function usePieceRecord(pieceId) {
  const [piece, setPiece] = useState(null);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!pieceId) return undefined;
    let cancelled = false;
    let timer;
    const load = () =>
      getSorterPiece(pieceId).then(
        (data) => {
          if (cancelled) return;
          setPiece(data);
          setError('');
          if (!SETTLED.has(data.status)) timer = setTimeout(load, POLL_MS);
        },
        (err) => {
          if (!cancelled) setError(err.message);
        },
      );
    load();
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [pieceId]);

  /** Saves a person's review; the piece comes back with it. */
  const review = async (r) => setPiece(await reviewSorterPiece(pieceId, r));

  return { piece: piece?.id === pieceId ? piece : null, error, review };
}

/** The piece's stages so far, in order, each { id, title, at, text, done }. */
export function pieceStages(piece) {
  const stages = [
    {
      id: 'c4',
      title: 'C4 · photographed and identified',
      at: piece.identified_at || piece.seen_at,
      done: Boolean(piece.identified_at),
      text: piece.identified_at
        ? `Landed and photographed; ${piece.part_id ? 'identified' : 'no answer'} after ${seconds(piece.seen_at, piece.identified_at)}.`
        : piece.status === 'multi_drop'
          ? 'Landed together with another piece.'
          : 'Landed; waiting for the identification.',
    },
    {
      id: 'chute',
      title: 'Chute',
      at: piece.distributed_at,
      done: Boolean(piece.distributed_at),
      text: piece.distributed_at
        ? piece.bin_layer
          ? `Dropped into layer ${piece.bin_layer}, bin ${piece.bin_number}${piece.route_reason ? ` (${piece.route_reason})` : ''}.`
          : `Dropped into the bucket${piece.route_reason ? ` (${piece.route_reason})` : ''}.`
        : piece.status === 'lost'
          ? 'Lost before it reached the chute.'
          : 'Not there yet.',
    },
  ];
  return stages;
}

function seconds(from, to) {
  const s = (new Date(to) - new Date(from)) / 1000;
  return Number.isFinite(s) ? `${s.toFixed(1)} s` : '—';
}
