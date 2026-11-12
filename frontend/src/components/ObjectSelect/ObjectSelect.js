import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';

const NONE = '__none__';

/**
 * Picks a board object (a stepper, a servo, an endstop) by its config section. One that's
 * chosen but not in the connected config stays listed, marked, so the choice isn't lost
 * while the board is away. `allowNone` adds a "None" choice for ''.
 */
export default function ObjectSelect({ id, value, objects, onChange, allowNone, className }) {
  const listed = objects.some((o) => o.section === value);
  return (
    <Select value={value || NONE} onValueChange={(v) => onChange(v === NONE ? '' : v)}>
      <SelectTrigger id={id} className={className ?? 'w-52'}>
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {allowNone && <SelectItem value={NONE}>None</SelectItem>}
        {value && !listed && (
          <SelectItem value={value}>
            {value} <span className="text-muted-foreground">(not connected)</span>
          </SelectItem>
        )}
        {objects.map((o) => (
          <SelectItem key={o.oid} value={o.section}>
            {o.section}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
