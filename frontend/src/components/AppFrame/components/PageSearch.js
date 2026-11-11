import { useNavigate } from 'react-router';

import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command';
import { NAV_GROUPS } from '@/helpers/navigation';

/**
 * Ctrl+K search. For now it finds pages and their parts; settings and actions join it
 * with the settings registry (build step 3).
 */
export default function PageSearch({ open, onOpenChange }) {
  const navigate = useNavigate();
  const go = (path) => {
    onOpenChange(false);
    navigate(path);
  };

  return (
    <CommandDialog
      open={open}
      onOpenChange={onOpenChange}
      title="Search"
      description="Find a page and go to it"
    >
      <CommandInput placeholder="Search pages…" />
      <CommandList>
        <CommandEmpty>No page matches.</CommandEmpty>
        {NAV_GROUPS.map((group) => (
          <CommandGroup key={group.id} heading={group.label}>
            {group.pages.flatMap((page) => [
              <CommandItem
                key={page.id}
                value={`${group.label} ${page.label} ${page.about}`}
                onSelect={() => go(page.path)}
              >
                <group.icon className="text-muted-foreground" aria-hidden />
                <span>{page.label}</span>
                <span className="ml-auto truncate text-xs text-muted-foreground">{page.about}</span>
              </CommandItem>,
              ...(page.parts || []).slice(1).map((part) => (
                <CommandItem
                  key={`${page.id}-${part.id}`}
                  value={`${group.label} ${page.label} ${part.label}`}
                  onSelect={() => go(`${page.path}/${part.id}`)}
                >
                  <span className="pl-6">
                    {page.label} › {part.label}
                  </span>
                </CommandItem>
              )),
            ])}
          </CommandGroup>
        ))}
      </CommandList>
    </CommandDialog>
  );
}
