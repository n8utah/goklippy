import { Pencil, Plus, Trash2 } from 'lucide-react';
import { useState } from 'react';

import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
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

/** Adds or edits a viewer: email (new ones only), name, password, and their collections. */
function ViewerDialog({ viewer, collections, busy, onSave, onClose }) {
  const isNew = !viewer.id;
  const [form, setForm] = useState({
    email: viewer.email ?? '',
    name: viewer.name ?? '',
    password: '',
    collections: viewer.collections ?? [],
  });
  const set = (k, v) => setForm((f) => ({ ...f, [k]: v }));
  const toggle = (id) =>
    set(
      'collections',
      form.collections.includes(id)
        ? form.collections.filter((c) => c !== id)
        : [...form.collections, id],
    );
  const submit = async (e) => {
    e.preventDefault();
    if (await onSave({ ...form, id: viewer.id })) onClose();
  };
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <form onSubmit={submit} className="flex flex-col gap-4">
          <DialogHeader>
            <DialogTitle>{isNew ? 'Add a viewer' : viewer.email}</DialogTitle>
            <DialogDescription>
              {isNew
                ? 'They sign in with this email and password. Send them both yourself.'
                : 'Leave the password empty to keep it. A new one signs them out everywhere.'}
            </DialogDescription>
          </DialogHeader>
          {isNew && (
            <div className="flex flex-col gap-2">
              <Label htmlFor="viewer-email">Email</Label>
              <Input
                id="viewer-email"
                type="email"
                required
                value={form.email}
                onChange={(e) => set('email', e.target.value)}
              />
            </div>
          )}
          <div className="flex flex-col gap-2">
            <Label htmlFor="viewer-name">Name</Label>
            <Input
              id="viewer-name"
              value={form.name}
              placeholder="Sam"
              onChange={(e) => set('name', e.target.value)}
            />
          </div>
          <div className="flex flex-col gap-2">
            <Label htmlFor="viewer-password">{isNew ? 'Password' : 'New password'}</Label>
            <Input
              id="viewer-password"
              type="text"
              autoComplete="off"
              required={isNew}
              minLength={8}
              value={form.password}
              placeholder="At least 8 characters"
              onChange={(e) => set('password', e.target.value)}
            />
          </div>
          <fieldset className="flex flex-col gap-2">
            <legend className="mb-1 text-sm font-medium">Can see</legend>
            {collections.length === 0 && (
              <p className="text-sm text-muted-foreground">No collections shared yet.</p>
            )}
            {collections.map((c) => (
              <label key={c.id} className="flex items-center gap-2 text-sm">
                <Checkbox
                  checked={form.collections.includes(c.id)}
                  onCheckedChange={() => toggle(c.id)}
                />
                {c.name}
              </label>
            ))}
          </fieldset>
          <DialogFooter>
            <DialogClose asChild>
              <Button type="button" variant="outline">
                Cancel
              </Button>
            </DialogClose>
            <Button type="submit" disabled={busy}>
              {isNew ? 'Add viewer' : 'Save'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

export default function ViewersPanel({ logic }) {
  const [editing, setEditing] = useState(null);
  const names = new Map(logic.collections.map((c) => [c.id, c.name]));
  return (
    <section className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <p className="text-sm text-muted-foreground">
          People who can watch. Each sees only the collections ticked for them.
        </p>
        <Button onClick={() => setEditing({})}>
          <Plus /> Add a viewer
        </Button>
      </div>
      {logic.viewers?.length === 0 && (
        <p className="rounded-xl border border-border bg-card p-5 text-sm text-muted-foreground">
          No viewers yet.
        </p>
      )}
      <ul className="flex flex-col gap-2">
        {(logic.viewers ?? []).map((v) => (
          <li
            key={v.id}
            className="flex items-center gap-3 rounded-xl border border-border bg-card px-4 py-3"
          >
            <div className="min-w-0 flex-1">
              <p className="truncate font-medium">
                {v.name || v.email}
                {v.name && <span className="font-normal text-muted-foreground"> · {v.email}</span>}
              </p>
              <p className="truncate text-sm text-muted-foreground">
                {v.collections.length
                  ? v.collections.map((id) => names.get(id) ?? `#${id}`).join(', ')
                  : 'Nothing yet'}
              </p>
            </div>
            <Button
              variant="ghost"
              size="icon-sm"
              aria-label={`Edit ${v.email}`}
              onClick={() => setEditing(v)}
            >
              <Pencil />
            </Button>
            <Button
              variant="ghost"
              size="icon-sm"
              aria-label={`Remove ${v.email}`}
              onClick={() => logic.askDeleteViewer(v)}
            >
              <Trash2 />
            </Button>
          </li>
        ))}
      </ul>
      {editing && (
        <ViewerDialog
          viewer={editing}
          collections={logic.collections}
          busy={logic.busy}
          onSave={logic.saveViewer}
          onClose={() => setEditing(null)}
        />
      )}
    </section>
  );
}
