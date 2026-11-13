import { Copy, KeyRound } from 'lucide-react';
import { useState } from 'react';
import { toast } from 'sonner';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';

const when = (t) => (t ? new Date(t).toLocaleString() : 'never');

/** The keys the sorter sends updates with. A new key shows once. */
export default function KeysPanel({ logic }) {
  const [name, setName] = useState('');
  const copy = () =>
    navigator.clipboard.writeText(logic.newKey).then(
      () => toast.success('Copied'),
      () => toast.error('Couldn’t copy; select it and copy by hand'),
    );
  return (
    <section className="flex flex-col gap-3">
      <p className="text-sm text-muted-foreground">
        The sorter sends updates with a key. Put this site’s address ({window.location.origin}) and
        a key into the sorter’s sharing settings.
      </p>
      <form
        className="flex flex-wrap gap-2"
        onSubmit={async (e) => {
          e.preventDefault();
          if (await logic.makeKey(name)) setName('');
        }}
      >
        <Input
          aria-label="Key name"
          placeholder="Which sorter, e.g. Bench sorter"
          value={name}
          onChange={(e) => setName(e.target.value)}
          className="w-64"
        />
        <Button type="submit" disabled={logic.busy || !name.trim()}>
          <KeyRound /> Make a key
        </Button>
      </form>
      {logic.newKey && (
        <div
          role="status"
          className="flex flex-col gap-2 rounded-xl border border-primary bg-card p-4"
        >
          <p className="text-sm font-medium">Copy this key now: it won’t be shown again.</p>
          <div className="flex gap-2">
            <Input
              readOnly
              value={logic.newKey}
              className="font-mono"
              onFocus={(e) => e.target.select()}
            />
            <Button variant="outline" onClick={copy}>
              <Copy /> Copy
            </Button>
          </div>
          <Button variant="ghost" size="sm" className="self-start" onClick={logic.clearNewKey}>
            Done
          </Button>
        </div>
      )}
      <ul className="flex flex-col gap-2">
        {(logic.keys ?? []).map((k) => (
          <li
            key={k.id}
            className="flex flex-wrap items-center gap-3 rounded-xl border border-border bg-card px-4 py-3"
          >
            <div className="min-w-0 flex-1">
              <p className="font-medium">
                {k.name}{' '}
                <span className="font-mono text-sm text-muted-foreground">{k.prefix}…</span>
              </p>
              <p className="text-sm text-muted-foreground">Last used {when(k.last_used_at)}</p>
            </div>
            {k.revoked_at ? (
              <Badge variant="idle">Revoked</Badge>
            ) : (
              <Button variant="outline" size="sm" onClick={() => logic.askRevokeKey(k)}>
                Revoke
              </Button>
            )}
          </li>
        ))}
      </ul>
    </section>
  );
}
