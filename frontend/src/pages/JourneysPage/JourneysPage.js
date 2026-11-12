import { sorterJourneyFileURL } from '@/helpers/api';
import { cn } from '@/lib/utils';

import JourneyDetail from './components/JourneyDetail';
import { reasonInfo, useJourneysPageLogic } from './JourneysPage.logic';

const topReasons = (reasons) =>
  Object.entries(reasons ?? {})
    .sort((a, b) => b[1] - a[1])
    .slice(0, 2)
    .map(([r, n]) => `${reasonInfo(r).label.toLowerCase()} ${n}`)
    .join(', ');

/**
 * Diagnose › Piece journeys: every piece the cameras tracked, newest first, with each frame
 * of it, the ones kept as good pictures and why the rest weren't. For tuning the picture
 * rules (System › Settings › Pictures). Updates live while sorting.
 */
const JourneysPage = () => {
  const logic = useJourneysPageLogic();
  return (
    <div className="ds flex flex-col gap-4">
      <p className="max-w-[80ch] text-sm text-muted-foreground">
        Every piece the cameras tracked, newest first: its frames, the ones kept as good pictures
        for identifying it, and why the rest weren’t. Tune the rules under System › Settings ›
        Pictures. Updates live while sorting.
      </p>
      {logic.error && <p className="text-sm text-destructive">{logic.error}</p>}
      <div className="grid items-start gap-4 lg:grid-cols-[320px_minmax(0,1fr)]">
        <ul className="flex flex-col gap-1.5 lg:sticky lg:top-20 lg:max-h-[calc(100dvh-6rem)] lg:overflow-y-auto">
          {logic.list.length === 0 && (
            <li className="rounded-xl border border-dashed border-border px-4 py-6 text-sm text-muted-foreground">
              No pieces tracked yet. They appear here as the cameras see them.
            </li>
          )}
          {logic.list.map((j) => (
            <li key={j.id}>
              <button
                type="button"
                onClick={() => logic.select(j.id)}
                aria-current={j.id === logic.selectedId ? 'true' : undefined}
                className={cn(
                  'flex w-full items-center gap-3 rounded-lg border bg-card p-2 text-left transition-colors hover:border-border-strong',
                  j.id === logic.selectedId ? 'border-primary' : 'border-border',
                )}
              >
                {j.thumb ? (
                  <img
                    src={sorterJourneyFileURL(j.id, j.thumb)}
                    alt=""
                    className="size-14 shrink-0 rounded-md bg-media object-cover"
                  />
                ) : (
                  <span className="flex size-14 shrink-0 items-center justify-center rounded-md bg-muted text-center text-[10px] text-muted-foreground">
                    none kept
                  </span>
                )}
                <span className="flex min-w-0 flex-col">
                  <span className="truncate text-sm font-medium">
                    {(j.channels ?? []).map((c) => c.toUpperCase()).join(' → ')} ·{' '}
                    {new Date(j.started).toLocaleTimeString()}
                  </span>
                  <span className="truncate text-xs text-muted-foreground">
                    {j.kept} kept of {j.seen} · {j.done ? j.end_reason : 'riding'}
                  </span>
                  {topReasons(j.reasons) && (
                    <span className="truncate text-xs text-muted-foreground">
                      {topReasons(j.reasons)}
                    </span>
                  )}
                </span>
              </button>
            </li>
          ))}
        </ul>
        <JourneyDetail logic={logic} />
      </div>
    </div>
  );
};

export default JourneysPage;
