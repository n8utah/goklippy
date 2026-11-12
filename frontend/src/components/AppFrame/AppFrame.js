import { useMemo, useState } from 'react';
import { Outlet } from 'react-router';

import JourneyModal from '@/components/JourneyModal/JourneyModal';
import { JourneyModalContext } from '@/components/JourneyModal/JourneyModal.logic';
import SettingQuickEditor from '@/components/SettingQuickEditor/SettingQuickEditor';
import { QuickEditorContext } from '@/components/SettingQuickEditor/SettingQuickEditor.logic';
import { Toaster } from '@/components/ui/sonner';
import { TooltipProvider } from '@/components/ui/tooltip';

import { useAppFrameLogic } from './AppFrame.logic';
import BottomBar from './components/BottomBar';
import PageHeader from './components/PageHeader';
import Rail from './components/Rail';
import SearchPalette from './components/SearchPalette';
import TopBar from './components/TopBar';

/**
 * The app around every page (docs/ui-ux-spec.md, "The frame"): the rail with its flyouts
 * from tablet up, the bottom bar on phones, the top bar with search, status and Stop, the
 * page's header and tabs, and the setting quick editor any page can open.
 *
 * The frame's parts and each page's root carry `ds` (Tailwind's scoped reset).
 */
export default function AppFrame() {
  const logic = useAppFrameLogic();
  const { openSetting } = logic;
  const quickEditor = useMemo(() => ({ open: openSetting }), [openSetting]);
  // The piece whose journey is open, or null.
  const [journeyPiece, setJourneyPiece] = useState(null);
  const journeyModal = useMemo(() => ({ open: setJourneyPiece }), []);

  return (
    <QuickEditorContext.Provider value={quickEditor}>
      <JourneyModalContext.Provider value={journeyModal}>
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
            <SearchPalette
              open={logic.searchOpen}
              onOpenChange={logic.setSearchOpen}
              actions={logic.actions}
              onOpenSetting={openSetting}
            />
            <SettingQuickEditor settingKey={logic.editKey} onClose={logic.closeSetting} />
            <JourneyModal pieceId={journeyPiece} onClose={() => setJourneyPiece(null)} />
            <Toaster position={logic.isPhone ? 'top-center' : 'bottom-right'} />
          </div>
        </TooltipProvider>
      </JourneyModalContext.Provider>
    </QuickEditorContext.Provider>
  );
}
