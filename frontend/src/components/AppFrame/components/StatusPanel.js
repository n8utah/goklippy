import { Link } from 'react-router';

import { Button } from '@/components/ui/button';
import { Progress } from '@/components/ui/progress';
import { Separator } from '@/components/ui/separator';
import { Switch } from '@/components/ui/switch';
import { cn } from '@/lib/utils';

const PILL_TONES = {
  running: 'bg-running-bg text-running-text border-transparent',
  ready: 'bg-card text-foreground border-border-strong',
  warning: 'bg-warning-bg text-warning border-transparent',
  error: 'bg-destructive/10 text-destructive border-transparent',
  idle: 'bg-muted text-muted-foreground border-border',
};

const DOT_TONES = {
  running: 'bg-running',
  ready: 'bg-running',
  warning: 'bg-warning',
  error: 'bg-destructive',
  idle: 'bg-idle',
};

/** A small ring filling with the running task's progress. */
function ProgressRing({ value }) {
  const r = 7;
  const c = 2 * Math.PI * r;
  return (
    <svg viewBox="0 0 18 18" className="size-4 -rotate-90" aria-hidden>
      <circle
        cx="9"
        cy="9"
        r={r}
        fill="none"
        stroke="currentColor"
        strokeOpacity="0.2"
        strokeWidth="2.5"
      />
      <circle
        cx="9"
        cy="9"
        r={r}
        fill="none"
        stroke="currentColor"
        strokeWidth="2.5"
        strokeDasharray={c}
        strokeDashoffset={c * (1 - value)}
        strokeLinecap="round"
      />
    </svg>
  );
}

/** The top bar's status: state and run, colored by state, opening the full panel. */
export function StatusPill({ status, progress, compact, ref, ...props }) {
  return (
    <button
      ref={ref}
      type="button"
      aria-label={`Machine status: ${status.label}. Open status and controls`}
      className={cn(
        'inline-flex h-10 max-w-[60vw] items-center gap-2 rounded-full border px-3.5 text-sm font-medium whitespace-nowrap transition-colors hover:brightness-95 pointer-coarse:h-11',
        PILL_TONES[status.tone],
      )}
      {...props}
    >
      <span className={cn('size-2 shrink-0 rounded-full', DOT_TONES[status.tone])} aria-hidden />
      <span className={cn('truncate', compact && 'max-sm:max-w-[9rem]')}>{status.label}</span>
      {progress !== null && <ProgressRing value={progress} />}
    </button>
  );
}

function Row({ label, children }) {
  return (
    <div className="flex items-start justify-between gap-4 py-1.5 text-sm">
      <span className="text-muted-foreground">{label}</span>
      <span className="text-right">{children}</span>
    </div>
  );
}

const time = (iso) =>
  iso ? new Date(iso).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) : '';

/** Everything about the machine right now, and its controls. */
export function StatusDetails({ logic, onNavigate }) {
  const { state, status, job, connected } = logic;
  const hw = state?.hardware;
  const supply = state?.supply;
  const lights = state?.lights;
  const chute = state?.chute;

  const problems = [];
  if (!connected) problems.push({ text: 'The backend isn’t answering. Is goklippy running?' });
  if (state?.error) problems.push({ text: state.error });
  if (state?.incident)
    problems.push({
      text: `${state.incident.title}: ${state.incident.message}`,
      to: '/run/sort',
      link: 'Resolve on Run › Sort',
    });
  if (supply?.low) problems.push({ text: supply.message || 'The motors’ supply is low.' });
  if (state?.last_job?.error && !job)
    problems.push({ text: `${state.last_job.name} failed: ${state.last_job.error}` });

  return (
    <div className="flex flex-col gap-3">
      <div>
        <p className="font-display text-lg font-semibold">{status.label}</p>
        {state?.since && (
          <p className="text-sm text-muted-foreground">
            {state.state === 'running' || state.state === 'paused' ? 'Run started' : 'Since'}{' '}
            {time(state.since)}
          </p>
        )}
      </div>

      {problems.length > 0 && (
        <ul className="flex flex-col gap-2" aria-label="Problems">
          {problems.map((p) => (
            <li
              key={p.text}
              className="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm"
            >
              {p.text}
              {p.to && (
                <>
                  {' '}
                  <Link
                    to={p.to}
                    onClick={onNavigate}
                    className="font-medium text-primary-text underline"
                  >
                    {p.link}
                  </Link>
                </>
              )}
            </li>
          ))}
        </ul>
      )}

      {job && (
        <div className="rounded-md border border-border p-3">
          <div className="flex items-center justify-between gap-2">
            <div className="min-w-0">
              <p className="text-sm font-medium capitalize">{job.name}</p>
              {job.detail && <p className="truncate text-xs text-muted-foreground">{job.detail}</p>}
            </div>
            <Button size="sm" variant="outline" onClick={logic.cancelJob} disabled={logic.busy}>
              Cancel
            </Button>
          </div>
          {job.total > 0 && (
            <div className="mt-2 flex items-center gap-2">
              <Progress value={(job.step / job.total) * 100} className="flex-1" />
              <span className="font-mono text-xs text-muted-foreground">
                {job.step}/{job.total}
              </span>
            </div>
          )}
        </div>
      )}

      <div>
        <Row label="Board">
          {hw?.connected ? (
            `${hw.simulated ? 'Simulated' : 'Connected'}${hw.port ? ` · ${hw.port}` : ''}`
          ) : (
            <Link to="/machine/board" onClick={onNavigate} className="text-primary-text underline">
              Not connected
            </Link>
          )}
        </Row>
        <Row label="Motor power">
          {supply?.known ? (
            <span className={cn('font-mono', supply.low && 'text-destructive')}>
              {supply.volts.toFixed(1)} V{supply.low ? ' · low' : ''}
            </span>
          ) : (
            <span className="text-muted-foreground">Not measured</span>
          )}
        </Row>
        <Row label="Chute">
          {chute?.homed
            ? `Homed${chute.angle != null ? ` · at ${chute.angle.toFixed(1)}°` : ''}`
            : 'Not homed'}
        </Row>
        <div className="py-1.5 text-sm">
          <div className="flex items-center justify-between gap-4">
            <label htmlFor="status-light" className="text-muted-foreground">
              Light
            </label>
            <Switch
              id="status-light"
              checked={Boolean(lights?.on)}
              onCheckedChange={logic.toggleLights}
              disabled={logic.busy || Boolean(lights?.message) || !connected}
            />
          </div>
          {/* Why it can't be switched (no board, or no light in the board config). */}
          {lights?.message && (
            <p className="mt-0.5 text-xs text-muted-foreground first-letter:uppercase">
              {lights.message}
            </p>
          )}
        </div>
      </div>

      <Separator />

      <div className="flex flex-wrap gap-2">
        {state?.state === 'paused' ? (
          <Button onClick={() => logic.command('resume')} disabled={!logic.canResume}>
            Resume
          </Button>
        ) : (
          <Button onClick={() => logic.command('start')} disabled={!logic.canStart}>
            Start sorting
          </Button>
        )}
        <Button variant="outline" onClick={() => logic.command('pause')} disabled={!logic.canPause}>
          Pause
        </Button>
        <Button variant="outline" onClick={() => logic.command('home')} disabled={!logic.canHome}>
          Home
        </Button>
      </div>
    </div>
  );
}
