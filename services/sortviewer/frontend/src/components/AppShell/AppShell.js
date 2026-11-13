import { LogOut, Moon, Sun } from 'lucide-react';
import { NavLink, Outlet } from 'react-router';

import { Button } from '@/components/ui/button';
import { useSession } from '@/helpers/session';
import { setTheme, useTheme } from '@/helpers/theme';
import { cn } from '@/lib/utils';

function BrickMark() {
  return (
    <svg viewBox="0 0 32 32" className="size-7 shrink-0" aria-hidden>
      <rect x="2" y="10" width="28" height="18" rx="3" className="fill-primary" />
      <rect x="7" y="5" width="7" height="6" rx="1.5" className="fill-primary" />
      <rect x="18" y="5" width="7" height="6" rx="1.5" className="fill-primary" />
    </svg>
  );
}

const navClass = ({ isActive }) =>
  cn(
    'rounded-md px-3 py-1.5 text-sm font-medium transition-colors',
    isActive ? 'bg-accent text-foreground' : 'text-muted-foreground hover:text-foreground',
  );

/** The frame around every signed-in page: a slim header, then the page. */
export default function AppShell() {
  const { me, signOut } = useSession();
  const { resolved } = useTheme();
  return (
    <div className="ds min-h-dvh bg-background text-foreground">
      <header className="sticky top-0 z-20 border-b border-border bg-background/90 backdrop-blur">
        <div className="mx-auto flex h-14 max-w-6xl items-center gap-3 px-4">
          <NavLink to="/" className="flex min-w-0 items-center gap-2">
            <BrickMark />
            <span className="truncate font-display text-lg font-semibold">LEGO sort</span>
          </NavLink>
          {me?.admin && (
            <nav className="ml-2 hidden gap-1 sm:flex">
              <NavLink to="/" end className={navClass}>
                Collections
              </NavLink>
              <NavLink to="/admin" className={navClass}>
                Admin
              </NavLink>
            </nav>
          )}
          <div className="ml-auto flex items-center gap-1">
            <span className="hidden max-w-48 truncate text-sm text-muted-foreground md:inline">
              {me?.name || me?.email}
            </span>
            <Button
              variant="ghost"
              size="icon-sm"
              aria-label={resolved === 'dark' ? 'Light theme' : 'Dark theme'}
              onClick={() => setTheme(resolved === 'dark' ? 'light' : 'dark')}
            >
              {resolved === 'dark' ? <Sun /> : <Moon />}
            </Button>
            <Button variant="ghost" size="sm" onClick={signOut}>
              <LogOut /> <span className="hidden sm:inline">Sign out</span>
            </Button>
          </div>
        </div>
        {me?.admin && (
          <nav className="mx-auto flex max-w-6xl gap-1 px-4 pb-2 sm:hidden">
            <NavLink to="/" end className={navClass}>
              Collections
            </NavLink>
            <NavLink to="/admin" className={navClass}>
              Admin
            </NavLink>
          </nav>
        )}
      </header>
      <main className="mx-auto max-w-6xl px-4 py-6 pb-[max(1.5rem,env(safe-area-inset-bottom))]">
        <Outlet />
      </main>
    </div>
  );
}
