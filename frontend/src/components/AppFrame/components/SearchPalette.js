import { defaultFilter } from 'cmdk';
import { Clock, SlidersHorizontal, Zap } from 'lucide-react';
import { useState } from 'react';
import { useNavigate } from 'react-router';

import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command';
import { findLocation, NAV_GROUPS } from '@/helpers/navigation';
import { useRecent } from '@/helpers/recent';
import { ALL_SETTINGS, findSetting, formatSettingValue } from '@/helpers/settingsRegistry';
import { useSettingValues } from '@/helpers/useSetting';

const SUGGESTED = 3;

// cmdk matches letters in order anywhere ("speed" in "stopping, an error or the backend");
// matches that scattered are noise, so they're dropped.
const MIN_SCORE = 0.05;

// A leading ">" searches actions only; the filter ignores it.
const ACTIONS_PREFIX = /^\s*>\s*/;
const filter = (value, search, keywords) => {
  const s = search.replace(ACTIONS_PREFIX, '');
  if (!s) return 1;
  const score = defaultFilter(value, s, keywords);
  return score >= MIN_SCORE ? score : 0;
};

/** Every page and part, as search entries: { path, label, place, icon, keywords }. */
const PAGE_ENTRIES = NAV_GROUPS.flatMap((group) =>
  group.pages.flatMap((page) => [
    {
      path: page.path,
      label: page.label,
      place: group.label,
      about: page.about,
      icon: group.icon,
      keywords: [group.label, page.about],
    },
    ...(page.parts || []).slice(1).map((part) => ({
      path: `${page.path}/${part.id}`,
      label: part.label,
      place: `${group.label} › ${page.label}`,
      icon: group.icon,
      keywords: [group.label, page.label],
    })),
  ]),
);

/** A recent page's entry, by its path. */
function pageEntry(path) {
  const found = PAGE_ENTRIES.find((p) => p.path === path);
  if (found) return found;
  const { group, page, part } = findLocation(path);
  if (!page) return null;
  return {
    path,
    label: part ? part.label : page.label,
    place: part ? `${group.label} › ${page.label}` : group.label,
    icon: group.icon,
  };
}

/**
 * Search (Ctrl+K or "/"): settings, pages and actions. Empty, it shows what was opened
 * last and up to three actions that fit the machine's state.
 */
export default function SearchPalette({ open, onOpenChange, actions, onOpenSetting }) {
  const navigate = useNavigate();
  const [query, setQuery] = useState('');
  const { valueOf } = useSettingValues();
  const recent = useRecent();
  const actionsOnly = query.trimStart().startsWith('>');
  const empty = query.trim() === '';

  const close = () => {
    onOpenChange(false);
    setQuery('');
  };
  const goTo = (path) => {
    close();
    navigate(path);
  };
  const openSetting = (key) => {
    setQuery('');
    onOpenSetting(key);
  };
  const runAction = (action) => {
    close();
    action.run();
  };

  const settingItem = (s, value) => (
    <CommandItem
      key={`setting-${s.key}`}
      value={`${s.label} ${s.section.label}`}
      keywords={[s.section.label, s.keywords || '']}
      onSelect={() => openSetting(s.key)}
    >
      <SlidersHorizontal aria-hidden />
      <Entry label={s.label} place={`Settings › ${s.section.label}`} />
      <span className="ml-auto shrink-0 font-mono text-xs text-muted-foreground">
        {formatSettingValue(s, value)}
      </span>
    </CommandItem>
  );

  const pageItem = (p, keyPrefix = 'page') => (
    <CommandItem
      key={`${keyPrefix}-${p.path}`}
      value={`${p.label} ${p.place}`}
      keywords={p.keywords}
      onSelect={() => goTo(p.path)}
    >
      <p.icon aria-hidden />
      <Entry label={p.label} place={p.place} />
    </CommandItem>
  );

  const actionItem = (a) => (
    <CommandItem
      key={`action-${a.id}`}
      value={a.label}
      keywords={[a.keywords]}
      disabled={!a.allowed}
      onSelect={() => runAction(a)}
    >
      <Zap aria-hidden />
      <Entry
        label={a.label}
        place={!a.allowed ? a.why : a.moves ? 'Moves the machine' : 'Action'}
      />
    </CommandItem>
  );

  return (
    <CommandDialog
      open={open}
      onOpenChange={(next) => (next ? onOpenChange(true) : close())}
      title="Search"
      description="Find a setting, a page or an action"
      commandProps={{ filter }}
      className="sm:max-w-xl max-sm:top-0 max-sm:left-0 max-sm:h-dvh max-sm:max-w-none max-sm:translate-x-0 max-sm:translate-y-0 max-sm:rounded-none max-sm:border-0"
    >
      <CommandInput
        value={query}
        onValueChange={setQuery}
        placeholder="Search settings, pages and actions…  (> for actions)"
      />
      <CommandList className="max-h-[min(60vh,480px)] max-sm:max-h-none max-sm:flex-1">
        <CommandEmpty>Nothing matches.</CommandEmpty>
        {empty ? (
          <>
            {recent.length > 0 && (
              <CommandGroup heading="Recent">
                {recent.map((r) => {
                  if (r.type === 'setting') {
                    const s = findSetting(r.id);
                    return s && settingItem(s, valueOf(s));
                  }
                  const p = pageEntry(r.id);
                  return p && pageItem({ ...p, icon: p.icon || Clock }, 'recent');
                })}
              </CommandGroup>
            )}
            <CommandGroup heading="Suggested">
              {actions
                .filter((a) => a.allowed)
                .slice(0, SUGGESTED)
                .map(actionItem)}
            </CommandGroup>
          </>
        ) : (
          <>
            {!actionsOnly && (
              <>
                <CommandGroup heading="Settings">
                  {ALL_SETTINGS.map((s) => settingItem(s, valueOf(s)))}
                </CommandGroup>
                <CommandGroup heading="Pages">{PAGE_ENTRIES.map((p) => pageItem(p))}</CommandGroup>
              </>
            )}
            <CommandGroup heading="Actions">
              {[...actions.filter((a) => a.allowed), ...actions.filter((a) => !a.allowed)].map(
                actionItem,
              )}
            </CommandGroup>
          </>
        )}
      </CommandList>
    </CommandDialog>
  );
}

function Entry({ label, place }) {
  return (
    <span className="flex min-w-0 flex-col">
      <span className="truncate">{label}</span>
      {place && <span className="truncate text-xs text-muted-foreground">{place}</span>}
    </span>
  );
}
