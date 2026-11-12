import { ArrowRight, CircleAlert, ListFilter, Play, Search } from 'lucide-react';
import { Link } from 'react-router';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Progress } from '@/components/ui/progress';

import { useHomePageLogic } from './HomePage.logic';

const TASKS = [
  {
    to: '/run/sort',
    icon: Play,
    title: 'Sort',
    text: 'Start a sort and watch the cameras, the numbers and each piece as it lands.',
  },
  {
    to: '/rules/profiles',
    icon: ListFilter,
    title: 'Sorting rules',
    text: 'Choose a profile, and see which bin each kind of piece goes to.',
  },
  {
    to: '/run/records',
    icon: Search,
    title: 'Look into a piece',
    text: 'Find a piece, its photos and what Brickognize made of it.',
  },
];

function heroTitle(state, connected) {
  if (!connected) return 'No backend';
  if (!state) return 'Connecting…';
  switch (state.state) {
    case 'running':
      return `Sorting · run ${state.run_id}`;
    case 'paused':
      return `Paused · run ${state.run_id}`;
    case 'homing':
      return 'Homing…';
    case 'ready':
      return 'Ready to sort';
    case 'error':
      return 'Stopped on an error';
    default:
      return state.hardware?.connected ? 'Standby' : 'Not set up to sort yet';
  }
}

const when = (t) =>
  new Date(t).toLocaleString([], {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });

/** Home: what the machine is doing, what's in the way, and where to go next. */
const HomePage = () => {
  const logic = useHomePageLogic();
  const { state } = logic;
  const s = state?.state;

  return (
    <div className="ds flex flex-col gap-6">
      {/* Status hero. */}
      <section className="rounded-xl border border-border bg-card p-6">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
          <div className="min-w-0">
            <h2 className="font-display text-[34px] leading-tight font-semibold">
              {heroTitle(state, logic.connected)}
            </h2>
            <p className="mt-1 text-muted-foreground">
              {logic.profile
                ? `Sorting by ${logic.profile.name}.`
                : logic.profile === null
                  ? 'No sorting profile is active.'
                  : ''}
            </p>
          </div>
          <div className="flex shrink-0 flex-wrap gap-2">
            {s === 'running' && (
              <>
                <Button
                  variant="outline"
                  disabled={!logic.canPause}
                  onClick={() => logic.command('pause')}
                >
                  Pause
                </Button>
                <Button asChild>
                  <Link to="/run/sort">
                    Watch it <ArrowRight />
                  </Link>
                </Button>
              </>
            )}
            {s === 'paused' && (
              <Button disabled={!logic.canResume} onClick={() => logic.command('resume')}>
                Resume
              </Button>
            )}
            {s === 'ready' && (
              <Button disabled={!logic.canStart} onClick={() => logic.command('start')}>
                Start sorting
              </Button>
            )}
            {(s === 'standby' || s === 'error') && state?.hardware?.connected && (
              <Button disabled={!logic.canHome} onClick={() => logic.command('home')}>
                Home the machine
              </Button>
            )}
          </div>
        </div>

        {logic.blockers.length > 0 && (
          <div className="mt-5 border-t border-border pt-4">
            <p className="mb-2 text-sm font-medium">
              {s === 'running' ? 'Worth a look' : 'Before sorting'}
            </p>
            <ul className="flex flex-col gap-2">
              {logic.blockers.map((b) => (
                <li key={b.id} className="flex items-start gap-2 text-sm">
                  <CircleAlert className="mt-0.5 size-4 shrink-0 text-warning" aria-hidden />
                  <span>
                    {b.text}{' '}
                    {b.to && (
                      <Link to={b.to} className="font-medium text-primary-text hover:underline">
                        {b.link}
                      </Link>
                    )}
                  </span>
                </li>
              ))}
            </ul>
          </div>
        )}
      </section>

      {/* Setup progress, until it's all done. */}
      {logic.setup.next && (
        <Link
          to="/machine"
          className="group flex flex-col gap-3 rounded-xl border border-border bg-card p-5 transition-colors hover:border-border-strong sm:flex-row sm:items-center sm:gap-6"
        >
          <div className="min-w-0 flex-1">
            <div className="flex items-baseline justify-between gap-3">
              <p className="font-display font-semibold">Setting up the machine</p>
              <span className="font-mono text-sm text-muted-foreground">
                {logic.setup.done} of {logic.setup.total}
              </span>
            </div>
            <Progress
              value={(logic.setup.done / logic.setup.total) * 100}
              className="mt-2"
              aria-label={`${logic.setup.done} of ${logic.setup.total} set up`}
            />
          </div>
          <span className="flex shrink-0 items-center gap-1 text-sm">
            Next: <span className="font-medium">{logic.setup.next.label}</span>
            <ArrowRight
              className="size-4 transition-transform group-hover:translate-x-0.5"
              aria-hidden
            />
          </span>
        </Link>
      )}

      {/* Task cards. */}
      <section aria-label="What to do" className="grid gap-4 md:grid-cols-3">
        {TASKS.map((t) => (
          <Link
            key={t.to}
            to={t.to}
            className="group flex flex-col gap-2 rounded-xl border border-border bg-card p-5 transition-colors hover:border-border-strong"
          >
            <t.icon className="size-6 text-primary-text" aria-hidden />
            <span className="flex items-center gap-1 font-display text-lg font-semibold">
              {t.title}
              <ArrowRight
                className="size-4 transition-transform group-hover:translate-x-0.5"
                aria-hidden
              />
            </span>
            <span className="text-sm text-muted-foreground">{t.text}</span>
          </Link>
        ))}
      </section>

      {/* Recent problems. */}
      <section aria-labelledby="home-problems">
        <div className="mb-2 flex items-baseline justify-between gap-2">
          <h3 id="home-problems" className="font-display text-lg font-semibold">
            Recent problems
          </h3>
          <Link to="/run/records" className="text-sm font-medium text-primary-text hover:underline">
            All records
          </Link>
        </div>
        {logic.incidents.length === 0 ? (
          <p className="text-sm text-muted-foreground">None. Problems during a sort show here.</p>
        ) : (
          <ul className="divide-y divide-border rounded-xl border border-border bg-card">
            {logic.incidents.map((inc) => (
              <li
                key={inc.id}
                className="flex flex-col gap-1 px-4 py-3 sm:flex-row sm:items-start sm:gap-4"
              >
                <span className="w-32 shrink-0 font-mono text-xs text-muted-foreground">
                  {when(inc.opened_at)}
                </span>
                <span className="min-w-0 flex-1 text-sm">
                  <span className="font-medium">{inc.title}</span>
                  <span className="block text-muted-foreground">{inc.message}</span>
                </span>
                <Badge variant={inc.resolved_at ? 'idle' : 'warning'} className="self-start">
                  {inc.resolved_at ? inc.resolution || 'Resolved' : 'Open'}
                </Badge>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
};

export default HomePage;
