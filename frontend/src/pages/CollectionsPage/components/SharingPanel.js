import { CircleAlert, CircleCheck, Share2 } from 'lucide-react';
import { useState } from 'react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';

const ago = (t) => {
  if (!t) return '';
  const s = Math.round((Date.now() - new Date(t).getTime()) / 1000);
  if (s < 60) return 'just now';
  if (s < 3600) return `${Math.round(s / 60)} min ago`;
  return new Date(t).toLocaleString();
};

/** How sending is going, in a line. */
export function SharingStatus({ status }) {
  if (!status?.url || !status?.key_set) {
    return <span className="text-muted-foreground">Not set up</span>;
  }
  if (status.last_error) {
    return (
      <span className="flex items-start gap-1.5 text-destructive">
        <CircleAlert className="mt-0.5 size-4 shrink-0" />
        <span>
          {status.waiting > 0 && `${status.waiting.toLocaleString()} waiting. `}
          {status.last_error}
        </span>
      </span>
    );
  }
  if (status.waiting > 0) {
    return (
      <span className="text-muted-foreground">
        Sending… {status.waiting.toLocaleString()} waiting
      </span>
    );
  }
  return (
    <span className="flex items-center gap-1.5 text-running-text">
      <CircleCheck className="size-4" />
      Up to date{status.last_sent ? ` · last sent ${ago(status.last_sent)}` : ''}
    </span>
  );
}

/**
 * Where shared collections go: the sort viewer site and the key it gave this sorter
 * (made on the site under Admin › Sorter keys). The saved key is never shown again.
 */
export default function SharingPanel({ sharing }) {
  const { status } = sharing;
  const [editing, setEditing] = useState(false);
  const [url, setUrl] = useState('');
  const [key, setKey] = useState('');

  const startEditing = () => {
    setUrl(status?.url || 'https://lego.n8p.xyz');
    setKey('');
    setEditing(true);
  };

  return (
    <section className="flex flex-col gap-3 rounded-xl border border-border bg-card p-5">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <h2 className="flex items-center gap-2 font-display text-lg font-semibold">
            <Share2 className="size-5" /> Sharing
          </h2>
          <p className="text-sm text-muted-foreground">
            Shared collections are sent to the sort viewer site as they sort, so their owners can
            watch.{' '}
            {status?.url && (
              <a
                href={status.url}
                target="_blank"
                rel="noreferrer"
                className="font-medium text-primary-text hover:underline"
              >
                {sharing.siteHost}
              </a>
            )}
          </p>
          <div className="mt-1 text-sm">
            <SharingStatus status={status} />
          </div>
        </div>
        {!editing && (
          <div className="flex shrink-0 gap-2">
            <Button
              variant="outline"
              disabled={sharing.busy || !sharing.ready}
              onClick={sharing.test}
            >
              Test
            </Button>
            <Button variant="outline" onClick={startEditing}>
              {sharing.ready ? 'Change' : 'Set up'}
            </Button>
          </div>
        )}
      </div>
      {editing && (
        <form
          className="flex flex-col gap-3 sm:flex-row sm:items-end"
          onSubmit={async (e) => {
            e.preventDefault();
            if (await sharing.save(url, key)) setEditing(false);
          }}
        >
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="sharing-url">Site</Label>
            <Input
              id="sharing-url"
              className="sm:w-64"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              placeholder="https://lego.n8p.xyz"
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="sharing-key">Key</Label>
            <Input
              id="sharing-key"
              className="font-mono sm:w-72"
              autoComplete="off"
              value={key}
              onChange={(e) => setKey(e.target.value)}
              placeholder={status?.key_set ? 'Saved; paste a new one to change it' : 'sv_…'}
            />
          </div>
          <div className="flex gap-2">
            <Button type="submit" disabled={sharing.busy}>
              Save
            </Button>
            <Button type="button" variant="ghost" onClick={() => setEditing(false)}>
              Cancel
            </Button>
          </div>
        </form>
      )}
    </section>
  );
}
