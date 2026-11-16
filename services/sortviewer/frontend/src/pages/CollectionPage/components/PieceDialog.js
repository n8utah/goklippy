import { ExternalLink } from 'lucide-react';
import { createContext, useCallback, useContext, useMemo, useState } from 'react';

import { Badge } from '@/components/ui/badge';
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '@/components/ui/dialog';
import { photoURL } from '@/helpers/api';
import { cn } from '@/lib/utils';

import { pieceBin, pieceName } from '../CollectionPage.logic';

const PieceDialogContext = createContext(() => {});

/**
 * Lets any piece on the page open the piece dialog: wrap the page, then
 * usePieceDialog()(piece) opens it.
 */
export function PieceDialogProvider({ children }) {
  const [piece, setPiece] = useState(null);
  const open = useCallback((p) => setPiece(p), []);
  return (
    <PieceDialogContext.Provider value={open}>
      {children}
      <PieceDialog piece={piece} onClose={() => setPiece(null)} />
    </PieceDialogContext.Provider>
  );
}

/** Opens the piece dialog for a piece. */
export const usePieceDialog = () => useContext(PieceDialogContext);

/** A big picture, or a quiet note when there isn't one (yet). */
function BigPic({ src, alt, empty, className }) {
  const [failed, setFailed] = useState('');
  if (!src || failed === src) {
    return (
      <div
        className={cn(
          'flex items-center justify-center rounded-lg bg-muted p-4 text-center text-sm text-muted-foreground',
          className,
        )}
      >
        {empty}
      </div>
    );
  }
  return (
    <div className={cn('overflow-hidden rounded-lg', className)}>
      <img
        src={src}
        alt={alt}
        className="size-full object-contain"
        onError={() => setFailed(src)}
      />
    </div>
  );
}

function Row({ label, children }) {
  if (!children) return null;
  return (
    <div className="flex justify-between gap-4 border-b border-border py-1.5 text-sm last:border-0">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="min-w-0 text-right">{children}</dd>
    </div>
  );
}

const when = (t) => (t ? new Date(t).toLocaleString() : '');

const STATUS = {
  tracking: 'Just landed',
  identifying: 'Being named',
  identified: 'Named, on its way to a bin',
  unidentified: 'Couldn’t be named',
  id_failed: 'Couldn’t be named this time',
  distributed: 'Sorted',
  multi_drop: 'Came with another piece',
  lost: 'Lost from view',
};

/** One piece in full: our photo big, the official picture, and what's known of it. */
function PieceDialog({ piece, onClose }) {
  const p = piece;
  const unknown = p && !p.part_id;
  const bin = p ? pieceBin(p) : '';
  return (
    <Dialog open={Boolean(p)} onOpenChange={(open) => !open && onClose()}>
      {p && (
        <DialogContent className="max-h-[92dvh] overflow-y-auto sm:max-w-3xl">
          <DialogTitle className="pr-8">{pieceName(p)}</DialogTitle>
          <DialogDescription className="flex flex-wrap items-center gap-1.5">
            {unknown ? (
              <Badge variant="warning">Unknown</Badge>
            ) : (
              <span className="font-mono">{p.part_id}</span>
            )}
            {p.color_name && <span>· {p.color_name}</span>}
          </DialogDescription>
          <div className="grid gap-3 sm:grid-cols-[3fr_2fr]">
            <figure className="flex flex-col gap-1">
              <BigPic
                src={photoURL(p.id)}
                alt="Photo from the sorter"
                empty="Our photo of it is on its way."
                className="aspect-square bg-media"
              />
              <figcaption className="text-xs text-muted-foreground">
                Our photo, on the sorter
              </figcaption>
            </figure>
            <figure className="flex flex-col gap-1">
              <BigPic
                src={p.part_img_url}
                alt={`Official picture of ${pieceName(p)}`}
                empty={
                  unknown ? 'Not named, so there’s no official picture.' : 'No official picture.'
                }
                className="aspect-square bg-white"
              />
              <figcaption className="text-xs text-muted-foreground">
                The part, as LEGO makes it (any colour)
              </figcaption>
            </figure>
          </div>
          <dl>
            <Row label="Part">
              {p.part_id && `${p.part_id}${p.part_category ? ` · ${p.part_category}` : ''}`}
            </Row>
            <Row label="Colour">{p.color_name}</Row>
            <Row label="Sure it’s this part">
              {p.part_id && p.part_score > 0 ? `${Math.round(p.part_score * 100)}%` : ''}
            </Row>
            <Row label="Went to">
              {bin && `${bin}${p.category_name ? ` · ${p.category_name}` : ''}`}
            </Row>
            <Row label="Status">{STATUS[p.status] ?? p.status}</Row>
            <Row label="Seen">{when(p.seen_at)}</Row>
            <Row label="Sorted">{when(p.sorted_at)}</Row>
          </dl>
          {p.part_id && (
            <a
              href={`https://www.bricklink.com/v2/catalog/catalogitem.page?P=${encodeURIComponent(p.part_id)}`}
              target="_blank"
              rel="noreferrer"
              className="flex items-center gap-1 self-start text-sm font-medium text-primary-text hover:underline"
            >
              This part on BrickLink <ExternalLink className="size-3.5" />
            </a>
          )}
        </DialogContent>
      )}
    </Dialog>
  );
}

/** Props that make an element open a piece's dialog, keyboard included. */
export function usePieceButton() {
  const open = usePieceDialog();
  return useMemo(
    () => (piece) => ({
      role: 'button',
      tabIndex: 0,
      onClick: () => open(piece),
      onKeyDown: (e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          open(piece);
        }
      },
    }),
    [open],
  );
}
