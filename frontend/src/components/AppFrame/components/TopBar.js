import { CircleStop, Search } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Sheet, SheetContent, SheetDescription, SheetTitle } from '@/components/ui/sheet';

import { StatusDetails, StatusPill } from './StatusPanel';

/** Search, the status pill (opening status and controls) and Stop, on every page. */
export default function TopBar({ logic }) {
  const { status, progress, isDesktop, statusOpen, setStatusOpen } = logic;
  const close = () => setStatusOpen(false);

  return (
    <header className="ds sticky top-0 z-20 flex h-16 items-center gap-3 border-b border-border bg-background/90 px-4 backdrop-blur sm:px-6">
      <button
        type="button"
        onClick={() => logic.setSearchOpen(true)}
        className="hidden h-10 w-full max-w-sm items-center gap-2 rounded-md border border-input bg-card px-3 text-sm text-muted-foreground hover:border-border-strong sm:flex"
      >
        <Search className="size-4" aria-hidden />
        <span className="flex-1 text-left">Search pages…</span>
        <kbd className="rounded border border-border px-1.5 font-mono text-[11px]">Ctrl K</kbd>
      </button>
      <Button
        variant="ghost"
        size="icon"
        className="sm:hidden"
        onClick={() => logic.setSearchOpen(true)}
        aria-label="Search"
      >
        <Search />
      </Button>

      <div className="ml-auto flex min-w-0 items-center gap-2">
        {isDesktop ? (
          <Popover open={statusOpen} onOpenChange={setStatusOpen}>
            <PopoverTrigger asChild>
              <StatusPill status={status} progress={progress} />
            </PopoverTrigger>
            <PopoverContent align="end" sideOffset={8} className="w-[380px] p-4">
              <StatusDetails logic={logic} onNavigate={close} />
            </PopoverContent>
          </Popover>
        ) : (
          <>
            <StatusPill
              status={status}
              progress={progress}
              compact
              onClick={() => setStatusOpen(true)}
            />
            <Sheet open={statusOpen} onOpenChange={setStatusOpen}>
              <SheetContent side="top" className="max-h-dvh overflow-y-auto p-5 max-sm:h-dvh">
                <SheetTitle className="sr-only">Machine status</SheetTitle>
                <SheetDescription className="sr-only">
                  The machine’s state, its parts and its controls.
                </SheetDescription>
                <StatusDetails logic={logic} onNavigate={close} />
              </SheetContent>
            </Sheet>
          </>
        )}

        <Button
          variant="destructive"
          onClick={logic.stop}
          disabled={!logic.canStop}
          title={logic.canStop ? 'Stop everything that’s moving' : 'Nothing is moving'}
          className="shrink-0"
        >
          <CircleStop aria-hidden />
          Stop
        </Button>
      </div>
    </header>
  );
}
