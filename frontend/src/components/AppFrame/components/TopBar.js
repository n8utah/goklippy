import { CircleStop, Loader2, Plug, Search } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Sheet, SheetContent, SheetDescription, SheetTitle } from '@/components/ui/sheet';

import { PageTabs } from './PageHeader';
import { StatusDetails, StatusPill } from './StatusPanel';

/**
 * On every page: the group's title and its page tabs (from tablet up; phones get them
 * under the bar), search, the status pill (opening status and controls) and Stop.
 */
export default function TopBar({ logic }) {
  const { status, progress, isDesktop, statusOpen, setStatusOpen, group } = logic;
  const close = () => setStatusOpen(false);

  return (
    <header className="ds sticky top-0 z-20 flex h-16 items-center gap-3 border-b border-border bg-background/90 px-4 backdrop-blur sm:px-6">
      {group && (
        <div className="hidden h-full min-w-0 flex-1 items-center gap-4 md:flex">
          {/* Shown from laptop up; on a tablet the rail says which group, and the tabs need the room. */}
          <h1 className="sr-only shrink-0 font-display text-lg leading-tight font-semibold lg:not-sr-only">
            {group.label}
          </h1>
          <PageTabs group={group} inBar />
        </div>
      )}

      <button
        type="button"
        onClick={() => logic.setSearchOpen(true)}
        className="hidden h-10 w-48 shrink-0 items-center gap-2 rounded-md border border-input bg-card px-3 text-sm text-muted-foreground hover:border-border-strong lg:flex xl:w-64"
      >
        <Search className="size-4 shrink-0" aria-hidden />
        <span className="flex-1 truncate text-left">Search…</span>
        <kbd className="rounded border border-border px-1.5 font-mono text-[11px]">Ctrl K</kbd>
      </button>
      <Button
        variant="ghost"
        size="icon"
        className="shrink-0 lg:hidden"
        onClick={() => logic.setSearchOpen(true)}
        aria-label="Search"
      >
        <Search />
      </Button>

      <div className="ml-auto flex shrink-0 items-center gap-2">
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

        {(logic.canConnect || logic.connecting) && (
          <Button
            variant="outline"
            onClick={logic.connectBoard}
            disabled={logic.connecting}
            title="Connect the board on the port it was last connected on"
            className="shrink-0"
          >
            {logic.connecting ? (
              <Loader2 className="animate-spin" aria-hidden />
            ) : (
              <Plug aria-hidden />
            )}
            <span className="max-sm:sr-only">{logic.connecting ? 'Connecting…' : 'Connect'}</span>
          </Button>
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
