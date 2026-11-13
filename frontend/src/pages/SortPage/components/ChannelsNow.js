import { cn } from '@/lib/utils';

const CHANNELS = [
  { id: 'c1', label: 'C1', name: 'Bulk' },
  { id: 'c2', label: 'C2', name: 'C-channel 2' },
  { id: 'c3', label: 'C3', name: 'C-channel 3' },
  { id: 'c4', label: 'C4', name: 'Classification' },
];

/** Words that mean the channel is held up or in trouble: shown in the warning colour. */
const TROUBLE = /^(stuck|stopped|paused while|recovery|a piece hangs)/i;

/**
 * What each channel is doing right now, in words with why ("a piece is at the exit;
 * waiting: C3 is full"), while a run is on. From the state's `doing` (backend
 * machine_sort.go channelsDoing), updated about once a second.
 */
export default function ChannelsNow({ doing }) {
  if (!doing || !Object.keys(doing).length) return null;
  return (
    <section aria-labelledby="sort-now" className="rounded-xl border border-border bg-card p-4">
      <h3 id="sort-now" className="mb-2 font-display text-lg font-semibold">
        Right now
      </h3>
      <ul className="grid gap-x-6 gap-y-2 md:grid-cols-2">
        {CHANNELS.map((ch) => {
          const what = doing[ch.id] || '—';
          return (
            <li key={ch.id} className="flex min-w-0 items-baseline gap-2 text-sm">
              <span className="w-7 shrink-0 font-medium">{ch.label}</span>
              <span
                className={cn(
                  'min-w-0 first-letter:uppercase',
                  TROUBLE.test(what) ? 'text-warning' : 'text-foreground',
                )}
                title={`${ch.label} · ${ch.name}`}
              >
                {what}
              </span>
            </li>
          );
        })}
      </ul>
    </section>
  );
}
