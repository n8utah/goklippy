import { X } from 'lucide-react';

import { arrivalText, useJourneyModal } from '@/components/JourneyModal/JourneyModal.logic';
import { Button } from '@/components/ui/button';
import { sorterJourneyFileURL } from '@/helpers/api';
import { cn } from '@/lib/utils';

import { KEPT_COLOR, REASONS, reasonInfo } from '../JourneysPage.logic';
import Moment from './Moment';
import RingDiagram from './RingDiagram';

const Dot = ({ color }) => (
  <span className="inline-block size-2.5 rounded-full" style={{ background: color }} aria-hidden />
);

function FrameLarge({ journeyId, f, onClose }) {
  const name = f.crop || f.thumb;
  const info = reasonInfo(f.reason);
  return (
    <div className="grid gap-4 rounded-lg border border-border bg-muted/40 p-4 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
      {name ? (
        <img
          src={sorterJourneyFileURL(journeyId, name)}
          alt="The piece in this frame"
          className="w-full rounded-md bg-media object-contain"
        />
      ) : (
        <p className="text-sm text-muted-foreground">No picture kept for this frame.</p>
      )}
      <div className="flex flex-col gap-2 text-sm">
        <div className="flex items-start justify-between gap-2">
          <p className="flex items-center gap-2 font-medium">
            <Dot color={f.kept ? KEPT_COLOR : info.color} />
            {f.kept ? 'Kept' : info.label}
          </p>
          <Button variant="ghost" size="icon-sm" aria-label="Close" onClick={onClose}>
            <X />
          </Button>
        </div>
        {!f.kept && info.hint && <p className="text-muted-foreground">{info.hint}</p>}
        <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1">
          <dt className="text-muted-foreground">When</dt>
          <dd className="font-mono">
            {new Date(f.at).toLocaleTimeString([], { hour12: false, fractionalSecondDigits: 1 })}
          </dd>
          <dt className="text-muted-foreground">Where</dt>
          <dd>
            <span className="font-mono">{f.angle.toFixed(1)}°</span>
            {f.zone ? ` · ${f.zone} zone` : ''}
          </dd>
          <dt className="text-muted-foreground">Sharpness</dt>
          <dd className="font-mono">{f.sharpness.toFixed(1)}</dd>
          <dt className="text-muted-foreground">Box</dt>
          <dd className="font-mono">
            {f.box.x2 - f.box.x1}×{f.box.y2 - f.box.y1} px ({f.area} px²) at ({f.box.x1}, {f.box.y1}
            )
          </dd>
        </dl>
      </div>
    </div>
  );
}

function Chip({ on, onClick, color, children, title }) {
  return (
    <button
      type="button"
      title={title}
      aria-pressed={on}
      onClick={onClick}
      className={cn(
        'inline-flex h-8 items-center gap-1.5 rounded-full border px-3 text-xs font-medium',
        on
          ? 'border-primary bg-accent text-foreground'
          : 'border-border text-muted-foreground hover:bg-accent',
      )}
    >
      {color && <Dot color={color} />}
      {children}
    </button>
  );
}

function Track({ j, t, ti, logic }) {
  const counts = {};
  t.frames.forEach((f) => {
    const k = f.kept ? 'kept' : f.reason;
    counts[k] = (counts[k] ?? 0) + 1;
  });
  const shown = t.frames.map((f, i) => [f, i]).filter(([f]) => logic.showFrame(f));
  const large = logic.frame?.track === ti ? t.frames[logic.frame.index] : null;

  return (
    <section className="flex flex-col gap-4 rounded-xl border border-border bg-card p-5">
      <div className="flex flex-col gap-4 sm:flex-row">
        <RingDiagram track={t} onPick={(index) => logic.setFrame({ track: ti, index })} />
        <div className="flex min-w-0 flex-col gap-2">
          <h3 className="font-display text-lg font-semibold">
            {t.channel.toUpperCase()} · track #{t.track_id}
          </h3>
          <p className="text-sm text-muted-foreground">{arrivalText(j, t)}</p>
          <p className="text-sm text-muted-foreground">
            {new Date(t.started).toLocaleTimeString()} to{' '}
            {t.ended ? new Date(t.ended).toLocaleTimeString() : 'now'} ·{' '}
            {t.end_reason || 'still riding'} · {t.frames.length + (t.unlisted || 0)} frames
            {t.unlisted ? ` (${t.unlisted} past the list’s limit not shown)` : ''}
          </p>
          <div className="flex flex-wrap gap-1.5">
            <Chip on={logic.filter === 'all'} onClick={() => logic.setFilter('all')}>
              All {t.frames.length}
            </Chip>
            <Chip
              on={logic.filter === 'kept'}
              onClick={() => logic.setFilter('kept')}
              color={KEPT_COLOR}
            >
              Kept {counts.kept ?? 0}
            </Chip>
            {REASONS.filter((r) => counts[r.id]).map((r) => (
              <Chip
                key={r.id}
                on={logic.filter === r.id}
                onClick={() => logic.setFilter(r.id)}
                color={r.color}
                title={r.hint}
              >
                {r.label} {counts[r.id]}
              </Chip>
            ))}
          </div>
        </div>
      </div>

      {t.moments.length > 0 && (
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
          {t.moments.map((m) => (
            <Moment
              key={m.image}
              journeyId={j.id}
              moment={m}
              frameW={t.frame_w}
              frameH={t.frame_h}
            />
          ))}
        </div>
      )}

      {large && <FrameLarge journeyId={j.id} f={large} onClose={() => logic.setFrame(null)} />}

      <div className="grid grid-cols-[repeat(auto-fill,minmax(88px,1fr))] gap-2">
        {shown.map(([f, i]) => {
          const color = f.kept ? KEPT_COLOR : reasonInfo(f.reason).color;
          return (
            <button
              key={i}
              type="button"
              title={`${f.angle.toFixed(0)}° · ${f.kept ? 'kept' : f.reason} · sharpness ${f.sharpness.toFixed(0)}`}
              onClick={() => logic.setFrame({ track: ti, index: i })}
              className={cn(
                'flex flex-col overflow-hidden rounded-md border-2 bg-card text-left',
                logic.frame?.track === ti && logic.frame?.index === i
                  ? 'border-primary'
                  : 'border-transparent',
              )}
            >
              {f.thumb ? (
                <img
                  src={sorterJourneyFileURL(j.id, f.thumb)}
                  alt=""
                  className="aspect-square w-full bg-media object-cover"
                />
              ) : (
                <span className="flex aspect-square w-full items-center justify-center bg-muted text-[11px] text-muted-foreground">
                  no picture
                </span>
              )}
              <span
                className="flex items-center gap-1 truncate border-t-2 px-1.5 py-0.5 text-[11px]"
                style={{ borderColor: color }}
              >
                {f.kept ? 'kept' : reasonInfo(f.reason).label}
              </span>
            </button>
          );
        })}
        {shown.length === 0 && (
          <p className="col-span-full text-sm text-muted-foreground">No frames of this kind.</p>
        )}
      </div>
    </section>
  );
}

/** One piece's journey: every channel it was tracked on, frame by frame. */
export default function JourneyDetail({ logic }) {
  const journeyModal = useJourneyModal();
  const j = logic.journey;
  if (!logic.selectedId) {
    return (
      <p className="rounded-xl border border-dashed border-border px-5 py-10 text-center text-sm text-muted-foreground">
        Pick a piece to see its pictures and the decisions about them.
      </p>
    );
  }
  if (!j) return <p className="px-5 py-10 text-center text-sm text-muted-foreground">Loading…</p>;
  return (
    <div className="flex min-w-0 flex-col gap-4">
      <p className="text-sm text-muted-foreground">
        <span className="font-mono text-foreground">{j.kept}</span> good picture
        {j.kept === 1 ? '' : 's'} kept of{' '}
        <span className="font-mono text-foreground">{j.seen}</span> frames ·{' '}
        {j.done ? 'finished' : 'still riding, updating live'}. Pick a dot or a picture for its
        details.
      </p>
      {j.piece_id ? (
        <p className="text-sm">
          It became piece <span className="font-mono">{j.piece_id.slice(0, 8)}</span>.{' '}
          <button
            type="button"
            onClick={() => journeyModal.open(j.piece_id)}
            className="font-medium text-primary-text hover:underline"
          >
            Open its record
          </button>
        </p>
      ) : (
        j.done && (
          <p className="text-sm text-muted-foreground">
            Not linked to a sorted piece: it didn’t reach C4’s drop zone, or more than one piece
            could have been it.
          </p>
        )
      )}
      {j.tracks.map((t, ti) => (
        <Track key={`${t.channel}-${t.track_id}`} j={j} t={t} ti={ti} logic={logic} />
      ))}
    </div>
  );
}
