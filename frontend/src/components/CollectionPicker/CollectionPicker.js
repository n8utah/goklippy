import { Plus } from 'lucide-react';
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
import ProfileSelect, { useSortingProfiles } from '@/components/ProfileSelect/ProfileSelect';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectSeparator,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';

import { useCollectionPicker } from './CollectionPicker.logic';

const NEW = '__new__';
const NONE = '__none__';

function NewCollectionDialog({ open, hasCurrent, busy, onCancel, onCreate }) {
  const [name, setName] = useState('');
  const { profiles, error } = useSortingProfiles();
  const [picked, setPicked] = useState(null);
  // Sort by part type unless another is picked.
  const profileId = picked ?? profiles?.[0]?.id ?? null;
  const submit = (e) => {
    e.preventDefault();
    if (name.trim() && profileId) onCreate(name.trim(), profileId);
  };
  return (
    <Dialog open={open} onOpenChange={(o) => !o && onCancel()}>
      <DialogContent>
        <form onSubmit={submit} className="flex flex-col gap-4">
          <DialogHeader>
            <DialogTitle>New collection</DialogTitle>
            <DialogDescription>
              Whose LEGO is this? Runs sort into it until you pick another.
              {hasCurrent && ' The bins start empty: they count as emptied for it.'}
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-2">
            <Label htmlFor="new-collection-name">Name</Label>
            <Input
              id="new-collection-name"
              autoFocus
              value={name}
              placeholder="Sam’s LEGO"
              onChange={(e) => setName(e.target.value)}
            />
          </div>
          <div className="flex flex-col gap-2">
            <Label htmlFor="new-collection-profile">Sort it by</Label>
            <ProfileSelect
              id="new-collection-profile"
              profiles={profiles}
              value={profileId}
              onChange={setPicked}
            />
            {error ? (
              <p className="text-sm text-destructive">{error}</p>
            ) : (
              <p className="text-xs text-muted-foreground">
                What groups it comes back in. You can change it later on Run › Collections.
              </p>
            )}
          </div>
          <DialogFooter>
            <DialogClose asChild>
              <Button type="button" variant="outline">
                Cancel
              </Button>
            </DialogClose>
            <Button type="submit" disabled={busy || !name.trim() || !profileId}>
              Create and pick
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

/**
 * The collection runs sort into (`current`, from the sorter's state), with a picker to
 * change it or make a new one. `locked` while a run is on: stop it first.
 */
export default function CollectionPicker({ current, locked, onPicked, id = 'collection' }) {
  const logic = useCollectionPicker(current, onPicked);
  return (
    <>
      <Select
        value={current ? String(current.id) : NONE}
        disabled={locked || logic.busy}
        onValueChange={(v) => (v === NEW ? logic.startCreating() : logic.choose(v))}
      >
        <SelectTrigger
          id={id}
          className="w-56"
          title={locked ? 'Stop the run to pick another collection' : undefined}
        >
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {!current && (
            <SelectItem value={NONE} disabled>
              Pick a collection
            </SelectItem>
          )}
          {logic.collections.map((c) => (
            <SelectItem key={c.id} value={String(c.id)}>
              {c.name}
            </SelectItem>
          ))}
          {logic.collections.length > 0 && <SelectSeparator />}
          <SelectItem value={NEW}>
            <Plus /> New collection…
          </SelectItem>
        </SelectContent>
      </Select>

      <NewCollectionDialog
        key={logic.creating ? 'open' : 'closed'}
        open={logic.creating}
        hasCurrent={Boolean(current)}
        busy={logic.busy}
        onCancel={logic.cancelCreating}
        onCreate={logic.create}
      />

      <Dialog open={Boolean(logic.confirm)} onOpenChange={(o) => !o && logic.cancelPick()}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Sort into {logic.confirm?.name}?</DialogTitle>
            <DialogDescription>
              Switching from {current?.name} means the bins have been emptied. Every bin counts from
              zero and {logic.confirm?.name} starts a fresh bin layout; nothing is added to bins
              left from before.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <DialogClose asChild>
              <Button variant="outline">Keep {current?.name}</Button>
            </DialogClose>
            <Button disabled={logic.busy} onClick={logic.confirmPick}>
              Bins are empty, switch
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
