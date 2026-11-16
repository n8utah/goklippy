import { Check, Plus, X } from 'lucide-react';
import { useState } from 'react';

import { Button } from '@/components/ui/button';
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { cn } from '@/lib/utils';

/**
 * Picks several values from a searchable list, shown as chips. `options` is
 * [{ value, label, family?, seen }]; with `grouped`, options are grouped by family.
 * `allowCustom` offers what was typed when nothing matches (a size like "2 x 14").
 */
export default function MultiPicker({
  label,
  options,
  value,
  onChange,
  placeholder = 'Search…',
  grouped = false,
  allowCustom = false,
  disabled = false,
}) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState('');
  const selected = value ?? [];
  const labelOf = (v) => options.find((o) => o.value === v)?.label ?? v;
  const toggle = (v) =>
    onChange(selected.includes(v) ? selected.filter((s) => s !== v) : [...selected, v]);

  const groups = new Map();
  for (const o of options) {
    const key = grouped ? o.family || 'Other' : '';
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key).push(o);
  }
  const typed = search.trim();
  const exact = options.some((o) => o.label.toLowerCase() === typed.toLowerCase());

  return (
    <div className="flex flex-col gap-2">
      <div className="flex flex-wrap items-center gap-1.5">
        {selected.map((v) => (
          <span
            key={v}
            className="inline-flex items-center gap-1 rounded-md bg-muted py-0.5 pr-1 pl-2 text-sm"
          >
            {labelOf(v)}
            {!disabled && (
              <button
                type="button"
                aria-label={`Remove ${labelOf(v)}`}
                className="rounded p-0.5 text-muted-foreground hover:bg-accent hover:text-foreground"
                onClick={() => toggle(v)}
              >
                <X className="size-3.5" />
              </button>
            )}
          </span>
        ))}
        <Popover
          open={open}
          onOpenChange={(o) => {
            setOpen(o);
            if (!o) setSearch('');
          }}
        >
          <PopoverTrigger asChild>
            <Button variant="outline" size="sm" disabled={disabled} aria-label={`Add ${label}`}>
              <Plus />
              {selected.length ? 'Add' : `Pick ${label}`}
            </Button>
          </PopoverTrigger>
          <PopoverContent className="w-72 p-0" align="start">
            <Command>
              <CommandInput placeholder={placeholder} value={search} onValueChange={setSearch} />
              <CommandList>
                <CommandEmpty>Nothing matches.</CommandEmpty>
                {allowCustom && typed && !exact && (
                  <CommandGroup>
                    <CommandItem
                      value={`add ${typed}`}
                      onSelect={() => {
                        if (!selected.includes(typed)) onChange([...selected, typed]);
                        setSearch('');
                      }}
                    >
                      <Plus /> Add “{typed}”
                    </CommandItem>
                  </CommandGroup>
                )}
                {[...groups].map(([group, list]) => (
                  <CommandGroup key={group} heading={group || undefined}>
                    {list.map((o) => (
                      <CommandItem
                        key={o.value}
                        value={`${o.label} ${o.value}`}
                        onSelect={() => toggle(o.value)}
                      >
                        <Check
                          className={cn(selected.includes(o.value) ? 'opacity-100' : 'opacity-0')}
                        />
                        <span className="flex-1">{o.label}</span>
                        {o.seen > 0 && (
                          <span className="font-mono text-xs text-muted-foreground">
                            {o.seen.toLocaleString()}
                          </span>
                        )}
                      </CommandItem>
                    ))}
                  </CommandGroup>
                ))}
              </CommandList>
            </Command>
          </PopoverContent>
        </Popover>
      </div>
    </div>
  );
}
