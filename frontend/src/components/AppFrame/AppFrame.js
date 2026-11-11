import { Outlet } from 'react-router';

import { Toaster } from '@/components/ui/sonner';
import { TooltipProvider } from '@/components/ui/tooltip';

import { useAppFrameLogic } from './AppFrame.logic';
import BottomBar from './components/BottomBar';
import PageHeader from './components/PageHeader';
import PageSearch from './components/PageSearch';
import Rail from './components/Rail';
import TopBar from './components/TopBar';

/**
 * The app around every page (docs/ui-ux-spec.md, "The frame"): the rail with its flyouts
 * from tablet up, the bottom bar on phones, the top bar with search, status and Stop, and
 * the page's header and tabs.
 *
 * The frame's own parts carry `ds` (Tailwind's reset); the page area doesn't, so pages not
 * yet rebuilt keep their CSS modules untouched.
 */
export default function AppFrame() {
  const logic = useAppFrameLogic();

  return (
    <TooltipProvider>
      <div className="min-h-dvh bg-background font-sans text-foreground sm:pl-[84px]">
        <Rail currentGroup={logic.group} />
        <TopBar logic={logic} />
        {/* box-border: main is outside `ds`, so it doesn't get the reset's border-box. */}
        <main className="mx-auto box-border w-full max-w-[1440px] px-4 pt-6 pb-28 sm:px-6 sm:pb-10 lg:px-8">
          <PageHeader group={logic.group} page={logic.page} part={logic.part} />
          <Outlet />
        </main>
        <BottomBar currentGroup={logic.group} />
        <PageSearch open={logic.searchOpen} onOpenChange={logic.setSearchOpen} />
        <Toaster position={logic.isPhone ? 'top-center' : 'bottom-right'} />
      </div>
    </TooltipProvider>
  );
}
