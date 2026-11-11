import { useState } from 'react';

import { Badge } from '@/components/ui/badge';
import { sorterPieceImageURL } from '@/helpers/api';
import { pieceBin, pieceName, pieceStatus, scorePct } from '@/helpers/sorterPieces';
import { cn } from '@/lib/utils';

// sorterPieces' tones as badge variants.
export const TONE_BADGE = {
  muted: 'idle',
  busy: 'secondary',
  good: 'running',
  warn: 'warning',
  bad: 'destructive',
};

/** A piece's first photo, or an empty square until it's saved. */
export function PieceThumb({ piece, className }) {
  const [failed, setFailed] = useState(false);
  // Photos are saved a moment after they're taken; until then there's nothing to show.
  const has = !failed && piece.status !== 'tracking';
  return (
    <span
      className={cn('block shrink-0 overflow-hidden rounded-md bg-media', className)}
      aria-hidden
    >
      {has && (
        <img
          src={sorterPieceImageURL(piece.id, 0)}
          alt=""
          className="size-full object-cover"
          onError={() => setFailed(true)}
        />
      )}
    </span>
  );
}

/**
 * One piece: photo, name, part and color, where it went, and its status. `layout` is
 * "tile" (a strip of recent pieces) or "row" (lists). Clicking opens its journey.
 */
export default function PieceCard({ piece, onOpen, layout = 'row' }) {
  const status = pieceStatus(piece.status);
  const detail = [
    piece.part_id && `${piece.part_id} ${scorePct(piece.part_score)}`.trim(),
    piece.color_name,
  ]
    .filter(Boolean)
    .join(' · ');

  if (layout === 'tile') {
    return (
      <button
        type="button"
        onClick={() => onOpen?.(piece)}
        className="flex w-40 shrink-0 flex-col gap-2 rounded-lg border border-border bg-card p-2 text-left transition-colors hover:border-border-strong focus-visible:border-primary"
      >
        <PieceThumb piece={piece} className="aspect-square w-full" />
        <span className="flex min-w-0 flex-col gap-0.5 px-0.5">
          <span className="truncate text-sm font-medium">{pieceName(piece)}</span>
          <span className="truncate text-xs text-muted-foreground">{detail || ' '}</span>
          <span className="truncate text-xs">{pieceBin(piece)}</span>
        </span>
        <Badge variant={TONE_BADGE[status.tone]} className="self-start">
          {status.label}
        </Badge>
      </button>
    );
  }

  return (
    <button
      type="button"
      onClick={() => onOpen?.(piece)}
      className="flex w-full items-center gap-3 rounded-lg border border-border bg-card p-2 text-left transition-colors hover:border-border-strong"
    >
      <PieceThumb piece={piece} className="size-14" />
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate text-sm font-medium">{pieceName(piece)}</span>
        {detail && <span className="truncate text-xs text-muted-foreground">{detail}</span>}
        <span className="truncate text-xs text-muted-foreground">
          {piece.category_name && `${piece.category_name} → `}
          {pieceBin(piece)}
        </span>
      </span>
      <Badge variant={TONE_BADGE[status.tone]}>{status.label}</Badge>
    </button>
  );
}
