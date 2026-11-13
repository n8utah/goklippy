import { Check, Circle } from 'lucide-react';
import { Link } from 'react-router';

import { PieceThumb, TONE_BADGE } from '@/components/PieceCard/PieceCard';
import { Badge } from '@/components/ui/badge';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Skeleton } from '@/components/ui/skeleton';
import { sorterJourneyFileURL, sorterPieceImageURL } from '@/helpers/api';
import { PROBLEM_LABELS, pieceBin, pieceName, pieceStatus, scorePct } from '@/helpers/sorterPieces';
import { cn } from '@/lib/utils';

import {
  arrivalText,
  isSettled,
  pieceStages,
  usePieceJourney,
  usePieceRecord,
} from './JourneyModal.logic';
import ReviewPanel from './ReviewPanel';

const time = (t) =>
  t
    ? new Date(t).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
    : '';

/**
 * A piece's way through the machine, opened from Run › Sort, Home and Records
 * (docs/ui-ux-spec.md, "Journey modal"). `pieceId` null keeps it closed.
 */
export default function JourneyModal({ pieceId, onClose }) {
  return (
    <Dialog open={Boolean(pieceId)} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-3xl">
        {pieceId && <Journey key={pieceId} pieceId={pieceId} onClose={onClose} />}
      </DialogContent>
    </Dialog>
  );
}

/** One camera channel the piece crossed: how it got there, and its kept pictures. */
function CameraStage({ journey, track }) {
  const kept = track.frames.filter((f) => f.kept);
  const span = track.ended
    ? ((new Date(track.ended) - new Date(track.started)) / 1000).toFixed(1)
    : null;
  return (
    <li className="flex flex-col gap-2 rounded-lg border border-border px-4 py-3">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <p className="text-sm font-medium">
          {track.channel.toUpperCase()} · {kept.length} good picture{kept.length === 1 ? '' : 's'}{' '}
          of {track.frames.length + (track.unlisted || 0)}
        </p>
        <span className="font-mono text-xs text-muted-foreground">
          {time(track.started)}
          {span && ` · ${span} s`}
        </span>
      </div>
      <p className="text-sm text-muted-foreground">
        {arrivalText(journey, track)}{' '}
        {track.end_reason ? `It ${track.end_reason}.` : 'Still riding.'}
      </p>
      {kept.length > 0 && (
        <div className="flex gap-1.5 overflow-x-auto pb-1">
          {kept.map((f) => (
            <img
              key={f.crop}
              src={sorterJourneyFileURL(journey.id, f.thumb || f.crop)}
              alt={`On ${track.channel.toUpperCase()} at ${f.angle.toFixed(0)}°`}
              title={`${f.angle.toFixed(0)}° · sharpness ${f.sharpness.toFixed(0)}`}
              className="size-16 shrink-0 rounded-md bg-media object-cover"
            />
          ))}
        </div>
      )}
    </li>
  );
}

function Journey({ pieceId, onClose }) {
  const { piece, error, review } = usePieceRecord(pieceId);
  const { journey, known } = usePieceJourney(pieceId, isSettled(piece));

  if (!piece) {
    return (
      <>
        <DialogHeader>
          <DialogTitle className="font-display">Piece {pieceId.slice(0, 8)}</DialogTitle>
          <DialogDescription>{error || 'Loading its record…'}</DialogDescription>
        </DialogHeader>
        {!error && <Skeleton className="h-40 w-full" />}
      </>
    );
  }

  const status = pieceStatus(piece.status);
  const prediction = piece.prediction;
  const matches = prediction?.items?.slice(0, 4) ?? [];
  const colors = prediction?.colors?.slice(0, 3) ?? [];

  return (
    <div className="flex flex-col gap-6">
      {/* Header: the piece. */}
      <div className="flex gap-4 pr-6">
        <PieceThumb piece={piece} className="size-24" />
        <DialogHeader className="min-w-0 flex-1 gap-1 text-left">
          <div className="flex flex-wrap items-center gap-2">
            <DialogTitle className="font-display text-xl">{pieceName(piece)}</DialogTitle>
            <Badge variant={TONE_BADGE[status.tone]}>{status.label}</Badge>
          </div>
          <DialogDescription asChild>
            <div className="flex flex-col gap-0.5 text-sm">
              <span>
                {[
                  piece.part_id && `${piece.part_id} ${scorePct(piece.part_score)}`.trim(),
                  piece.color_name && `${piece.color_name} ${scorePct(piece.color_score)}`.trim(),
                ]
                  .filter(Boolean)
                  .join(' · ') || 'Not identified'}
              </span>
              <span>
                {piece.category_name && `${piece.category_name} → `}
                {pieceBin(piece)}
              </span>
              <span className="text-xs">
                {time(piece.seen_at)} · <span className="font-mono">{piece.id.slice(0, 8)}</span>
                {piece.run_id ? (
                  <>
                    {' · '}
                    <Link
                      to={`/pieces/all?run=${piece.run_id}`}
                      onClick={onClose}
                      className="font-medium text-primary-text hover:underline"
                    >
                      More from run {piece.run_id}
                    </Link>
                  </>
                ) : null}
              </span>
            </div>
          </DialogDescription>
        </DialogHeader>
      </div>

      {piece.problems?.length > 0 && (
        <ul className="flex flex-col gap-2">
          {piece.problems.map((p, i) => (
            <li
              key={i}
              className="rounded-md border border-warning/40 bg-warning-bg px-3 py-2 text-sm text-warning"
            >
              <strong>{PROBLEM_LABELS[p.kind] ?? p.kind}:</strong> {p.message}
            </li>
          ))}
        </ul>
      )}

      {/* Its way through the machine. */}
      <section aria-labelledby="journey-way">
        <h3 id="journey-way" className="mb-3 font-display text-base font-semibold">
          Its way through the machine
        </h3>
        <ol className="flex flex-col gap-3">
          {journey ? (
            journey.tracks.map((t) => (
              <CameraStage key={`${t.channel}-${t.track_id}`} journey={journey} track={t} />
            ))
          ) : (
            <li className="rounded-lg border border-dashed border-border px-4 py-3 text-sm text-muted-foreground">
              {known === null
                ? 'Looking for its pictures from the cameras above…'
                : 'No pictures from the cameras above: they didn’t see it fall, or more than one piece could have been it.'}
            </li>
          )}
          {journey && (
            <li className="text-sm">
              <Link
                to={`/diagnose/journeys?id=${journey.id}`}
                onClick={onClose}
                className="font-medium text-primary-text hover:underline"
              >
                Every frame and decision, under Diagnose › Piece journeys
              </Link>
            </li>
          )}
          {pieceStages(piece).map((s) => (
            <li key={s.id} className="flex gap-3 rounded-lg border border-border px-4 py-3">
              {s.done ? (
                <Check className="mt-0.5 size-4 shrink-0 text-running" aria-label="Done" />
              ) : (
                <Circle className="mt-0.5 size-4 shrink-0 text-idle" aria-label="Not yet" />
              )}
              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-baseline justify-between gap-2">
                  <p className="text-sm font-medium">{s.title}</p>
                  {s.at && (
                    <span className="font-mono text-xs text-muted-foreground">{time(s.at)}</span>
                  )}
                </div>
                <p className="text-sm text-muted-foreground">{s.text}</p>
              </div>
            </li>
          ))}
        </ol>
      </section>

      {/* What was sent to Brickognize. */}
      <section aria-labelledby="journey-sent">
        <h3 id="journey-sent" className="mb-1 font-display text-base font-semibold">
          Sent to Brickognize
        </h3>
        {piece.images?.length > 0 ? (
          <>
            <p className="mb-3 text-sm text-muted-foreground">
              Outlined photos were sent; the others were taken but not used.
            </p>
            <div className="flex flex-wrap gap-2">
              {piece.images.map((img) => (
                <a
                  key={img.index}
                  href={sorterPieceImageURL(piece.id, img.index)}
                  target="_blank"
                  rel="noreferrer"
                  className={cn(
                    'block size-24 overflow-hidden rounded-md border-2 bg-media',
                    img.used ? 'border-primary' : 'border-transparent opacity-70',
                  )}
                  title={img.used ? 'Sent' : 'Not sent'}
                >
                  <img
                    src={sorterPieceImageURL(piece.id, img.index)}
                    alt={`Photo ${img.index + 1}${img.used ? ', sent' : ''}`}
                    className="size-full object-cover"
                  />
                </a>
              ))}
            </div>
          </>
        ) : (
          <p className="text-sm text-muted-foreground">No photos of this piece.</p>
        )}

        {matches.length > 0 && (
          <div className="mt-4">
            <p className="mb-2 text-sm font-medium">Its answer</p>
            <ul className="grid gap-2 sm:grid-cols-2">
              {matches.map((m, i) => (
                <li
                  key={`${m.id}-${m.rank ?? i}`}
                  className={cn(
                    'flex items-center gap-3 rounded-md border px-2 py-1.5',
                    i === 0 ? 'border-primary/50' : 'border-border',
                  )}
                >
                  {m.img_url ? (
                    <img
                      src={m.img_url}
                      alt=""
                      className="size-10 rounded bg-white object-contain"
                    />
                  ) : (
                    <span className="size-10 rounded bg-muted" />
                  )}
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-sm">{m.name}</span>
                    <span className="block truncate text-xs text-muted-foreground">
                      {m.id} · {m.category}
                    </span>
                  </span>
                  <span className="font-mono text-xs">{scorePct(m.score)}</span>
                </li>
              ))}
            </ul>
            {colors.length > 0 && (
              <p className="mt-2 text-sm text-muted-foreground">
                Colors: {colors.map((c) => `${c.name} ${scorePct(c.score)}`).join(', ')}
              </p>
            )}
          </div>
        )}
      </section>

      {prediction && (
        <div className="border-t border-border pt-5">
          <ReviewPanel key={piece.id} piece={piece} onSave={review} />
        </div>
      )}
    </div>
  );
}
