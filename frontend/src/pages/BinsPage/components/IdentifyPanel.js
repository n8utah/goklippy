import { useState } from 'react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { identifySorterPiece } from '@/helpers/api';

import { ROUTE_REASONS } from '../BinsPage.logic';

const MAX_PHOTOS = 8;
const pct = (score) => `${Math.round(score * 100)}%`;

/**
 * Identify a piece from photos (Brickognize) and see where the active profile would send
 * it, without the cameras. Nothing is saved.
 */
export default function IdentifyPanel() {
  const [files, setFiles] = useState([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [result, setResult] = useState(null);

  const identify = async () => {
    setBusy(true);
    setError('');
    setResult(null);
    try {
      setResult(await identifySorterPiece(files));
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  };

  const items = result?.brickognize.items.slice(0, 3) ?? [];
  const color = result?.brickognize.colors[0];
  const route = result?.route;

  return (
    <section className="flex flex-col gap-4 rounded-xl border border-border bg-card p-5">
      <div>
        <h2 className="font-display text-xl font-semibold">Identify from photos</h2>
        <p className="text-sm text-muted-foreground">
          Up to {MAX_PHOTOS} photos of one piece (JPEG or PNG). They go to Brickognize over the
          internet. Nothing is saved.
        </p>
      </div>
      <div className="flex flex-wrap items-center gap-3">
        <Input
          type="file"
          accept="image/jpeg,image/png"
          multiple
          aria-label="Photos of the piece"
          className="w-auto max-w-full"
          onChange={(e) => {
            setFiles(Array.from(e.target.files || []).slice(0, MAX_PHOTOS));
            setResult(null);
          }}
        />
        <Button disabled={busy || files.length === 0} onClick={identify}>
          {busy ? 'Asking Brickognize…' : `Identify (${files.length})`}
        </Button>
      </div>
      {error && <p className="text-sm text-destructive">{error}</p>}
      {result && (
        <div className="flex flex-col gap-3 border-t border-border pt-4">
          {items.length === 0 ? (
            <p className="text-sm text-muted-foreground">No part found.</p>
          ) : (
            <ul className="grid gap-2 sm:grid-cols-3">
              {items.map((item, i) => (
                <li
                  key={`${item.id}-${item.rank}`}
                  className={`flex items-center gap-3 rounded-md border px-2 py-1.5 ${i === 0 ? 'border-primary/50' : 'border-border'}`}
                >
                  {item.img_url ? (
                    <img
                      src={item.img_url}
                      alt=""
                      className="size-10 rounded bg-white object-contain"
                    />
                  ) : (
                    <span className="size-10 rounded bg-muted" />
                  )}
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-sm">{item.name}</span>
                    <span className="block truncate text-xs text-muted-foreground">
                      {item.id} · {item.category}
                    </span>
                  </span>
                  <span className="font-mono text-xs">{pct(item.score)}</span>
                </li>
              ))}
            </ul>
          )}
          {color && (
            <p className="text-sm">
              Color: <strong>{color.name}</strong>{' '}
              <span className="text-muted-foreground">
                (id {color.id}, {pct(color.score)})
              </span>
            </p>
          )}
          {route && (
            <p className="text-sm">
              <strong>{route.category_name}</strong> →{' '}
              {route.choice.bin
                ? `layer ${route.choice.bin.layer}, bin ${route.choice.bin.bin}`
                : 'the bucket'}
              : {ROUTE_REASONS[route.choice.reason] ?? route.choice.reason}.
            </p>
          )}
          {result.route_error && (
            <p className="text-sm text-muted-foreground">{result.route_error}</p>
          )}
        </div>
      )}
    </section>
  );
}
