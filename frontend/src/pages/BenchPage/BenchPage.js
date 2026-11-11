import { Link } from 'react-router';

import { useMCUSession } from '@/context/MCUSessionContext';
import { useKlipperWorkspace } from '@/helpers/useKlipperWorkspace';

import GlobalServosPanel from './components/GlobalServosPanel/GlobalServosPanel';
import GlobalSteppersPanel from './components/GlobalSteppersPanel/GlobalSteppersPanel';

/**
 * Diagnose › Motor bench: every stepper and servo by hand, in groups and with routines,
 * for trying motors and wiring. The panels are the earlier ones, in light panels until
 * they're restyled.
 */
const BenchPage = () => {
  const { connected, session } = useMCUSession();
  // PCA9685 servo boards are added to the board config.
  const config = useKlipperWorkspace();

  return (
    <div className="flex flex-col gap-4">
      <section className="ds flex flex-col gap-1 rounded-xl border border-border bg-card px-5 py-4">
        <p className="text-muted-foreground">
          Run any motor by hand to try it and its wiring. These move the real machine at once; the
          sorter’s own controls are on Run › Sort and Machine › Channels.
        </p>
        <p className="text-sm">
          {connected ? (
            <>
              Board on <span className="font-mono">{session.port}</span>.
            </>
          ) : (
            <>
              No board connected.{' '}
              <Link
                to="/machine/board/connection"
                className="font-medium text-primary-text hover:underline"
              >
                Connect it under Machine › Board
              </Link>{' '}
              to move motors; groups and routines can be edited without it.
            </>
          )}
        </p>
        {config.error && <p className="text-sm text-destructive">{config.error}</p>}
      </section>
      <div className="legacy-page grid items-start gap-4 lg:grid-cols-2">
        <GlobalSteppersPanel />
        <GlobalServosPanel
          pcaBoards={config.pcaBoards}
          configBusy={config.loading}
          onAddPCA9685Board={config.addPCA9685Board}
        />
      </div>
    </div>
  );
};

export default BenchPage;
