import { useEffect, useRef } from 'react';
import { Link, NavLink, useLocation } from 'react-router';

import { cn } from '@/lib/utils';

/**
 * The group's pages as tabs. In the top bar from tablet up (`inBar`), underlined along
 * its bottom edge; on phones under it, as a row of its own. Scrolls sideways when tight.
 */
export function PageTabs({ group, inBar }) {
  const ref = useRef(null);
  const { pathname } = useLocation();
  // Keep the current tab in view when the row scrolls.
  useEffect(() => {
    ref.current
      ?.querySelector('[aria-current="page"]')
      ?.scrollIntoView({ block: 'nearest', inline: 'nearest' });
  }, [pathname]);
  if (!group || group.pages.length < 2) return null;
  return (
    <nav
      ref={ref}
      aria-label={`${group.label} pages`}
      className={cn(
        'flex min-w-0 overflow-x-auto',
        inBar ? 'h-full self-stretch' : '-mx-4 border-b border-border px-4',
      )}
    >
      {group.pages.map((p) => (
        <NavLink
          key={p.id}
          to={p.path}
          end={p.exact}
          className={({ isActive }) =>
            cn(
              'flex shrink-0 items-center border-b-2 border-transparent px-3 text-sm font-medium whitespace-nowrap text-muted-foreground hover:text-foreground',
              inBar ? 'h-full' : '-mb-px h-11',
              isActive && 'border-primary text-foreground',
            )
          }
        >
          {p.label}
        </NavLink>
      ))}
    </nav>
  );
}

/**
 * Above the page: on phones the group's title and page tabs (the top bar has them from
 * tablet up), then the page's part tabs when it has them.
 */
export default function PageHeader({ group, page, part }) {
  if (!group) return null;
  const tabsHere = group.pages.length > 1;
  if (!page?.parts && !tabsHere) {
    return <h1 className="ds sr-only md:hidden">{group.label}</h1>;
  }
  return (
    <div className="ds mb-5 flex flex-col gap-3 md:mb-4">
      <div className="md:hidden">
        <h1 className="font-display text-xl leading-tight font-semibold">{group.label}</h1>
        <div className="mt-2">
          <PageTabs group={group} />
        </div>
      </div>
      {page?.parts && (
        <nav aria-label={`${page.label} parts`} className="flex gap-2 overflow-x-auto">
          {page.parts.map((p) => {
            // The page's own URL shows its first part, so `part` (from the frame) says which.
            const active = p.id === part?.id;
            return (
              <Link
                key={p.id}
                to={`${page.path}/${p.id}`}
                aria-current={active ? 'page' : undefined}
                className={cn(
                  'flex h-9 shrink-0 items-center rounded-full border border-border px-4 text-sm font-medium whitespace-nowrap text-muted-foreground hover:bg-accent hover:text-foreground pointer-coarse:h-11',
                  active &&
                    'border-primary bg-primary text-primary-foreground hover:bg-primary hover:text-primary-foreground',
                )}
              >
                {p.label}
              </Link>
            );
          })}
        </nav>
      )}
    </div>
  );
}
