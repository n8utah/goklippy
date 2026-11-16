import { useState } from 'react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { checkEditorPiece } from '@/helpers/api';

const UNKNOWN = '__unknown__';

/** "Where would this go?" against the rules as they are now, saved or not. */
export default function WhereCheck({ doc, colors }) {
  const [part, setPart] = useState('');
  const [color, setColor] = useState(UNKNOWN);
  const [result, setResult] = useState(null);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  const check = async () => {
    setBusy(true);
    setError('');
    try {
      setResult(await checkEditorPiece(doc, part.trim(), color === UNKNOWN ? '' : color));
    } catch (err) {
      setResult(null);
      setError(err.message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="flex flex-col gap-3">
      <form
        className="flex flex-wrap gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          if (part.trim()) check();
        }}
      >
        <Input
          value={part}
          onChange={(e) => setPart(e.target.value)}
          placeholder="Part number, like 3001"
          className="w-40 font-mono"
          aria-label="Part number"
        />
        <Select value={color} onValueChange={setColor}>
          <SelectTrigger className="w-48" aria-label="Colour">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={UNKNOWN}>Colour unknown</SelectItem>
            {colors.map((c) => (
              <SelectItem key={c.value} value={c.value}>
                {c.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Button type="submit" variant="outline" disabled={busy || !part.trim()}>
          Check
        </Button>
      </form>
      {error && <p className="text-sm text-destructive">{error}</p>}
      {result && (
        <div className="rounded-lg bg-muted px-3 py-2.5 text-sm">
          <p>
            {result.part_name ? (
              <>
                <span className="font-medium">{result.part_name}</span>{' '}
                <span className="text-muted-foreground">({result.part_category})</span>
              </>
            ) : (
              <span className="text-muted-foreground">
                The catalog hasn’t seen <span className="font-mono">{result.part_id}</span>, so its
                type and size aren’t known; only part numbers and colours can match it.
              </span>
            )}
          </p>
          <p className="mt-1">
            {result.rule_name ? (
              <>
                Taken by <span className="font-medium">{result.rule_name}</span>, into{' '}
                <span className="font-medium">{result.category_name}</span>.
              </>
            ) : (
              <>
                No rule takes it: it goes to{' '}
                <span className="font-medium">{result.category_name}</span>.
              </>
            )}
          </p>
        </div>
      )}
    </div>
  );
}
