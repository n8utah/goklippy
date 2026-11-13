import { useState } from 'react';

import { Badge } from '@/components/ui/badge';
import { photoURL } from '@/helpers/api';
import { cn } from '@/lib/utils';

import { pieceBin, pieceName } from '../CollectionPage.logic';

/** A picture that quietly gives way to an empty square when it can't load. */
export function Pic({ src, alt, className }) {
  const [failed, setFailed] = useState(false);
  return (
    <span className={cn('block overflow-hidden rounded-md bg-muted', className)}>
      {src && !failed && (
        <img
          src={src}
          alt={alt}
          loading="lazy"
          className="size-full object-contain"
          onError={() => setFailed(true)}
        />
      )}
    </span>
  );
}

/** One piece: our photo of it beside the official picture of the part, its name and colour. */
export default function PieceTile({ piece, fresh }) {
  const unknown = !piece.part_id;
  return (
    <div
      className={cn(
        'flex flex-col gap-2 rounded-xl border bg-card p-2 transition-colors',
        fresh ? 'border-primary' : 'border-border',
      )}
    >
      <div className="grid grid-cols-2 gap-1.5">
        <Pic
          src={piece.has_photo ? photoURL(piece.id) : ''}
          alt="Photo from the sorter"
          className="aspect-square"
        />
        <Pic
          src={piece.part_img_url}
          alt={unknown ? '' : `Official picture of ${pieceName(piece)}`}
          className="aspect-square bg-white"
        />
      </div>
      <div className="min-w-0 px-0.5">
        <p className="line-clamp-2 text-sm font-medium" title={pieceName(piece)}>
          {pieceName(piece)}
        </p>
        <p className="flex items-center gap-1.5 truncate text-xs text-muted-foreground">
          {unknown ? (
            <Badge variant="warning">Unknown</Badge>
          ) : (
            <>
              <span className="font-mono">{piece.part_id}</span>
              {piece.color_name && <span className="truncate">· {piece.color_name}</span>}
            </>
          )}
        </p>
        {pieceBin(piece) && (
          <p className="truncate text-xs text-muted-foreground">
            {pieceBin(piece)}
            {piece.category_name && ` · ${piece.category_name}`}
          </p>
        )}
      </div>
    </div>
  );
}
