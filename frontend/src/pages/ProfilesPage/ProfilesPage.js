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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { cn } from '@/lib/utils';

import { ACTIVATION_MODES, useProfilesPageLogic } from './ProfilesPage.logic';

function ProfileRow({ profile, busy, onActivate, onDelete }) {
  const [mode, setMode] = useState('keep');
  const { summary } = profile;
  const hint = ACTIVATION_MODES.find((m) => m.mode === mode)?.hint;

  return (
    <li
      className={cn(
        'flex flex-col gap-4 rounded-xl border bg-card p-5 lg:flex-row lg:items-start lg:justify-between',
        profile.active ? 'border-primary/60' : 'border-border',
      )}
    >
      <div className="min-w-0">
        <div className="flex flex-wrap items-center gap-2">
          <h2 className="font-display text-lg font-semibold">{profile.name}</h2>
          {profile.active && <Badge variant="running">Active</Badge>}
          {summary.set_based && <Badge variant="outline">Set</Badge>}
        </div>
        <p className="text-sm text-muted-foreground">
          <span className="font-mono">{summary.categories.length}</span> categories ·{' '}
          <span className="font-mono">{summary.part_count.toLocaleString()}</span> parts ·{' '}
          {summary.format === 'program' ? 'compiled program' : 'part map'}
          {summary.fallback_by_color && ' · the rest by color'} · added{' '}
          {new Date(profile.created_at).toLocaleDateString()}
        </p>
        {summary.description && <p className="mt-2 max-w-[75ch] text-sm">{summary.description}</p>}
      </div>
      <div className="flex shrink-0 flex-col gap-2 lg:items-end">
        <div className="flex flex-wrap gap-2">
          <Select value={mode} onValueChange={setMode} disabled={busy}>
            <SelectTrigger className="w-56" aria-label="What happens to the bins">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {ACTIVATION_MODES.map((m) => (
                <SelectItem key={m.mode} value={m.mode}>
                  {m.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Button disabled={busy} onClick={() => onActivate(profile, mode)}>
            {profile.active ? 'Activate again' : 'Activate'}
          </Button>
          <Button
            variant="ghost"
            className="text-destructive"
            disabled={busy || profile.active}
            title={profile.active ? 'Activate another profile first' : undefined}
            onClick={() => onDelete(profile)}
          >
            Delete
          </Button>
        </div>
        {hint && <p className="max-w-sm text-xs text-muted-foreground lg:text-right">{hint}</p>}
      </div>
    </li>
  );
}

/** Rules › Profiles: upload sorting profiles built in Hive and choose the active one. */
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
          ). Upload a <code className="font-mono">sorting_profile.json</code> built in Hive.
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
      ) : logic.profiles.length === 0 ? (
        <p className="text-sm text-muted-foreground">
          No profiles yet. Upload one to start sorting by it.
        </p>
      ) : (
        <ul className="flex flex-col gap-3">
          {logic.profiles.map((p) => (
            <ProfileRow
              key={p.id}
              profile={p}
              busy={logic.busy}
              onActivate={logic.activate}
              onDelete={setDeleting}
            />
          ))}
        </ul>
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
