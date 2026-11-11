import { Ellipsis } from 'lucide-react';
import { useState } from 'react';
import { Link, NavLink } from 'react-router';

import { Sheet, SheetContent, SheetDescription, SheetTitle } from '@/components/ui/sheet';
import { NAV_GROUPS } from '@/helpers/navigation';
import { cn } from '@/lib/utils';

// The phone shows the first groups in the bar and the rest under More.
const IN_BAR = 3;

const itemClass = (current) =>
  cn(
    'flex h-16 flex-1 flex-col items-center justify-center gap-1 text-[11px] font-medium text-muted-foreground',
    current && 'text-primary-text',
  );

/** Phones only: the groups along the bottom; a group's pages are the tabs at the top. */
export default function BottomBar({ currentGroup }) {
  const [moreOpen, setMoreOpen] = useState(false);
  const bar = NAV_GROUPS.slice(0, IN_BAR);
  const more = NAV_GROUPS.slice(IN_BAR);
  const inMore = more.some((g) => g.id === currentGroup?.id);

  return (
    <nav
      aria-label="Main"
      className="ds fixed inset-x-0 bottom-0 z-30 flex border-t border-border bg-card pb-[env(safe-area-inset-bottom)] sm:hidden"
    >
      {bar.map((group) => {
        const current = group.id === currentGroup?.id;
        return (
          <Link
            key={group.id}
            to={group.pages[0].path}
            className={itemClass(current)}
            aria-current={current ? 'page' : undefined}
          >
            <group.icon className="size-[22px]" strokeWidth={current ? 2.25 : 1.75} aria-hidden />
            {group.label}
          </Link>
        );
      })}
      <button type="button" className={itemClass(inMore)} onClick={() => setMoreOpen(true)}>
        <Ellipsis className="size-[22px]" aria-hidden />
        More
      </button>
      <Sheet open={moreOpen} onOpenChange={setMoreOpen}>
        <SheetContent
          side="bottom"
          className="rounded-t-xl p-4 pb-[calc(1rem+env(safe-area-inset-bottom))]"
        >
          <SheetTitle className="font-display">More</SheetTitle>
          <SheetDescription className="sr-only">The other groups and their pages</SheetDescription>
          {more.map((group) => (
            <div key={group.id} className="mt-3">
              <p className="mb-1 flex items-center gap-2 text-sm font-semibold">
                <group.icon className="size-4 text-muted-foreground" aria-hidden />
                {group.label}
              </p>
              <ul>
                {group.pages.map((page) => (
                  <li key={page.id}>
                    <NavLink
                      to={page.path}
                      onClick={() => setMoreOpen(false)}
                      className={({ isActive }) =>
                        cn(
                          'flex min-h-11 flex-col justify-center rounded-md px-3 py-1.5 hover:bg-accent',
                          isActive && 'bg-accent',
                        )
                      }
                    >
                      <span className="text-sm font-medium">{page.label}</span>
                      <span className="text-xs text-muted-foreground">{page.about}</span>
                    </NavLink>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </SheetContent>
      </Sheet>
    </nav>
  );
}
