import { Plus, Trash2 } from 'lucide-react';
import { useState } from 'react';
import { Link } from 'react-router';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { lookupSorterSet } from '@/helpers/api';

/**
 * A set rule's sets: type a LEGO set number and its parts come from the sort viewer
 * site, or Rebrickable's API as the backup (minifigures and spare parts left out). Each
 * set fills its own bins.
 */
export default function SetPicker({ sets, onChange, disabled }) {
  const [number, setNumber] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(null); // { message, needsSource }

  const add = async () => {
    const n = number.trim();
    if (!n) return;
    setBusy(true);
    setError(null);
    try {
      const set = await lookupSorterSet(n);
      if (sets.some((s) => s.number === set.number)) {
        setError({ message: `${set.number} is already in this rule.` });
      } else {
        onChange([...sets, set]);
        setNumber('');
      }
    } catch (err) {
      setError({ message: err.message, needsSource: Boolean(err.body?.needs_source) });
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="flex flex-col gap-3">
      {sets.length > 0 && (
        <ul className="flex flex-col gap-2">
          {sets.map((s) => {
            const pieces = s.parts.reduce((n, l) => n + l.need, 0);
            return (
              <li
                key={s.number}
                className="flex items-center gap-3 rounded-lg border border-border p-2.5"
              >
                {s.image_url ? (
                  <img
                    src={s.image_url}
                    alt=""
                    className="size-12 shrink-0 rounded bg-white object-contain"
                  />
                ) : (
                  <span className="size-12 shrink-0 rounded bg-muted" />
                )}
                <span className="min-w-0 flex-1">
                  <span className="block truncate font-medium">
                    <span className="font-mono">{s.number}</span> {s.name}
                  </span>
                  <span className="block text-sm text-muted-foreground">
                    {s.year ? `${s.year} · ` : ''}
                    {s.source === 'rebrickable' ? 'from Rebrickable · ' : ''}
                    <span className="font-mono">{pieces.toLocaleString()}</span> pieces in{' '}
                    <span className="font-mono">{s.parts.length}</span> part and colour lines
                    {s.unmapped?.length > 0 && (
                      <span title={s.unmapped.join('\n')}>
                        {' '}
                        · {s.unmapped.length} with no BrickLink colour (any colour counts)
                      </span>
                    )}
                  </span>
                </span>
                {!disabled && (
                  <Button
                    variant="ghost"
                    size="icon"
                    aria-label={`Remove ${s.number}`}
                    onClick={() => onChange(sets.filter((x) => x.number !== s.number))}
                  >
                    <Trash2 />
                  </Button>
                )}
              </li>
            );
          })}
        </ul>
      )}
      {!disabled && (
        <form
          className="flex gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            add();
          }}
        >
          <Input
            value={number}
            onChange={(e) => setNumber(e.target.value)}
            placeholder="Set number, like 75192 or 10294-1"
            className="max-w-64 font-mono"
            aria-label="Set number"
          />
          <Button type="submit" variant="outline" disabled={busy || !number.trim()}>
            <Plus />
            {busy ? 'Looking it up…' : 'Add set'}
          </Button>
        </form>
      )}
      {error && (
        <p className="text-sm text-destructive">
          {error.needsSource ? (
            <>
              Sets come from the sort viewer site, and the sorter isn’t connected to it.{' '}
              <Link to="/run/collections" className="font-medium text-primary-text hover:underline">
                Connect it under Sharing
              </Link>
              , or add a{' '}
              <Link
                to="/system/settings/services"
                className="font-medium text-primary-text hover:underline"
              >
                Rebrickable API key
              </Link>{' '}
              as the backup.
            </>
          ) : (
            error.message
          )}
        </p>
      )}
    </div>
  );
}
