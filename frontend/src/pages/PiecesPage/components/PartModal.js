import { useEffect, useState } from 'react';

import { useJourneyModal } from '@/components/JourneyModal/JourneyModal.logic';
import PieceCard from '@/components/PieceCard/PieceCard';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { getSorterCatalogPart, sorterPieceImageURL } from '@/helpers/api';
import { scorePct } from '@/helpers/sorterPieces';
import { cn } from '@/lib/utils';

import Picture from './Picture';

const when = (t) => (t ? new Date(t).toLocaleString() : '—');

/** The part in full, loaded when it opens. */
function usePart(target) {
  const [state, setState] = useState({ part: null, error: '' });
  useEffect(() => {
    if (!target) return undefined;
    let cancelled = false;
    getSorterCatalogPart(target.part_id, target.color_id).then(
      (part) => !cancelled && setState({ part, error: '' }),
      (err) => !cancelled && setState({ part: null, error: err.message }),
    );
    return () => {
      cancelled = true;
    };
  }, [target]);
  const same = state.part?.part_id === target?.part_id && state.part?.color_id === target?.color_id;
  return { part: same ? state.part : null, error: state.error };
}

function Fact({ label, children }) {
  return (
    <div>
      <dt className="text-xs text-muted-foreground">{label}</dt>
      <dd className="text-sm">{children}</dd>
    </div>
  );
}

/** Every kept camera photo: one large, the rest to pick from. */
function Photos({ photos, onOpenPiece }) {
  const [pick, setPick] = useState(0);
  if (photos.length === 0) {
    return (
      <p className="text-sm text-muted-foreground">
        No camera photos kept of it: they were cleared, or its pieces were sorted before photos were
        saved.
      </p>
    );
  }
  const shown = photos[Math.min(pick, photos.length - 1)];
  return (
    <div className="flex flex-col gap-3">
      <figure className="flex flex-col gap-2">
        {/* A photo, so its frame is the same in both themes. */}
        <div className="flex max-h-[50vh] justify-center overflow-hidden rounded-lg bg-media">
          <img
            src={sorterPieceImageURL(shown.piece_id, shown.index)}
            alt="Camera photo of the part"
            className="max-h-[50vh] object-contain"
          />
        </div>
        <figcaption className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
          <span>{when(shown.taken_at)}</span>
          {shown.run_id ? <span>Run {shown.run_id}</span> : null}
          {shown.used && <span>Sent to Brickognize</span>}
          <Button
            variant="link"
            size="xs"
            className="h-auto p-0"
            onClick={() => onOpenPiece(shown.piece_id)}
          >
            Its piece’s journey
          </Button>
        </figcaption>
      </figure>
      <ul className="grid grid-cols-5 gap-2 sm:grid-cols-8">
        {photos.map((ph, i) => (
          <li key={`${ph.piece_id}/${ph.index}`}>
            <button
              type="button"
              onClick={() => setPick(i)}
              aria-label={`Photo ${i + 1}`}
              aria-pressed={i === pick}
              className={cn(
                'block aspect-square w-full overflow-hidden rounded-md border-2 bg-media',
                i === pick ? 'border-primary' : 'border-transparent hover:border-border-strong',
              )}
            >
              <img
                src={sorterPieceImageURL(ph.piece_id, ph.index)}
                alt=""
                loading="lazy"
                className="size-full object-cover"
              />
            </button>
          </li>
        ))}
      </ul>
    </div>
  );
}

/**
 * One part and colour in full: Brickognize's picture and what it is, how many and when,
 * which runs (each opens its pieces), every camera photo kept of it, and its pieces
 * (each opens its journey).
 */
export default function PartModal({ target, onClose, onShowRun }) {
  const { part, error } = usePart(target);
  const journey = useJourneyModal();

  return (
    <Dialog open={Boolean(target)} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-h-[92dvh] overflow-y-auto sm:max-w-3xl">
        <DialogHeader className="flex-row items-start gap-4 text-left">
          {target && (
            <Picture
              official={target.img_url}
              camera=""
              mode="official"
              alt={target.part_name}
              className="size-28 sm:size-36"
            />
          )}
          <div className="min-w-0 flex-1">
            <DialogTitle className="text-xl leading-tight">
              {target?.part_name || target?.part_id}
            </DialogTitle>
            <DialogDescription className="mt-1">
              <span className="font-mono">{target?.part_id}</span>
              {target?.color_name && ` · ${target.color_name}`}
              {target?.category && ` · ${target.category}`}
            </DialogDescription>
            {part && (
              <dl className="mt-3 grid grid-cols-2 gap-x-4 gap-y-2 sm:grid-cols-4">
                <Fact label="Seen">
                  <span className="font-mono">{part.seen.toLocaleString()}</span>
                </Fact>
                <Fact label="Best match">{scorePct(part.best_score) || '—'}</Fact>
                <Fact label="First seen">{when(part.first_seen)}</Fact>
                <Fact label="Last seen">{when(part.last_seen)}</Fact>
              </dl>
            )}
          </div>
        </DialogHeader>

        {error && <p className="text-sm text-destructive">{error}</p>}
        {!part && !error && <p className="text-sm text-muted-foreground">Loading…</p>}

        {part && (
          <>
            <section className="flex flex-col gap-2">
              <h3 className="font-medium">Runs</h3>
              {part.runs.length === 0 ? (
                <p className="text-sm text-muted-foreground">Not tied to a run.</p>
              ) : (
                <ul className="flex flex-wrap gap-2">
                  {part.runs.map((r) => (
                    <li key={r.run_id}>
                      <Button variant="outline" size="sm" onClick={() => onShowRun(r.run_id)}>
                        Run {r.run_id}
                        <span className="font-mono text-muted-foreground">×{r.seen}</span>
                      </Button>
                    </li>
                  ))}
                </ul>
              )}
            </section>

            <section className="flex flex-col gap-2">
              <h3 className="font-medium">
                Camera photos{' '}
                <span className="font-normal text-muted-foreground">({part.photos.length})</span>
              </h3>
              <Photos
                key={`${part.part_id}/${part.color_id}`}
                photos={part.photos}
                onOpenPiece={journey.open}
              />
            </section>

            <section className="flex flex-col gap-2">
              <h3 className="font-medium">
                Its pieces{' '}
                <span className="font-normal text-muted-foreground">
                  ({part.pieces.length}
                  {part.pieces.length < part.seen ? ` of ${part.seen} still in the records` : ''})
                </span>
              </h3>
              {part.pieces.length === 0 ? (
                <p className="text-sm text-muted-foreground">None left in the records.</p>
              ) : (
                <div className="grid gap-2 sm:grid-cols-2">
                  {part.pieces.map((p) => (
                    <PieceCard key={p.id} piece={p} onOpen={() => journey.open(p.id)} />
                  ))}
                </div>
              )}
            </section>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}
