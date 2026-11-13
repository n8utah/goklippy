import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';

import { partPictures, seenIn } from '../PiecesPage.logic';
import Picture from './Picture';

const day = (t) => (t ? new Date(t).toLocaleDateString() : '—');
const key = (p) => `${p.part_id}/${p.color_id}`;

/** Every part and colour as cards; a card opens the part. */
export function PartGrid({ parts, run, picture, onOpen }) {
  return (
    <ul className="grid grid-cols-2 gap-3 p-4 sm:grid-cols-3 md:grid-cols-4 xl:grid-cols-6">
      {parts.map((p) => (
        <li key={key(p)}>
          <button
            type="button"
            onClick={() => onOpen(p)}
            className="flex size-full flex-col gap-2 rounded-lg border border-border bg-background p-2 text-left transition-colors hover:border-border-strong focus-visible:border-primary"
          >
            <span className="relative block w-full">
              <Picture
                {...partPictures(p)}
                mode={picture}
                alt={p.part_name}
                className="aspect-square w-full"
              />
              <span className="absolute top-1.5 right-1.5 rounded-full bg-primary px-2 py-0.5 font-mono text-xs font-semibold text-primary-foreground">
                ×{seenIn(p, run).toLocaleString()}
              </span>
            </span>
            <span className="block min-w-0 px-0.5 text-sm">
              <span className="line-clamp-2 font-medium" title={p.part_name}>
                {p.part_name || p.part_id}
              </span>
              <span className="block truncate text-xs text-muted-foreground">
                <span className="font-mono">{p.part_id}</span>
                {p.color_name && ` · ${p.color_name}`}
              </span>
              {p.category && (
                <span className="block truncate text-xs text-muted-foreground">{p.category}</span>
              )}
            </span>
          </button>
        </li>
      ))}
    </ul>
  );
}

/** Every part and colour as rows; a row opens the part. */
export function PartList({ parts, run, picture, onOpen }) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead className="w-16 pl-5">
            <span className="sr-only">Picture</span>
          </TableHead>
          <TableHead>Part</TableHead>
          <TableHead className="hidden md:table-cell">Color</TableHead>
          <TableHead className="hidden lg:table-cell">Category</TableHead>
          <TableHead className="text-right">Seen</TableHead>
          <TableHead className="hidden sm:table-cell">Runs</TableHead>
          <TableHead className="hidden pr-5 xl:table-cell">First · last seen</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {parts.map((p) => {
          const n = seenIn(p, run);
          return (
            <TableRow key={key(p)} className="cursor-pointer" onClick={() => onOpen(p)}>
              <TableCell className="pl-5">
                <Picture {...partPictures(p)} mode={picture} className="size-12" />
              </TableCell>
              <TableCell className="whitespace-normal">
                <button
                  type="button"
                  className="text-left font-medium hover:underline"
                  onClick={(e) => {
                    e.stopPropagation();
                    onOpen(p);
                  }}
                >
                  {p.part_name || p.part_id}
                </button>
                <span className="block font-mono text-xs text-muted-foreground">
                  {p.part_id}
                  <span className="md:hidden">{p.color_name && ` · ${p.color_name}`}</span>
                </span>
              </TableCell>
              <TableCell className="hidden md:table-cell">{p.color_name || '—'}</TableCell>
              <TableCell className="hidden whitespace-normal lg:table-cell">
                {p.category || '—'}
              </TableCell>
              <TableCell className="text-right font-mono">
                {n.toLocaleString()}
                {run && n !== p.seen && (
                  <span className="text-xs text-muted-foreground"> of {p.seen}</span>
                )}
              </TableCell>
              <TableCell className="hidden font-mono text-xs whitespace-normal sm:table-cell">
                {p.runs.map((r) => `#${r.run_id}${r.seen > 1 ? ` ×${r.seen}` : ''}`).join(' ') ||
                  '—'}
              </TableCell>
              <TableCell className="hidden pr-5 text-sm text-muted-foreground xl:table-cell">
                {day(p.first_seen)} · {day(p.last_seen)}
              </TableCell>
            </TableRow>
          );
        })}
      </TableBody>
    </Table>
  );
}
