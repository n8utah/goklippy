import { useState } from 'react';

import { Checkbox } from '@/components/ui/checkbox';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Label } from '@/components/ui/label';
import { cn } from '@/lib/utils';

import { coverText, setShare } from '../CollectionPage.logic';
import { Pic } from './PieceTile';
import SetCover from './SetCover';

function PartRow({ line }) {
  const missing = line.need - line.have;
  return (
    <li className="flex items-center gap-3 py-2">
      <Pic src={line.img_url} alt="" className="size-12 shrink-0 bg-white" />
      <span className="min-w-0 flex-1 text-sm">
        <span className="line-clamp-2 font-medium" title={line.part_name}>
          {line.part_name}
        </span>
        <span className="flex items-center gap-1.5 truncate text-xs text-muted-foreground">
          <span className="font-mono">{line.part_num}</span>·
          {/* The part's real LEGO colour, so it's data, not a theme colour. */}
          <span
            className="inline-block size-3 shrink-0 rounded-full border border-border-strong"
            style={{ backgroundColor: line.rgb ? `#${line.rgb}` : undefined }}
            aria-hidden="true"
          />
          {line.color_name}
        </span>
      </span>
      <span className="flex shrink-0 flex-col items-end text-sm">
        <span className={cn('font-mono font-medium', missing === 0 && 'text-running-text')}>
          {line.have}/{line.need}
        </span>
        {missing > 0 && <span className="text-xs text-muted-foreground">missing {missing}</span>}
        {missing > 0 && line.other_colours > 0 && (
          <span className="text-xs text-muted-foreground">
            {line.other_colours} other colour{line.other_colours === 1 ? '' : 's'}
          </span>
        )}
      </span>
    </li>
  );
}

/** One set's parts against the collection, the missing ones first. */
export default function SetDetail({ open, detail, any, onClose }) {
  const [missingOnly, setMissingOnly] = useState(false);
  const set = detail?.set;
  const lines = [...(detail?.parts ?? [])].sort(
    (a, b) => (a.have >= a.need) - (b.have >= b.need) || b.need - b.have - (a.need - a.have),
  );
  const shown = missingOnly ? lines.filter((l) => l.have < l.need) : lines;

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="flex max-h-[calc(100dvh-2rem)] flex-col gap-3 p-4 sm:max-w-2xl sm:p-6">
        <DialogHeader className="text-left">
          <DialogTitle className="pr-6 leading-snug">{set?.name ?? 'Set'}</DialogTitle>
          <DialogDescription>
            {set ? (
              <>
                <span className="font-mono">{set.set_num}</span> · {set.year}
                {set.theme && ` · ${set.theme}`}
              </>
            ) : detail?.error ? (
              detail.error
            ) : (
              'Loading…'
            )}
          </DialogDescription>
        </DialogHeader>
        {set && (
          <>
            <div className="flex gap-3">
              <Pic
                src={set.img_url}
                alt=""
                className="aspect-[4/3] w-28 shrink-0 bg-white sm:w-36"
              />
              <div className="flex min-w-0 flex-1 flex-col justify-center gap-2">
                <p className="font-mono text-2xl font-semibold text-primary-text">
                  {setShare(set, any)}%
                </p>
                <SetCover set={set} />
                <p className="text-sm text-muted-foreground">{coverText(set, any)}</p>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <Checkbox
                id="missing-only"
                checked={missingOnly}
                onCheckedChange={(v) => setMissingOnly(v === true)}
              />
              <Label htmlFor="missing-only" className="text-sm">
                Only what’s missing
              </Label>
            </div>
            <ul className="-mx-1 min-h-0 flex-1 divide-y divide-border overflow-y-auto px-1">
              {shown.map((l) => (
                <PartRow key={`${l.part_num}/${l.color_id}`} line={l} />
              ))}
              {shown.length === 0 && (
                <li className="py-4 text-sm text-muted-foreground">Nothing missing.</li>
              )}
            </ul>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}
