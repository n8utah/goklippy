import { Copy, Pencil, Plus, Upload } from 'lucide-react';
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
import NewProfileDialog from './components/NewProfileDialog';
import { useProfilesPageLogic } from './ProfilesPage.logic';

const LEFTOVER_TEXT = {
  misc: 'the rest to misc',
  part_type: 'the rest by part type',
  color: 'the rest by colour',
};

/** What a profile sorts by, in a line. */
function describe(summary) {
  if (summary.format === 'builtin') {
    return summary.builtin === 'color'
      ? 'One category per colour, made as colours arrive'
      : `${summary.categories.length} usual part types, plus any other as it arrives`;
  }
  if (summary.format === 'editor') {
    const rules = summary.rules ?? [];
    return (
      <>
        <span className="font-mono">{rules.length}</span> rule{rules.length === 1 ? '' : 's'}
        {rules.length > 0 && `: ${rules.map((r) => r.name).join(', ')}`} ·{' '}
        {LEFTOVER_TEXT[summary.leftover] ?? LEFTOVER_TEXT.misc}
      </>
    );
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

function ProfileRow({ profile, busy, onDelete, onCopy }) {
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
      <div className="flex shrink-0 flex-wrap gap-1">
        <Button variant="outline" asChild>
          <Link to={`/rules/profiles/${profile.id}`}>
            <Pencil />
            {builtin ? 'See rules' : 'Edit rules'}
          </Link>
        </Button>
        <Button variant="ghost" disabled={busy} onClick={() => onCopy(profile)}>
          <Copy /> Copy
        </Button>
        {!builtin && (
          <Button
            variant="ghost"
            className="text-destructive"
            disabled={busy || used.length > 0}
            title={used.length > 0 ? 'Give its collections another profile first' : undefined}
            onClick={() => onDelete(profile)}
          >
            Delete
          </Button>
        )}
      </div>
    </li>
  );
}

/**
 * Rules › Profiles: the built-in profiles, those made here, and those uploaded from Hive. Each collection
 * picks the one it sorts by (Run › Collections).
 */
const ProfilesPage = () => {
  const logic = useProfilesPageLogic();
  const [deleting, setDeleting] = useState(null);
  const [making, setMaking] = useState(false);
  const copy = (p) => logic.create({ copy_of: p.id });
  const rows = (list, label) => (
    <ul className="flex flex-col gap-3" aria-label={label}>
      {list.map((p) => (
        <ProfileRow key={p.id} profile={p} busy={logic.busy} onDelete={setDeleting} onCopy={copy} />
      ))}
    </ul>
  );

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
          . Make your own from premade rules, copy any profile and change it, or upload a{' '}
          <code className="font-mono">sorting_profile.json</code> built in Hive.
        </p>
        <div className="flex shrink-0 flex-wrap gap-2">
          <Button disabled={logic.busy} onClick={() => setMaking(true)}>
            <Plus /> New profile
          </Button>
          <Button asChild variant="outline" disabled={logic.busy}>
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
        </div>
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
          {rows(logic.builtins, 'Built-in profiles')}
          <h2 className="mt-2 font-display text-lg font-semibold">Made here</h2>
          {logic.made.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              None yet. Make one with New profile, or copy one above.
            </p>
          ) : (
            rows(logic.made, 'Profiles made here')
          )}
          <h2 className="mt-2 font-display text-lg font-semibold">Uploaded from Hive</h2>
          {logic.uploads.length === 0 ? (
            <p className="text-sm text-muted-foreground">None uploaded.</p>
          ) : (
            rows(logic.uploads, 'Uploaded profiles')
          )}
        </>
      )}

      {making && (
        <NewProfileDialog
          open
          starters={logic.starters}
          busy={logic.busy}
          onCancel={() => setMaking(false)}
          onCreate={(body) => logic.create(body)}
        />
      )}

      <Dialog open={Boolean(deleting)} onOpenChange={(o) => !o && setDeleting(null)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle className="font-display">Delete “{deleting?.name}”?</DialogTitle>
            <DialogDescription>
              It can’t be undone. Pieces already sorted keep their records.
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
