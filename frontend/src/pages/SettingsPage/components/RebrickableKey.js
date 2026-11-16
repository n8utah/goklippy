import { useEffect, useState } from 'react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { getRebrickable, putRebrickable } from '@/helpers/api';

/**
 * The sorter's Rebrickable API key: the backup for set rules when the sort viewer site
 * can't answer. The key itself is never shown.
 */
export default function RebrickableKey() {
  const [keySet, setKeySet] = useState(null);
  const [key, setKey] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');

  useEffect(() => {
    getRebrickable().then(
      (r) => setKeySet(r.key_set),
      (err) => setError(err.message),
    );
  }, []);

  const save = async (value) => {
    setBusy(true);
    setError('');
    setNotice('');
    try {
      const r = await putRebrickable(value);
      setKeySet(r.key_set);
      setKey('');
      setNotice(r.key_set ? 'Key saved.' : 'Key removed.');
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="flex flex-col gap-3 py-5">
      <div>
        <h3 className="font-medium">Rebrickable API key (backup)</h3>
        <p className="text-sm text-muted-foreground">
          {keySet === null
            ? 'Loading…'
            : keySet
              ? 'A key is saved. Paste a new one to replace it.'
              : 'None, and none is needed while the sorter is connected to the sort viewer site. For a backup, make a free account on rebrickable.com and copy the key from Settings › API.'}
        </p>
      </div>
      <form
        className="flex flex-wrap items-end gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          if (key.trim()) save(key.trim());
        }}
      >
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="rebrickable-key" className="sr-only">
            Rebrickable API key
          </Label>
          <Input
            id="rebrickable-key"
            type="password"
            autoComplete="off"
            value={key}
            onChange={(e) => setKey(e.target.value)}
            placeholder={keySet ? '••••••••' : 'Paste the key'}
            className="w-80 max-w-full font-mono"
          />
        </div>
        <Button type="submit" disabled={busy || !key.trim()}>
          Save key
        </Button>
        {keySet && (
          <Button type="button" variant="ghost" disabled={busy} onClick={() => save('')}>
            Remove
          </Button>
        )}
      </form>
      {error && <p className="text-sm text-destructive">{error}</p>}
      {notice && <p className="text-sm text-muted-foreground">{notice}</p>}
    </div>
  );
}
