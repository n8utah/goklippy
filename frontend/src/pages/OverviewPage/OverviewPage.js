import SetupChecklist from '@/components/SetupChecklist/SetupChecklist';
import { Progress } from '@/components/ui/progress';
import { useSetupChecklist } from '@/helpers/useSetupChecklist';

/**
 * Machine › Overview: what's set up and what isn't, live, in the order to do it, each
 * with a link to the page that does it.
 */
const OverviewPage = () => {
  const { items, done, total, next } = useSetupChecklist();
  return (
    <div className="ds flex flex-col gap-4">
      <section className="flex flex-col gap-3 rounded-xl border border-border bg-card p-5">
        <div className="flex items-baseline justify-between gap-4">
          <h2 className="font-display text-xl font-semibold">
            {done === total ? 'Set up to sort' : 'Setting up the machine'}
          </h2>
          <span className="font-mono text-sm text-muted-foreground">
            {done} of {total}
          </span>
        </div>
        <Progress value={(done / total) * 100} aria-label={`${done} of ${total} set up`} />
        <p className="text-sm text-muted-foreground">
          {next ? (
            <>
              Next: <span className="font-medium text-foreground">{next.label}</span>.
            </>
          ) : (
            'Everything needed to sort is set up.'
          )}{' '}
          Ticks update live; the ones marked with a question mark are checked once the board is
          connected.
        </p>
      </section>
      <section className="rounded-xl border border-border bg-card">
        <SetupChecklist items={items} />
      </section>
    </div>
  );
};

export default OverviewPage;
