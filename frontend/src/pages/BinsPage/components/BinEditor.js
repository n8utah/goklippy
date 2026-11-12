import { useMemo, useState } from 'react';

import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet';

/**
 * Chooses a bin's categories. A category already in another bin moves here when saved.
 * The parent remounts it (key) for each bin, so the draft starts fresh.
 */
export default function BinEditor({ bin, layer, categories, binOf, onSave, onPoint, onEmpty }) {
  const [chosen, setChosen] = useState(() => bin.categories.map((c) => c.id));
  const [filter, setFilter] = useState('');

  const shown = useMemo(() => {
    const q = filter.trim().toLowerCase();
    return q
      ? categories.filter((c) => c.name.toLowerCase().includes(q) || c.id.toLowerCase().includes(q))
      : categories;
  }, [categories, filter]);

  const toggle = (id) =>
    setChosen((prev) => (prev.includes(id) ? prev.filter((c) => c !== id) : [...prev, id]));

  return (
    <div className="flex h-full flex-col gap-4">
      <SheetHeader className="p-0">
        <SheetTitle className="font-display text-xl">
          Layer {layer}, bin {bin.bin}
        </SheetTitle>
        <SheetDescription>
          Section {bin.section}, bin {bin.in_section} of it, at {bin.angle.toFixed(1)}°. {bin.count}{' '}
          piece{bin.count === 1 ? '' : 's'} since it was last emptied
          {bin.full ? '; full, so it takes no more' : ''}.
        </SheetDescription>
      </SheetHeader>
      {!bin.reachable && (
        <p className="rounded-md bg-warning-bg px-3 py-2 text-sm text-warning">
          The chute can’t reach this bin, so it gets no pieces.
        </p>
      )}

      <div className="flex flex-wrap gap-2">
        <Button variant="outline" size="sm" disabled={!bin.reachable} onClick={onPoint}>
          Point the chute here
        </Button>
        <Button variant="outline" size="sm" disabled={bin.count === 0} onClick={onEmpty}>
          I emptied it
        </Button>
      </div>

      <div className="flex min-h-0 flex-1 flex-col gap-2">
        <label htmlFor="bin-filter" className="text-sm font-medium">
          Categories it takes ({chosen.length})
        </label>
        <Input
          id="bin-filter"
          placeholder="Find a category"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
        />
        <ul className="min-h-0 flex-1 overflow-y-auto rounded-md border border-border">
          {shown.map((c) => {
            const elsewhere = binOf[c.id]?.filter((b) => !(b.layer === layer && b.bin === bin.bin));
            return (
              <li key={c.id} className="border-b border-border last:border-0">
                <label className="flex cursor-pointer items-center gap-3 px-3 py-2 text-sm hover:bg-accent">
                  <Checkbox checked={chosen.includes(c.id)} onCheckedChange={() => toggle(c.id)} />
                  <span className="min-w-0 flex-1 truncate">{c.name}</span>
                  {elsewhere?.length > 0 && (
                    <span className="shrink-0 text-xs text-muted-foreground">
                      now in {elsewhere.map((b) => `L${b.layer} bin ${b.bin}`).join(', ')}
                    </span>
                  )}
                </label>
              </li>
            );
          })}
          {shown.length === 0 && (
            <li className="px-3 py-2 text-sm text-muted-foreground">No categories match.</li>
          )}
        </ul>
      </div>

      <div className="flex gap-2 border-t border-border pt-4">
        <Button onClick={() => onSave(chosen)}>Save</Button>
        <Button variant="outline" onClick={() => onSave([])}>
          Unassign
        </Button>
      </div>
    </div>
  );
}
