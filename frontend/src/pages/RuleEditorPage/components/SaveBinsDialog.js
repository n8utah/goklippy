import { useState } from 'react';

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
import { cn } from '@/lib/utils';

const BIN_CHOICES = [
  {
    value: 'emptied',
    label: 'I emptied the bins',
    hint: 'Every bin counts from zero and the rules start a fresh layout.',
  },
  {
    value: 'continue',
    label: 'Keep what’s in the bins and carry on',
    hint: 'Bins keep their groups where the rules still have them; groups that are gone free their bins.',
  },
];

/** Saving the profile the bins are being sorted by: what's in the bins? */
export default function SaveBinsDialog({ open, busy, onCancel, onSave }) {
  const [bins, setBins] = useState(null);
  return (
    <Dialog open={open} onOpenChange={(o) => !o && onCancel()}>
      <DialogContent>
        <form
          className="flex flex-col gap-4"
          onSubmit={(e) => {
            e.preventDefault();
            if (bins) onSave(bins);
          }}
        >
          <DialogHeader>
            <DialogTitle>The bins sort by this profile</DialogTitle>
            <DialogDescription>
              The collection in the bins sorts by it, so say what’s in them.
            </DialogDescription>
          </DialogHeader>
          <fieldset className="flex flex-col gap-2">
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
          <DialogFooter>
            <DialogClose asChild>
              <Button type="button" variant="outline">
                Cancel
              </Button>
            </DialogClose>
            <Button type="submit" disabled={busy || !bins}>
              Save
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
