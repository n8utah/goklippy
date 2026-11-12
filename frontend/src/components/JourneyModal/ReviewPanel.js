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
import { cn } from '@/lib/utils';

const ISSUES = [
  { id: 'not_lego', label: 'Not LEGO' },
  { id: 'multiple_pieces', label: 'Several pieces in frame' },
  { id: 'no_piece', label: 'No piece in frame' },
  { id: 'assembly', label: 'Assembly' },
  { id: 'entangled', label: 'Pieces entangled' },
  { id: 'blurry', label: 'Blurry' },
];
const PICK = '__pick__';

function YesNo({ label, value, onChange }) {
  return (
    <div
      className="inline-flex rounded-md border border-border p-0.5"
      role="group"
      aria-label={label}
    >
      {[
        [true, 'Yes'],
        [false, 'No'],
      ].map(([v, text]) => (
        <button
          key={text}
          type="button"
          aria-pressed={value === v}
          onClick={() => onChange(v)}
          className={cn(
            'h-8 rounded px-3 text-sm font-medium pointer-coarse:h-10',
            value === v
              ? 'bg-primary text-primary-foreground'
              : 'text-muted-foreground hover:bg-accent',
          )}
        >
          {text}
        </button>
      ))}
    </div>
  );
}

/**
 * A person's check of the identification. Every click saves the whole review, so what's
 * shown is what's stored.
 */
export default function ReviewPanel({ piece, onSave }) {
  const saved = piece.review;
  const [review, setReview] = useState({
    part_correct: saved?.part_correct ?? null,
    color_correct: saved?.color_correct ?? null,
    color_id: saved?.color_id ?? '',
    issues: saved?.issues ?? [],
  });
  const [status, setStatus] = useState('');
  const colors = piece.prediction?.colors ?? [];

  const save = async (patch) => {
    const next = { ...review, ...patch };
    if (next.color_correct !== false) next.color_id = '';
    setReview(next);
    setStatus('Saving…');
    try {
      await onSave(next);
      setStatus('Saved');
    } catch (err) {
      setStatus(`Not saved: ${err.message}`);
    }
  };

  const toggleIssue = (id) =>
    save({
      issues: review.issues.includes(id)
        ? review.issues.filter((i) => i !== id)
        : [...review.issues, id],
    });

  return (
    <section aria-labelledby="journey-review" className="flex flex-col gap-3">
      <div className="flex items-baseline justify-between">
        <h3 id="journey-review" className="font-display text-base font-semibold">
          Your check
        </h3>
        {status && (
          <span
            className={cn(
              'text-xs',
              status.startsWith('Not') ? 'text-destructive' : 'text-muted-foreground',
            )}
          >
            {status}
          </span>
        )}
      </div>
      <div className="flex flex-wrap items-center gap-x-6 gap-y-3 text-sm">
        <span className="flex items-center gap-2">
          Part right?
          <YesNo
            label="Part right?"
            value={review.part_correct}
            onChange={(v) => save({ part_correct: v })}
          />
        </span>
        <span className="flex items-center gap-2">
          Color right?{piece.color_name ? ` (${piece.color_name})` : ''}
          <YesNo
            label="Color right?"
            value={review.color_correct}
            onChange={(v) => save({ color_correct: v })}
          />
        </span>
      </div>
      {review.color_correct === false && (
        <div className="flex flex-wrap items-center gap-2 text-sm">
          It’s
          <Select
            value={colors.some((c) => c.id === review.color_id) ? review.color_id : PICK}
            onValueChange={(v) => v !== PICK && save({ color_id: v })}
          >
            <SelectTrigger size="sm" className="w-44">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={PICK}>pick one</SelectItem>
              {colors.map((c) => (
                <SelectItem key={c.id} value={c.id}>
                  {c.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          or color id
          <Input
            className="h-9 w-20 font-mono"
            aria-label="Color id"
            value={review.color_id}
            onChange={(e) => setReview({ ...review, color_id: e.target.value })}
            onBlur={() => save({})}
          />
        </div>
      )}
      <div className="flex flex-wrap gap-2" role="group" aria-label="Photo issues">
        {ISSUES.map((i) => (
          <Button
            key={i.id}
            size="sm"
            variant={review.issues.includes(i.id) ? 'default' : 'outline'}
            aria-pressed={review.issues.includes(i.id)}
            onClick={() => toggleIssue(i.id)}
          >
            {i.label}
          </Button>
        ))}
      </div>
    </section>
  );
}
