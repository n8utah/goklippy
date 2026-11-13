import { TONE_BADGE } from '@/components/PieceCard/PieceCard';
import { Badge } from '@/components/ui/badge';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { pieceBin, pieceName, pieceStatus } from '@/helpers/sorterPieces';

import { piecePictures } from '../PiecesPage.logic';
import Picture from './Picture';

const when = (t) => (t ? new Date(t).toLocaleString() : '—');

function Problems({ piece }) {
  const n = piece.problems?.length ?? 0;
  return n > 0 ? (
    <span className="text-xs text-warning">
      {n} problem{n === 1 ? '' : 's'}
    </span>
  ) : null;
}

/** Every piece seen as cards; a card opens its journey. */
export function PieceGrid({ pieces, picture, onOpen }) {
  return (
    <ul className="grid grid-cols-2 gap-3 p-4 sm:grid-cols-3 md:grid-cols-4 xl:grid-cols-6">
      {pieces.map((p) => {
        const status = pieceStatus(p.status);
        return (
          <li key={p.id}>
            <button
              type="button"
              onClick={() => onOpen(p.id)}
              className="flex size-full flex-col gap-2 rounded-lg border border-border bg-background p-2 text-left transition-colors hover:border-border-strong focus-visible:border-primary"
            >
              <Picture
                {...piecePictures(p)}
                mode={picture}
                alt={pieceName(p)}
                className="aspect-square w-full"
              />
              <span className="block min-w-0 px-0.5 text-sm">
                <span className="line-clamp-2 font-medium">{pieceName(p)}</span>
                <span className="block truncate text-xs text-muted-foreground">
                  {[p.part_id, p.color_name].filter(Boolean).join(' · ') || ' '}
                </span>
                <span className="block truncate text-xs text-muted-foreground">
                  {when(p.seen_at)}
                </span>
              </span>
              <span className="mt-auto flex flex-wrap items-center gap-2">
                <Badge variant={TONE_BADGE[status.tone]}>{status.label}</Badge>
                <Problems piece={p} />
              </span>
            </button>
          </li>
        );
      })}
    </ul>
  );
}

/** Every piece seen as rows; a row opens its journey. */
export function PieceList({ pieces, picture, onOpen }) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead className="w-16 pl-5">
            <span className="sr-only">Picture</span>
          </TableHead>
          <TableHead>Piece</TableHead>
          <TableHead className="hidden md:table-cell">Color</TableHead>
          <TableHead className="hidden lg:table-cell">Category → bin</TableHead>
          <TableHead>Status</TableHead>
          <TableHead className="hidden pr-5 sm:table-cell">Seen</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {pieces.map((p) => {
          const status = pieceStatus(p.status);
          return (
            <TableRow key={p.id} className="cursor-pointer" onClick={() => onOpen(p.id)}>
              <TableCell className="pl-5">
                <Picture {...piecePictures(p)} mode={picture} className="size-12" />
              </TableCell>
              <TableCell className="whitespace-normal">
                <button
                  type="button"
                  className="text-left font-medium hover:underline"
                  onClick={(e) => {
                    e.stopPropagation();
                    onOpen(p.id);
                  }}
                >
                  {pieceName(p)}
                </button>
                {p.part_id && (
                  <span className="ml-2 font-mono text-xs text-muted-foreground">{p.part_id}</span>
                )}
              </TableCell>
              <TableCell className="hidden md:table-cell">{p.color_name || '—'}</TableCell>
              <TableCell className="hidden whitespace-normal lg:table-cell">
                {p.category_name ? `${p.category_name} → ` : ''}
                {pieceBin(p)}
              </TableCell>
              <TableCell>
                <Badge variant={TONE_BADGE[status.tone]}>{status.label}</Badge>{' '}
                <Problems piece={p} />
              </TableCell>
              <TableCell className="hidden pr-5 text-sm text-muted-foreground sm:table-cell">
                {when(p.seen_at)}
              </TableCell>
            </TableRow>
          );
        })}
      </TableBody>
    </Table>
  );
}
