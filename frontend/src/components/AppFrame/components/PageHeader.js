import { Link, NavLink } from 'react-router';

import { cn } from '@/lib/utils';

/**
 * The group's title and its pages as tabs, then the page's part tabs when it has them.
 * Tabs scroll sideways on narrow screens.
 */
export default function PageHeader({ group, page, part }) {
  if (!group) return null;
  return (
    <div className="ds mb-6">
      <h1 className="font-display text-[28px] leading-tight font-semibold">{group.label}</h1>
      {group.pages.length > 1 && (
        <nav
          aria-label={`${group.label} pages`}
          className="-mx-4 mt-3 flex overflow-x-auto border-b border-border px-4 sm:mx-0 sm:px-0"
        >
          {group.pages.map((p) => (
            <NavLink
              key={p.id}
              to={p.path}
              className={({ isActive }) =>
                cn(
                  '-mb-px flex h-11 shrink-0 items-center border-b-2 border-transparent px-3 text-sm font-medium whitespace-nowrap text-muted-foreground hover:text-foreground',
                  isActive && 'border-primary text-foreground',
                )
              }
            >
              {p.label}
            </NavLink>
          ))}
        </nav>
      )}
      {page?.parts && (
        <nav aria-label={`${page.label} parts`} className="mt-4 flex gap-2 overflow-x-auto">
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
