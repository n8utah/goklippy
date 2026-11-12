import { CircleStop, TriangleAlert } from 'lucide-react';
import { Link, Navigate, useNavigate, useParams } from 'react-router';

import { Button } from '@/components/ui/button';
import { FEEDER_IDS, useMachineMotors } from '@/helpers/useMachineMotors';

import ChannelTab from './components/ChannelTab';
import MachineDiagram from './components/MachineDiagram';
import RoutinesTab from './components/RoutinesTab';

const PARTS = ['c1', 'c2', 'c3', 'c4', 'routines'];

/**
 * Machine › Channels: C1–C4 by hand, each on its own tab (/machine/channels/<c1…c4>), and
 * the routines they can run. Speed and direction come from the board's step counter; with
 * no encoders, skipped steps don't show.
 */
const ChannelsPage = () => {
  const { part = 'c1' } = useParams();
  const navigate = useNavigate();
  const logic = useMachineMotors();
  const { channels, connected, settings } = logic;

  if (!PARTS.includes(part)) return <Navigate to="/machine/channels" replace />;
  const feeder = FEEDER_IDS.includes(part);
  const feedersRunning = FEEDER_IDS.some((id) => channels[id].running);

  return (
    <div className="ds flex flex-col gap-4">
      <section className="flex flex-col gap-3 rounded-xl border border-border bg-card px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
        <p className="text-sm">
          {connected ? (
            <>
              Board on <span className="font-mono">{logic.port}</span>
              {logic.wsConnected ? '' : ' · waiting for live motion…'}
            </>
          ) : (
            <>
              No board connected, so the controls are off.{' '}
              <Link
                to="/machine/board/connection"
                className="font-medium text-primary-text hover:underline"
              >
                Connect it
              </Link>
            </>
          )}
        </p>
        <div className="flex shrink-0 gap-2">
          <Button
            variant="outline"
            disabled={!connected || logic.resetting}
            onClick={logic.handleReset}
          >
            {logic.resetting ? 'Restarting…' : 'Reset board'}
          </Button>
          <Button variant="destructive" disabled={!connected} onClick={logic.stopAll}>
            <CircleStop /> Stop all motors
          </Button>
        </div>
      </section>

      {logic.shutdownReason && (
        <p
          role="alert"
          className="flex gap-2 rounded-xl border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm"
        >
          <TriangleAlert className="size-4 shrink-0 text-destructive" aria-hidden />
          <span>
            The board shut down: <strong>{logic.shutdownReason}</strong>. Reset it, or start a
            channel to clear it.
          </span>
        </p>
      )}
      {logic.error && <p className="text-sm text-destructive">{logic.error}</p>}
      {logic.syncError && <p className="text-sm text-warning">{logic.syncError}</p>}

      <div className="flex justify-center rounded-xl border border-border bg-card px-2 py-3">
        <MachineDiagram
          channels={channels}
          indexDegrees={settings.indexDegrees}
          current={part}
          onPick={(id) => navigate(`/machine/channels/${id}`)}
        />
      </div>

      {feeder && (
        <section className="flex flex-col gap-2 rounded-xl border border-border bg-card px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
          <p className="text-sm">
            <span className="font-medium">Feeders together:</span>{' '}
            <span className="font-mono">
              {FEEDER_IDS.map((id) => `${id.toUpperCase()} ${settings.feederRpm[id]}`).join(' · ')}{' '}
              rpm
            </span>
            {' · '}
            {logic.speedsOrdered ? (
              <span className="text-running-text">each faster than the one before</span>
            ) : (
              <span className="text-warning">should rise C1 → C2 → C3 so pieces spread out</span>
            )}
          </p>
          {feedersRunning ? (
            <Button onClick={logic.stopFeeders} className="shrink-0">
              Stop feeders
            </Button>
          ) : (
            <Button disabled={!connected} onClick={logic.startFeeders} className="shrink-0">
              Run C1–C3
            </Button>
          )}
        </section>
      )}

      {part === 'routines' ? (
        <RoutinesTab routines={logic.routines} onChange={logic.setRoutines} />
      ) : (
        <ChannelTab key={part} id={part} logic={logic} />
      )}
    </div>
  );
};

export default ChannelsPage;
