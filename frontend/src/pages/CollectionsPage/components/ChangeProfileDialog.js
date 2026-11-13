import { useState } from 'react';

import ProfileSelect, { useSortingProfiles } from '@/components/ProfileSelect/ProfileSelect';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Label } from '@/components/ui/label';
import { cn } from '@/lib/utils';

const BIN_CHOICES = [
  {
    value: 'emptied',
    label: 'I emptied the bins',
    hint: 'Every bin counts from zero and the new profile starts a fresh layout.',
  },
  {
    value: 'continue',
    label: 'Keep what’s in the bins and carry on',
    hint: 'Bins holding pieces are left alone; the new profile only gets the unused bins.',
  },
];

/**
 * Changes the profile a collection sorts by. For the current collection (`current`),
 * whose pieces are in the bins, it asks whether the bins were emptied.
 */
export default function ChangeProfileDialog({ open, collection, current, busy, onCancel, onSave }) {
  const { profiles, error } = useSortingProfiles();
  const [profileId, setProfileId] = useState(collection.profile_id ?? null);
  const [bins, setBins] = useState(null);
  const changed = profileId && profileId !== collection.profile_id;
  const ready = changed && (!current || bins);

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onCancel()}>
      <DialogContent>
        <form
          className="flex flex-col gap-4"
          onSubmit={(e) => {
            e.preventDefault();
            if (ready) onSave(profileId, current ? bins : undefined);
          }}
        >
          <DialogHeader>
            <DialogTitle>Sort {collection.name} by…</DialogTitle>
            <DialogDescription>
              {current
                ? 'Its pieces are in the bins now, so say what’s in them.'
                : 'It’s not the collection in the bins, so nothing in the bins changes; its bins start afresh when it’s picked.'}
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-2">
            <Label htmlFor="change-profile">Profile</Label>
            <ProfileSelect
              id="change-profile"
              profiles={profiles}
              value={profileId}
              onChange={setProfileId}
            />
            {error && <p className="text-sm text-destructive">{error}</p>}
          </div>
          {current && (
            <fieldset className="flex flex-col gap-2">
              <legend className="mb-2 text-sm font-medium">The bins</legend>
              {BIN_CHOICES.map((c) => (
                <label
                  key={c.value}
                  className={cn(
                    'flex cursor-pointer gap-3 rounded-lg border p-3 text-sm',
                    bins === c.value ? 'border-primary bg-accent' : 'border-border',
                  )}
                >
                  <input
                    type="radio"
                    name="bins"
                    value={c.value}
                    checked={bins === c.value}
                    onChange={() => setBins(c.value)}
                    className="mt-0.5 accent-primary"
                  />
                  <span>
                    <span className="font-medium">{c.label}</span>
                    <span className="block text-muted-foreground">{c.hint}</span>
                  </span>
                </label>
              ))}
            </fieldset>
          )}
          <DialogFooter>
            <DialogClose asChild>
              <Button type="button" variant="outline">
                Cancel
              </Button>
            </DialogClose>
            <Button type="submit" disabled={busy || !ready}>
              Change profile
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
