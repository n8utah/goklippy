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
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { cn } from '@/lib/utils';

const BLANK = '';

/** Makes a profile in the editor: blank, or from a starter to build on. */
export default function NewProfileDialog({ open, starters, busy, onCancel, onCreate }) {
  const [starter, setStarter] = useState(starters[0]?.key ?? BLANK);
  const [name, setName] = useState('');
  const choices = [
    ...starters.map((s) => ({ value: s.key, label: s.name, hint: s.description })),
    { value: BLANK, label: 'Blank', hint: 'No rules yet; add premade ones or your own.' },
  ];
  const ready = starter !== BLANK || name.trim();

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onCancel()}>
      <DialogContent>
        <form
          className="flex flex-col gap-4"
          onSubmit={(e) => {
            e.preventDefault();
            if (ready) onCreate({ name: name.trim(), starter });
          }}
        >
          <DialogHeader>
            <DialogTitle>New profile</DialogTitle>
            <DialogDescription>
              Start from a ready-made one and change it, or from nothing.
            </DialogDescription>
          </DialogHeader>
          <fieldset className="flex flex-col gap-2">
            {choices.map((c) => (
              <label
                key={c.value || 'blank'}
                className={cn(
                  'flex cursor-pointer gap-3 rounded-lg border p-3 text-sm',
                  starter === c.value ? 'border-primary bg-accent' : 'border-border',
                )}
              >
                <input
                  type="radio"
                  name="starter"
                  checked={starter === c.value}
                  onChange={() => setStarter(c.value)}
                  className="mt-0.5 accent-primary"
                />
                <span>
                  <span className="font-medium">{c.label}</span>
                  <span className="block text-muted-foreground">{c.hint}</span>
                </span>
              </label>
            ))}
          </fieldset>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="new-profile-name">Name</Label>
            <Input
              id="new-profile-name"
              value={name}
              placeholder={choices.find((c) => c.value === starter && c.value)?.label ?? 'Name it'}
              onChange={(e) => setName(e.target.value)}
            />
          </div>
          <DialogFooter>
            <DialogClose asChild>
              <Button type="button" variant="outline">
                Cancel
              </Button>
            </DialogClose>
            <Button type="submit" disabled={busy || !ready}>
              Make it
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
