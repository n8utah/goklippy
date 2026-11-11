import { Link } from 'react-router';

import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';

/**
 * The board's connection on every Board tab: where it's connected and live telemetry,
 * with Reset and Disconnect; or that it isn't, with the way to connect.
 */
export default function ConnectionStrip({ logic, onConnectionTab }) {
  const { mcu } = logic;
  const session = mcu.session;
  const tone = mcu.connected ? 'running' : mcu.connecting ? 'warning' : 'idle';

  return (
    <section
      aria-label="Board connection"
      className="flex flex-col gap-3 rounded-xl border border-border bg-card px-4 py-3 sm:flex-row sm:items-center sm:justify-between"
    >
      <div className="flex min-w-0 items-start gap-3">
        <span
          className={cn(
            'mt-1.5 size-2.5 shrink-0 rounded-full',
            tone === 'running' && 'bg-running',
            tone === 'warning' && 'bg-warning',
            tone === 'idle' && 'bg-idle',
          )}
          aria-hidden
        />
        <div className="min-w-0">
          {mcu.connected ? (
            <>
              <p className="font-medium">
                Connected to <span className="font-mono">{session.port}</span>
              </p>
              <p className="text-sm text-muted-foreground">
                {session.objects?.length || 0} objects ·{' '}
                {mcu.wsConnected ? 'live telemetry' : 'waiting for live telemetry…'}
              </p>
            </>
          ) : mcu.connecting ? (
            <>
              <p className="font-medium">Connecting…</p>
              <p className="text-sm text-muted-foreground">
                {mcu.connectStatus ||
                  'Opening the port and asking the board what it is; usually 30–90 seconds.'}
              </p>
            </>
          ) : (
            <>
              <p className="font-medium">Not connected</p>
              <p className="text-sm text-muted-foreground">
                {onConnectionTab ? (
                  'Choose the board’s port below.'
                ) : (
                  <>
                    Wiring can be edited without a board.{' '}
                    <Link
                      to="/machine/board/connection"
                      className="font-medium text-primary-text hover:underline"
                    >
                      Connect it
                    </Link>
                  </>
                )}
              </p>
            </>
          )}
          {mcu.error && !mcu.error.includes('reconnecting') && (
            <p className="mt-1 text-sm text-destructive">{mcu.error}</p>
          )}
        </div>
      </div>
      {mcu.connected && (
        <div className="flex shrink-0 gap-2">
          <Button
            variant="outline"
            disabled={logic.resetting}
            onClick={logic.reset}
            title="Restarts the board and stops every motor. Takes a few seconds."
          >
            {logic.resetting ? 'Restarting…' : 'Reset board'}
          </Button>
          <Button variant="outline" disabled={logic.resetting} onClick={mcu.disconnect}>
            Disconnect
          </Button>
        </div>
      )}
    </section>
  );
}
