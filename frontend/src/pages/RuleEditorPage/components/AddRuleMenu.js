import { Plus } from 'lucide-react';
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

/** Adds a rule: blank, a set rule, or one of the premade rules to change. */
export default function AddRuleMenu({ presets, onAdd, disabled }) {
  const [open, setOpen] = useState(false);
  const groups = new Map();
  for (const p of presets ?? []) {
    if (!groups.has(p.group)) groups.set(p.group, []);
    groups.get(p.group).push(p);
  }
  const add = (rule) => {
    onAdd(rule);
    setOpen(false);
  };

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button disabled={disabled}>
          <Plus /> Add a rule
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-80 p-0" align="start">
        <Command>
          <CommandInput placeholder="Search premade rules…" />
          <CommandList className="max-h-96">
            <CommandEmpty>Nothing matches.</CommandEmpty>
            <CommandGroup heading="Start from nothing">
              <CommandItem value="blank rule" onSelect={() => add({ name: 'New rule' })}>
                <span>
                  <span className="block font-medium">Blank rule</span>
                  <span className="block text-xs text-muted-foreground">
                    Pick the part types, sizes or colours it takes.
                  </span>
                </span>
              </CommandItem>
              <CommandItem
                value="set rule lego set number"
                onSelect={() => add({ name: 'Sets', kind: 'sets', sets: [] })}
              >
                <span>
                  <span className="block font-medium">Sets by number</span>
                  <span className="block text-xs text-muted-foreground">
                    Pull out what LEGO sets need, each into its own bins.
                  </span>
                </span>
              </CommandItem>
            </CommandGroup>
            {[...groups].map(([group, list]) => (
              <CommandGroup key={group} heading={group}>
                {list.map((p) => (
                  <CommandItem
                    key={p.key}
                    value={`${p.rule.name} ${p.rule.description ?? ''}`}
                    onSelect={() => add(p.rule)}
                  >
                    <span>
                      <span className="block font-medium">{p.rule.name}</span>
                      <span className="block text-xs text-muted-foreground">
                        {p.rule.description}
                      </span>
                    </span>
                  </CommandItem>
                ))}
              </CommandGroup>
            ))}
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  );
}
