import { useRef, useState } from 'react';
import { Link, NavLink } from 'react-router';

import { Popover, PopoverAnchor, PopoverContent } from '@/components/ui/popover';
import { NAV_GROUPS } from '@/helpers/navigation';
import { cn } from '@/lib/utils';

const OPEN_DELAY_MS = 120;
const CLOSE_DELAY_MS = 180;

/**
 * The left rail (tablet and up): the app mark, then one icon per group. Clicking an icon
 * goes to the group's first page; hovering it (or → on the keyboard) opens the flyout
 * with all its pages.
 */
export default function Rail({ currentGroup }) {
  return (
    <nav
      aria-label="Main"
      className="ds fixed inset-y-0 left-0 z-30 hidden w-[84px] flex-col items-center gap-1 border-r border-border bg-card py-3 sm:flex"
    >
      <Link
        to="/"
        className="mb-3 flex size-11 items-center justify-center rounded-lg bg-primary font-display text-lg font-bold text-primary-foreground"
        aria-label="goklippy, start page"
      >
        gk
      </Link>
      {NAV_GROUPS.map((group) => (
        <RailGroup key={group.id} group={group} current={group.id === currentGroup?.id} />
      ))}
    </nav>
  );
}

function RailGroup({ group, current }) {
  const [open, setOpen] = useState(false);
  const timer = useRef(null);
  // Hover opens without moving focus; the keyboard opens with focus on the first page.
  const byKeyboard = useRef(false);
  const Icon = group.icon;

  const later = (fn, ms) => {
    clearTimeout(timer.current);
    timer.current = setTimeout(fn, ms);
  };
  const hoverOpen = (e) => {
    if (e.pointerType !== 'mouse') return;
    byKeyboard.current = false;
    later(() => setOpen(true), OPEN_DELAY_MS);
  };
  const hoverClose = (e) => {
    if (e.pointerType !== 'mouse') return;
    later(() => setOpen(false), CLOSE_DELAY_MS);
  };
  const keep = () => clearTimeout(timer.current);

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverAnchor asChild>
        <Link
          to={group.pages[0].path}
          onPointerEnter={hoverOpen}
          onPointerLeave={hoverClose}
          onClick={() => {
            clearTimeout(timer.current);
            setOpen(false);
          }}
          onKeyDown={(e) => {
            if (e.key === 'ArrowRight') {
              e.preventDefault();
              byKeyboard.current = true;
              setOpen(true);
            }
          }}
          aria-current={current ? 'page' : undefined}
          aria-haspopup="menu"
          className={cn(
            'flex w-[72px] flex-col items-center gap-1 rounded-lg py-2 text-xs font-medium text-muted-foreground transition-colors hover:bg-accent hover:text-foreground',
            current && 'bg-accent text-primary-text hover:text-primary-text',
          )}
        >
          <Icon className="size-[22px]" strokeWidth={current ? 2.25 : 1.75} aria-hidden />
          {group.label}
        </Link>
      </PopoverAnchor>
      <PopoverContent
        side="right"
        align="start"
        sideOffset={10}
        className="w-72 p-2"
        onPointerEnter={keep}
        onPointerLeave={hoverClose}
        onOpenAutoFocus={(e) => {
          if (!byKeyboard.current) e.preventDefault();
        }}
      >
        <p className="px-2 pt-1 pb-2 font-display text-sm font-semibold">{group.label}</p>
        <ul role="menu" aria-label={group.label}>
          {group.pages.map((page) => (
            <li key={page.id} role="none">
              <NavLink
                to={page.path}
                role="menuitem"
                onClick={() => setOpen(false)}
                className={({ isActive }) =>
                  cn(
                    'block rounded-md px-2 py-2 hover:bg-accent focus-visible:bg-accent',
                    isActive && 'bg-accent',
                  )
                }
              >
                {({ isActive }) => (
                  <>
                    <span
                      className={cn('block text-sm font-medium', isActive && 'text-primary-text')}
                    >
                      {page.label}
                    </span>
                    <span className="block text-xs text-muted-foreground">{page.about}</span>
                  </>
                )}
              </NavLink>
            </li>
          ))}
        </ul>
      </PopoverContent>
    </Popover>
  );
}
