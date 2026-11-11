import { Navigate, useNavigate, useParams } from 'react-router';

import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';

import BoardEditorPage from '../BoardEditorPage/BoardEditorPage';
import { useBoardPageLogic } from './BoardPage.logic';
import BoardDiagram from './components/BoardDiagram/BoardDiagram';
import ConfigPart from './components/ConfigPart';
import ConnectionPart from './components/ConnectionPart';
import ConnectionStrip from './components/ConnectionStrip';
import SlotPanel from './components/SlotPanel/SlotPanel';

const PARTS = ['connection', 'wiring', 'config', 'picture'];

/**
 * Machine › Board: Connection, Wiring, Config file and Board picture
 * (/machine/board/<part>), on one config workspace so switching tabs keeps edits.
 *
 * The board diagram, its slot panel and the picture editor are the earlier components,
 * shown in light panels until they're restyled.
 */
const BoardPage = () => {
  const { part = 'connection' } = useParams();
  const navigate = useNavigate();
  const logic = useBoardPageLogic();
  const { config } = logic;

  if (!PARTS.includes(part)) return <Navigate to="/machine/board" replace />;

  return (
    <div className="flex flex-col gap-4">
      <div className="ds flex flex-col gap-4">
        <ConnectionStrip logic={logic} onConnectionTab={part === 'connection'} />
        {config.dirty && part !== 'picture' && (
          <section
            aria-label="Unsaved wiring"
            className="flex flex-col gap-2 rounded-xl border border-warning/50 bg-warning-bg px-4 py-3 sm:flex-row sm:items-center sm:justify-between"
          >
            <p className="text-sm text-warning">
              The wiring has changes not saved to disk. The board uses them from its next connect.
            </p>
            <Button onClick={config.save} disabled={config.loading} className="shrink-0">
              {config.loading ? 'Saving…' : 'Save to disk'}
            </Button>
          </section>
        )}
      </div>

      {part === 'connection' && (
        <div className="ds">
          <ConnectionPart logic={logic} />
        </div>
      )}

      {part === 'wiring' && (
        <WiringPart logic={logic} onOpenConfig={() => navigate('/machine/board/config')} />
      )}

      {part === 'config' && (
        <div className="ds">
          <ConfigPart config={config} />
        </div>
      )}

      {part === 'picture' && (
        <div className="legacy-page">
          <BoardEditorPage />
        </div>
      )}

      <Dialog
        open={Boolean(logic.pendingPort)}
        onOpenChange={(o) => !o && logic.answerPending('cancel')}
      >
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle className="font-display">Save the wiring first?</DialogTitle>
            <DialogDescription>
              The wiring has changes not saved to disk. The board can connect with them either way;
              saving keeps them for next time.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="ghost" onClick={() => logic.answerPending('cancel')}>
              Cancel
            </Button>
            <Button variant="outline" onClick={() => logic.answerPending('keep')}>
              Connect without saving
            </Button>
            <Button onClick={() => logic.answerPending('save')}>Save and connect</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
};

/** Board › Wiring: click a pin on the board to say what's wired there. */
function WiringPart({ logic, onOpenConfig }) {
  const { config, mcu } = logic;
  const hotspot = logic.selectedHotspot;
  const slotId = hotspot?.slot_ids?.[0];

  return (
    <>
      {config.error && (
        <p
          role="alert"
          className="ds rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-2 text-sm text-destructive"
        >
          {config.error}
        </p>
      )}
      <div className="legacy-page grid gap-4 xl:grid-cols-[minmax(0,1fr)_380px]">
        <BoardDiagram
          diagram={config.diagram}
          profileId="ramps14"
          slotStates={config.slotStates}
          selectedHotspot={hotspot}
          onSelectHotspot={logic.selectHotspot}
          readingForSlot={mcu.connected ? mcu.readingForSlot : null}
        />
        <SlotPanel
          hotspot={hotspot}
          slots={config.profile?.slots || {}}
          slotStates={config.slotStates}
          onUpdate={config.updateSlot}
          onClear={config.clearSlot}
          onOpenConfig={onOpenConfig}
          connected={mcu.connected}
          liveObject={slotId ? mcu.objectsBySlot[slotId] : null}
          liveReading={slotId ? mcu.readingForSlot(slotId) : null}
          onSetDigitalOut={mcu.setDigitalOut}
          onSetServo={mcu.setServo}
          onJogStepper={mcu.jogStepper}
          onStopStepper={mcu.stopStepper}
        />
      </div>
    </>
  );
}

export default BoardPage;
