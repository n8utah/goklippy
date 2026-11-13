import { Upload } from 'lucide-react';
import { useState } from 'react';
import { Link } from 'react-router';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { useProfilesPageLogic } from './ProfilesPage.logic';

/** What a profile sorts by, in a line. */
function describe(summary) {
  if (summary.format === 'builtin') {
    return summary.builtin === 'color'
      ? 'One category per colour, made as colours arrive'
      : `${summary.categories.length} usual part types, plus any other as it arrives`;
  }
  return (
    <>
      <span className="font-mono">{summary.categories.length}</span> categories ·{' '}
      <span className="font-mono">{summary.part_count.toLocaleString()}</span> parts ·{' '}
      {summary.format === 'program' ? 'compiled program' : 'part map'}
      {summary.fallback_by_color && ' · the rest by color'}
    </>
  );
}

function ProfileRow({ profile, busy, onDelete }) {
  const { summary } = profile;
  const builtin = profile.source === 'builtin';
  const used = profile.used_by ?? [];

  return (
    <li className="flex flex-col gap-4 rounded-xl border border-border bg-card p-5 lg:flex-row lg:items-start lg:justify-between">
      <div className="min-w-0">
        <div className="flex flex-wrap items-center gap-2">
          <h2 className="font-display text-lg font-semibold">{profile.name}</h2>
          {builtin && <Badge variant="outline">Built in</Badge>}
          {summary.set_based && <Badge variant="outline">Set</Badge>}
        </div>
        <p className="text-sm text-muted-foreground">
          {describe(summary)}
          {!builtin && <> · added {new Date(profile.created_at).toLocaleDateString()}</>}
        </p>
        {summary.description && <p className="mt-2 max-w-[75ch] text-sm">{summary.description}</p>}
        <p className="mt-2 text-sm text-muted-foreground">
          {used.length === 0 ? (
            'No collection sorts by it.'
          ) : (
            <>
              Sorting{' '}
              {used.map((c, i) => (
                <span key={c.id}>
                  {i > 0 && ', '}
                  <Link
                    to={`/run/collections/${c.id}`}
                    className="font-medium text-primary-text hover:underline"
                  >
                    {c.name}
                  </Link>
                </span>
              ))}
            </>
          )}
        </p>
      </div>
      {!builtin && (
        <Button
          variant="ghost"
          className="shrink-0 text-destructive"
          disabled={busy || used.length > 0}
          title={used.length > 0 ? 'Give its collections another profile first' : undefined}
          onClick={() => onDelete(profile)}
        >
          Delete
        </Button>
      )}
    </li>
  );
}

/**
 * Rules › Profiles: the built-in profiles and those uploaded from Hive. Each collection
 * picks the one it sorts by (Run › Collections).
 */
const ProfilesPage = () => {
  const logic = useProfilesPageLogic();
  const [deleting, setDeleting] = useState(null);

  return (
    <div className="ds flex flex-col gap-4">
      <section className="flex flex-col gap-3 rounded-xl border border-border bg-card px-5 py-4 sm:flex-row sm:items-center sm:justify-between">
        <p className="max-w-[75ch] text-sm text-muted-foreground">
          A profile says which category each part goes in; the machine picks the bin for each
          category (see{' '}
          <Link to="/rules/bins" className="font-medium text-primary-text hover:underline">
            Rules › Bins
          </Link>
          ). Each collection picks its profile on{' '}
          <Link to="/run/collections" className="font-medium text-primary-text hover:underline">
            Run › Collections
          </Link>
          . The built-in ones need nothing; for finer rules, upload a{' '}
          <code className="font-mono">sorting_profile.json</code> built in Hive.
        </p>
        <Button asChild disabled={logic.busy} className="shrink-0">
          <label className="cursor-pointer">
            <Upload />
            {logic.busy ? 'Working…' : 'Upload a profile'}
            <input
              type="file"
              accept=".json,application/json"
              className="sr-only"
              disabled={logic.busy}
              onChange={(e) => {
                const file = e.target.files?.[0];
                e.target.value = '';
                if (file) logic.upload(file);
              }}
            />
          </label>
        </Button>
      </section>

      {logic.error && <p className="text-sm text-destructive">{logic.error}</p>}
      {logic.notice && (
        <p className="rounded-xl border border-running/40 bg-running-bg px-4 py-3 text-sm text-running-text">
          {logic.notice}
        </p>
      )}

      {logic.loading ? (
        <p className="text-sm text-muted-foreground">Loading…</p>
      ) : (
        <>
          <ul className="flex flex-col gap-3" aria-label="Built-in profiles">
            {logic.builtins.map((p) => (
              <ProfileRow key={p.id} profile={p} busy={logic.busy} onDelete={setDeleting} />
            ))}
          </ul>
          <h2 className="mt-2 font-display text-lg font-semibold">Uploaded from Hive</h2>
          {logic.uploads.length === 0 ? (
            <p className="text-sm text-muted-foreground">None uploaded.</p>
          ) : (
            <ul className="flex flex-col gap-3" aria-label="Uploaded profiles">
              {logic.uploads.map((p) => (
                <ProfileRow key={p.id} profile={p} busy={logic.busy} onDelete={setDeleting} />
              ))}
            </ul>
          )}
        </>
      )}

      <Dialog open={Boolean(deleting)} onOpenChange={(o) => !o && setDeleting(null)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle className="font-display">Delete “{deleting?.name}”?</DialogTitle>
            <DialogDescription>
              It can’t be undone; upload the file again to get it back. Pieces already sorted keep
              their records.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleting(null)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={() => {
                logic.remove(deleting);
                setDeleting(null);
              }}
            >
              Delete
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
};

export default ProfilesPage;
