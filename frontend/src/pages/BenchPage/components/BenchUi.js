import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';

/** A bench card: title, a line on what it's for, an action on the right. */
export function BenchCard({ title, about, action, children, className }) {
  return (
    <section
      className={cn('flex flex-col gap-4 rounded-xl border border-border bg-card p-5', className)}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h2 className="font-display text-lg font-semibold">{title}</h2>
          {about && <p className="text-sm text-muted-foreground">{about}</p>}
        </div>
        {action}
      </div>
      {children}
    </section>
  );
}

/** Two buttons that scale by a factor, labelled with it: Slower ×0.8 · Faster ×1.25. */
export function ScaleButtons({ lower, higher, lowFactor, highFactor, disabled, onScale }) {
  return (
    <div className="grid grid-cols-2 gap-2">
      {[
        [lower, lowFactor],
        [higher, highFactor],
      ].map(([label, factor]) => (
        <Button key={label} variant="outline" disabled={disabled} onClick={() => onScale(factor)}>
          {label} <span className="font-mono text-xs text-muted-foreground">×{factor}</span>
        </Button>
      ))}
    </div>
  );
}

/**
 * Members to pick for a group or routine, each with an optional number beside it (a
 * speed or an angle). `values` is { key: number }; omit `onValue` for checkboxes only.
 */
export function MemberPicker({ options, chosen, onToggle, values, onValue, unit, min, max }) {
  return (
    <ul className="divide-y divide-border rounded-md border border-border">
      {options.map(({ key, label }) => {
        const checked = chosen.includes(key);
        return (
          <li key={key} className="flex items-center gap-3 px-3 py-2">
            <label className="flex min-w-0 flex-1 cursor-pointer items-center gap-2 text-sm">
              <Checkbox checked={checked} onCheckedChange={() => onToggle(key)} />
              <span className="truncate">{label}</span>
            </label>
            {onValue && (
              <span className="flex items-center gap-1.5">
                <Input
                  type="number"
                  min={min}
                  max={max}
                  className="h-9 w-24 text-right font-mono"
                  disabled={!checked}
                  value={checked ? values[key] : ''}
                  onChange={(e) => onValue(key, Number(e.target.value))}
                  aria-label={`${label} ${unit}`}
                />
                <span className="w-12 text-xs text-muted-foreground">{unit}</span>
              </span>
            )}
          </li>
        );
      })}
    </ul>
  );
}

/** A saved group or routine: name, what it holds, notes, and its buttons. */
export function SavedItem({ name, badge, summary, notes, onEdit, onDelete, children }) {
  return (
    <li className="flex flex-col gap-3 rounded-lg border border-border p-4">
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <p className="flex flex-wrap items-center gap-2 font-medium">
            {name}
            {badge}
          </p>
          <p className="text-sm text-muted-foreground">{summary}</p>
        </div>
        <div className="flex shrink-0 gap-1">
          <Button variant="ghost" size="sm" onClick={onEdit}>
            Edit
          </Button>
          <Button variant="ghost" size="sm" className="text-destructive" onClick={onDelete}>
            Delete
          </Button>
        </div>
      </div>
      {notes?.map((n) => (
        <p key={n} className="rounded-md bg-warning-bg px-3 py-1.5 text-sm text-warning">
          {n}
        </p>
      ))}
      {children}
    </li>
  );
}
