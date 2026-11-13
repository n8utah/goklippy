import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { formatDuration } from '@/helpers/sorterRuns';
import { cn } from '@/lib/utils';

/** The states, in the series order their colours follow (backend time_spent.go). */
const STATES = [
  {
    id: 'moving',
    label: 'Moving',
    swatch: 'bg-series-1',
    about: 'turning: a pulse, an index, a nudge',
  },
  {
    id: 'busy',
    label: 'Busy',
    swatch: 'bg-series-2',
    about: 'C4 photographing or identifying; C2/C3 pausing after a drop',
  },
  {
    id: 'waiting',
    label: 'Waiting',
    swatch: 'bg-series-3',
    about: 'ready, but the next channel can’t take a piece (C4: waiting to turn)',
  },
  { id: 'empty', label: 'Empty', swatch: 'bg-series-4', about: 'nothing to do' },
  { id: 'stuck', label: 'Stuck', swatch: 'bg-series-5', about: 'recovering a stuck piece' },
];

const CHANNELS = ['c1', 'c2', 'c3', 'c4'];

const pct = (part, total) => (total > 0 ? Math.round((part / total) * 100) : 0);
const secs = (ms) => `${(ms / 1000).toFixed(ms < 10000 ? 1 : 0)} s`;

/** The busiest channel (most of its time moving or busy): the likely bottleneck. */
function busiest(timeMs) {
  let best = null;
  for (const ch of CHANNELS) {
    const t = timeMs[ch] ?? {};
    const total = STATES.reduce((n, s) => n + (t[s.id] ?? 0), 0);
    if (!total) continue;
    const share = ((t.moving ?? 0) + (t.busy ?? 0)) / total;
    if (!best || share > best.share) best = { ch, share };
  }
  return best;
}

/**
 * Where the run's time goes, per channel: a 100% bar of moving, busy, waiting on the
 * next channel, empty and stuck, with the shares written out under it. The busiest
 * channel is named as the likely bottleneck.
 */
export default function TimeSpent({ timeMs, current, idleDroppedMs }) {
  if (!timeMs || !Object.keys(timeMs).length) return null;
  const top = busiest(timeMs);

  return (
    <section aria-labelledby="sort-time" className="rounded-xl border border-border bg-card p-4">
      <div className="mb-3 flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
        <h3 id="sort-time" className="font-display text-lg font-semibold">
          Where the time goes
        </h3>
        <span className="text-sm text-muted-foreground">
          {current ? 'This run' : 'Last run'}
          {idleDroppedMs >= 1000 &&
            ` · leaves out ${formatDuration(idleDroppedMs)} of C1 turning an empty hopper`}
          {top &&
            ` · busiest: ${top.ch.toUpperCase()} (${Math.round(top.share * 100)}% moving or busy)`}
        </span>
      </div>

      <ul
        className="mb-3 flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground"
        aria-label="Key"
      >
        {STATES.map((s) => (
          <li key={s.id} className="flex items-center gap-1.5" title={s.about}>
            <span className={cn('size-2.5 rounded-sm', s.swatch)} aria-hidden />
            {s.label}
          </li>
        ))}
      </ul>

      <div className="flex flex-col gap-3">
        {CHANNELS.map((ch) => {
          const t = timeMs[ch] ?? {};
          const total = STATES.reduce((n, s) => n + (t[s.id] ?? 0), 0);
          const parts = STATES.filter((s) => (t[s.id] ?? 0) > 0);
          return (
            <div key={ch} className="grid grid-cols-[2.5rem_1fr] items-start gap-x-3">
              <span className="text-sm leading-3 font-medium">{ch.toUpperCase()}</span>
              <div className="min-w-0">
                {total ? (
                  <div
                    className="flex h-3 gap-0.5"
                    role="img"
                    aria-label={`${ch.toUpperCase()}: ${parts.map((s) => `${s.label} ${pct(t[s.id], total)}%`).join(', ')}`}
                  >
                    {parts.map((s) => (
                      <Tooltip key={s.id}>
                        <TooltipTrigger asChild>
                          <div
                            className={cn('h-full min-w-1 rounded-sm', s.swatch)}
                            style={{ flexGrow: t[s.id] }}
                          />
                        </TooltipTrigger>
                        <TooltipContent>
                          {ch.toUpperCase()} · {s.label}: {secs(t[s.id])} ({pct(t[s.id], total)}%)
                          <span className="block text-xs opacity-80">{s.about}</span>
                        </TooltipContent>
                      </Tooltip>
                    ))}
                  </div>
                ) : (
                  <div className="h-3 rounded-sm bg-muted" />
                )}
                <p className="mt-1 text-xs text-muted-foreground">
                  {total
                    ? parts
                        .map((s) => `${s.label.toLowerCase()} ${pct(t[s.id], total)}%`)
                        .join(' · ')
                    : 'No time yet'}
                </p>
              </div>
            </div>
          );
        })}
      </div>
    </section>
  );
}
