import PieceCard from '@/components/PieceCard/PieceCard';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';

import { useBinPieces } from '../CollectionsPage.logic';

/** The pieces that went into one bin (or the bucket), as photos; one opens its journey. */
export default function BinPiecesDialog({ collectionId, bin, onClose, onOpenPiece }) {
  const { loading, pieces, total, error } = useBinPieces(collectionId, bin);
  const bucket = bin?.layer === 0;
  return (
    <Dialog open={Boolean(bin)} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-h-[85vh] overflow-y-auto sm:max-w-3xl">
        <DialogHeader>
          <DialogTitle>
            {bucket ? 'The bucket' : `Layer ${bin?.layer} · bin ${bin?.bin}`}
          </DialogTitle>
          <DialogDescription>
            {loading
              ? 'Loading…'
              : `${total.toLocaleString()} piece${total === 1 ? '' : 's'} since its bins started afresh${
                  total > pieces.length ? `, newest ${pieces.length} shown` : ''
                }.`}
          </DialogDescription>
        </DialogHeader>
        {error && <p className="text-sm text-destructive">{error}</p>}
        {!loading && !error && pieces.length === 0 && (
          <p className="text-sm text-muted-foreground">Nothing has gone in yet.</p>
        )}
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4">
          {pieces.map((p) => (
            <PieceCard key={p.id} piece={p} layout="tile" onOpen={() => onOpenPiece(p.id)} />
          ))}
        </div>
      </DialogContent>
    </Dialog>
  );
}
