// How long a run really sorted, for per-minute figures (Run › Sort, Run › Records).
//
// A run's sorting ends with its last piece sorted (last_piece_at), not when it was
// stopped: time after that was C1 turning an empty hopper, or the machine left running,
// and would drag the rate down. While a run is on, it's sorting until now, except once
// it's finishing on its own (C2 empty a while), when the last piece is the end too.

/** The run's sorting time in ms; finishing: the run is finishing on its own now. */
export function sortingMs(run, finishing = false, now = Date.now()) {
  if (!run) return 0;
  const start = new Date(run.started_at).getTime();
  let end;
  if (run.ended_at || finishing) {
    end = run.last_piece_at ? new Date(run.last_piece_at).getTime() : start;
  } else {
    end = now;
  }
  return Math.max(0, end - start);
}

/** Pieces sorted per minute of sorting. */
export function perMinute(run, finishing = false, now = Date.now()) {
  const ms = sortingMs(run, finishing, now);
  return ms >= 1000 ? run.distributed / (ms / 60000) : 0;
}

/** Under this much sorting, a per-minute figure isn't a steady rate. */
export const SHORT_RUN_MS = 60_000;

export const SHORT_RUN_NOTE =
  '* Sorted for under a minute: too short for a steady rate, so the figure isn’t comparable.';

/**
 * Pieces per minute for people: "8.0", or "14.4*" when the run sorted for under a
 * minute (SHORT_RUN_NOTE says why). "—" when nothing was sorted.
 */
export function perMinuteText(run, finishing = false, now = Date.now()) {
  if (!run?.distributed) return '—';
  const rate = perMinute(run, finishing, now).toFixed(1);
  return sortingMs(run, finishing, now) < SHORT_RUN_MS ? `${rate}*` : rate;
}

/** A duration for people: "45 s", "2.2 min", "1 h 05 min". */
export function formatDuration(ms) {
  const s = ms / 1000;
  if (s < 90) return `${Math.round(s)} s`;
  if (s < 3600) return `${(s / 60).toFixed(1)} min`;
  const h = Math.floor(s / 3600);
  return `${h} h ${String(Math.round((s % 3600) / 60)).padStart(2, '0')} min`;
}
