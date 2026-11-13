/**
 * The sorting plan: each bin is a group the collection comes back in, with what it
 * takes and how many pieces it has so far. `view` is { plan, counts } from the server;
 * the plan is the sorter's: { profile: { name }, bins: [{ layer, bin, categories }] }.
 */
export default function PlanView({ view }) {
  if (!view) return <p className="text-sm text-muted-foreground">Loading…</p>;
  const plan = view.plan ?? {};
  const counts = new Map((view.counts ?? []).map((c) => [`${c.layer}-${c.bin}`, c.pieces]));
  const bins = [...(plan.bins ?? [])].sort((a, b) => a.layer - b.layer || a.bin - b.bin);
  const bucket = counts.get('0-0') ?? 0;

  if (bins.length === 0) {
    return (
      <p className="rounded-xl border border-border bg-card p-5 text-sm text-muted-foreground">
        No plan yet. Each group gets its bin as the first piece of it arrives.
      </p>
    );
  }
  return (
    <div className="flex flex-col gap-3">
      <p className="text-sm text-muted-foreground">
        {plan.profile?.name ? `Sorted by ${plan.profile.name}. ` : ''}
        Each bin below is a group you’ll get back.
      </p>
      <ul className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {bins.map((b) => {
          const names = (b.categories ?? []).map((c) => c.name || c.id);
          return (
            <li
              key={`${b.layer}-${b.bin}`}
              className="flex items-start gap-3 rounded-xl border border-border bg-card p-4"
            >
              <span className="rounded-md bg-muted px-2 py-1 font-mono text-xs whitespace-nowrap">
                Bin {b.layer}-{b.bin}
              </span>
              <span className="min-w-0 flex-1">
                <span className="block font-medium">{names.join(', ') || 'Nothing now'}</span>
              </span>
              <span className="font-mono text-lg font-medium">
                {(counts.get(`${b.layer}-${b.bin}`) ?? 0).toLocaleString()}
              </span>
            </li>
          );
        })}
      </ul>
      {bucket > 0 && (
        <p className="text-sm text-muted-foreground">
          <span className="font-mono text-foreground">{bucket.toLocaleString()}</span> went to the
          bucket: no group for them, or too big for a bin.
        </p>
      )}
    </div>
  );
}
