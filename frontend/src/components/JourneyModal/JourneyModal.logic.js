import { createContext, useContext, useEffect, useState } from 'react';

import { getSorterPiece, getSorterPieceJourney, reviewSorterPiece } from '@/helpers/api';

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

/**
 * The camera journey that became the piece (its tracks on C2, C3 and C4), refreshed
 * while the piece is still on its way. null while loading or when none is known: the
 * cameras above didn't see it fall, or it was one of several.
 */
export function usePieceJourney(pieceId, settled) {
  const [journey, setJourney] = useState(null);
  const [known, setKnown] = useState(null); // null loading, false none

  useEffect(() => {
    if (!pieceId) return undefined;
    let cancelled = false;
    let timer;
    const load = () =>
      getSorterPieceJourney(pieceId).then(
        (data) => {
          if (cancelled) return;
          setJourney(data);
          setKnown(true);
          if (!data.done) timer = setTimeout(load, POLL_MS);
        },
        () => {
          if (cancelled) return;
          setKnown(false);
          // It may yet be linked: the cameras settle it within a couple of seconds.
          if (!settled) timer = setTimeout(load, POLL_MS);
        },
      );
    load();
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [pieceId, settled]);

  return { journey: journey?.piece_id === pieceId ? journey : null, known };
}

export const isSettled = (piece) => Boolean(piece && SETTLED.has(piece.status));

const CH = (c) => c.toUpperCase();

/** How a piece came onto a track's channel, from the journey's handoffs. */
export function arrivalText(journey, track) {
  const h = journey.handoffs?.find((x) => x.to === track.channel);
  if (!h) return 'First seen here.';
  switch (h.result) {
    case 'linked':
      return `Fell from ${CH(h.from)} cleanly, landing ${h.gap_ms ?? 0} ms after it left.`;
    case 'several':
      return `Landed when ${h.candidates > 1 ? `${h.candidates} pieces` : 'more than one piece'} could have come from ${CH(h.from)}, so its pictures from there aren’t used.`;
    default:
      return `Nothing was seen leaving ${CH(h.from)} just before it landed.`;
  }
}

/** The piece's stages so far, in order, each { id, title, at, text, done }. */
export function pieceStages(piece) {
  const stages = [
    {
      id: 'c4',
      title: 'Identification (C4)',
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
