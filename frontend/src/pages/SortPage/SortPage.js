import { TriangleAlert } from 'lucide-react';
import { Link } from 'react-router';

import { useJourneyModal } from '@/components/JourneyModal/JourneyModal.logic';
import PieceCard from '@/components/PieceCard/PieceCard';
import { Button } from '@/components/ui/button';

import { CAMERA_CHANNELS, useSortPageLogic } from './SortPage.logic';
import CameraFullscreen from './components/CameraFullscreen';
import CameraTile from './components/CameraTile';
import ChannelControls from './components/ChannelControls';
import ChannelsNow from './components/ChannelsNow';
import TimeSpent from './components/TimeSpent';

/** What the machine is doing, in a sentence, for the top of the page. */
function headline(state, connected, numbers) {
  if (!connected) return { title: 'No backend', text: 'Waiting for goklippy’s backend to answer.' };
  if (!state) return { title: 'Connecting…', text: '' };
  const profile = numbers.profile
    ? `Profile: ${numbers.profile.name}.`
    : 'No sorting profile is active.';
  switch (state.state) {
    case 'running':
      return { title: `Sorting · run ${state.run_id}`, text: profile };
    case 'paused':
      return {
        title: `Paused · run ${state.run_id}`,
        text: 'Resume to carry on where it stopped.',
      };
    case 'homing':
      return { title: 'Homing…', text: 'Starting the hardware and homing the chute.' };
    case 'ready':
      return { title: 'Ready to sort', text: profile };
    case 'error':
      return { title: 'Stopped on an error', text: state.error };
    default:
      return state.hardware?.connected
        ? { title: 'Standby', text: 'Home to start the hardware and home the chute.' }
        : { title: 'No board', text: state.hardware?.message || 'Connect the board first.' };
  }
}

function Stat({ label, value, detail }) {
  return (
    <div className="rounded-xl border border-border bg-card px-4 py-3">
      <p className="text-sm text-muted-foreground">{label}</p>
      <p className="font-mono text-2xl font-medium">{value}</p>
      {detail && <p className="truncate text-xs text-muted-foreground">{detail}</p>}
    </div>
  );
}

/**
 * Run › Sort: start and watch a sort. Cameras over C2–C4, the run's numbers, what's on C4,
 * the pieces sorted last (each opens its journey), and turning channels by hand.
 */
const SortPage = () => {
  const logic = useSortPageLogic();
  const { state, numbers } = logic;
  const journey = useJourneyModal();
  const head = headline(state, logic.connected, numbers);
  const aboard = state?.aboard ?? [];
  const recent = state?.recent ?? [];
  const run = numbers.run;

  return (
    <div className="ds flex flex-col gap-6">
      {/* What it's doing, and the one thing to do next. */}
      <section className="flex flex-col gap-4 rounded-xl border border-border bg-card p-5 sm:flex-row sm:items-center sm:justify-between">
        <div className="min-w-0">
          <h2 className="font-display text-2xl font-semibold">{head.title}</h2>
          {head.text && <p className="text-muted-foreground">{head.text}</p>}
          {state?.state === 'standby' && !state?.hardware?.connected && (
            <Link
              to="/machine/board"
              className="text-sm font-medium text-primary-text hover:underline"
            >
              Connect the board
            </Link>
          )}
        </div>
        <div className="flex shrink-0 flex-wrap gap-2">
          <Button variant="outline" disabled={!logic.canHome} onClick={() => logic.command('home')}>
            {state?.chute?.homed ? 'Re-home' : 'Home'}
          </Button>
          {logic.canPause ? (
            <Button variant="outline" onClick={() => logic.command('pause')}>
              Pause
            </Button>
          ) : null}
          {state?.state === 'paused' ? (
            <Button disabled={!logic.canResume} onClick={() => logic.command('resume')}>
              Resume
            </Button>
          ) : state?.state !== 'running' ? (
            <Button disabled={!logic.canStart} onClick={() => logic.command('start')}>
              Start sorting
            </Button>
          ) : null}
        </div>
      </section>

      {state?.incident && (
        <section
          role="alert"
          className="flex flex-col gap-3 rounded-xl border border-warning/50 bg-warning-bg p-4 sm:flex-row sm:items-center sm:justify-between"
        >
          <div className="flex gap-3">
            <TriangleAlert className="mt-0.5 size-5 shrink-0 text-warning" aria-hidden />
            <div>
              <p className="font-medium text-warning">
                {state.incident.id ? (
                  <Link
                    to={`/run/incidents?incident=${state.incident.id}`}
                    className="font-mono hover:underline"
                    title="Its pictures and every channel's state, under Run › Incidents"
                  >
                    #{state.incident.id}
                  </Link>
                ) : null}{' '}
                {state.incident.title}
              </p>
              <p className="text-sm">{state.incident.message}</p>
            </div>
          </div>
          <div className="flex shrink-0 flex-wrap gap-2">
            {state.incident.actions.map((action) => (
              <Button
                key={action}
                variant={action === 'done' ? 'default' : 'outline'}
                disabled={logic.busy}
                onClick={() => logic.resolve(action)}
                className="capitalize"
              >
                {action === 'done' ? 'Done' : action}
              </Button>
            ))}
          </div>
        </section>
      )}

      {/* The run's numbers. */}
      <section aria-label="This run" className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <Stat
          label={numbers.current ? 'Pieces this run' : 'Pieces last run'}
          value={run ? run.pieces.toLocaleString() : '—'}
          detail={run ? `Run ${run.id} · ${run.profile_name || 'no profile'}` : 'No runs yet'}
        />
        <Stat
          label="Sorted"
          value={run ? run.distributed.toLocaleString() : '—'}
          detail="Into bins"
        />
        <Stat
          label="Per minute"
          value={run ? numbers.perMinute.toFixed(1) : '—'}
          detail={numbers.current ? 'Since the run started' : 'Over the last run'}
        />
        <Stat
          label="Bins full"
          value={numbers.binsTotal ? `${numbers.binsFull} / ${numbers.binsTotal}` : '—'}
          detail={
            numbers.binsFull > 0 ? (
              <Link to="/rules/bins" className="text-primary-text hover:underline">
                Empty them under Rules › Bins
              </Link>
            ) : (
              'Bins in use'
            )
          }
        />
      </section>

      <ChannelsNow doing={state?.doing} />

      {/* Cameras. */}
      <section aria-label="Cameras" className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        {CAMERA_CHANNELS.map((ch) => (
          <CameraTile
            key={ch.id}
            channel={ch}
            summary={state?.channels?.[ch.id]}
            camera={logic.cameras[ch.id]}
            onForgetIgnored={() => logic.forgetIgnored(ch.id)}
            onOpen={() => logic.openFullscreen(ch.id)}
            simulated={state?.hardware?.simulated}
          />
        ))}
      </section>
      <CameraFullscreen channel={logic.fullscreen} onClose={logic.closeFullscreen} />

      <TimeSpent
        timeMs={state?.time_ms}
        current={state?.state === 'running' || state?.state === 'paused'}
      />

      {/* On C4 now. */}
      <section aria-labelledby="sort-aboard">
        <h3 id="sort-aboard" className="mb-2 font-display text-lg font-semibold">
          On C4 now
        </h3>
        {aboard.length === 0 ? (
          <p className="text-sm text-muted-foreground">Nothing aboard.</p>
        ) : (
          <div className="grid gap-2 md:grid-cols-2 xl:grid-cols-3">
            {aboard.map((p) => (
              <PieceCard key={p.id} piece={p} onOpen={() => journey.open(p.id)} />
            ))}
          </div>
        )}
      </section>

      {/* Recently sorted: newest first, each opens its journey. */}
      <section aria-labelledby="sort-recent">
        <div className="mb-2 flex items-baseline justify-between gap-2">
          <h3 id="sort-recent" className="font-display text-lg font-semibold">
            Recently sorted
          </h3>
          <Link to="/pieces/all" className="text-sm font-medium text-primary-text hover:underline">
            All pieces
          </Link>
        </div>
        {recent.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            No pieces yet. Each one shows here as it drops into its bin.
          </p>
        ) : (
          <div className="-mx-4 flex gap-3 overflow-x-auto px-4 pb-2 sm:mx-0 sm:px-0">
            {recent.map((p) => (
              <PieceCard key={p.id} piece={p} layout="tile" onOpen={() => journey.open(p.id)} />
            ))}
          </div>
        )}
      </section>

      <ChannelControls
        shakes={logic.shakes}
        disabled={!logic.handsFree || logic.busy}
        why={logic.handsBlockedWhy}
        sorting={logic.sorting}
        onTurn={logic.turn}
        onShake={logic.shake}
        clearSeconds={logic.clearSeconds}
        clearRpm={logic.clearRpm}
        chuteLeftOut={logic.chuteLeftOut}
        onClear={logic.clear}
      />
    </div>
  );
};

export default SortPage;
