import { useState } from 'react';

import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';

/**
 * A labelled number with its unit, for values that aren't registry settings (a channel's
 * sections, a layer's servo angles). Typing that isn't a number in range waits; leaving the
 * field puts back the last good value.
 */
export default function NumberField({
  id,
  label,
  value,
  onChange,
  min,
  max,
  step = 1,
  unit,
  className,
  inputClassName,
  disabled,
}) {
  const [text, setText] = useState(String(value ?? ''));
  const [shown, setShown] = useState(value);
  if (value !== shown) {
    setShown(value);
    if (Number(text) !== value) setText(String(value ?? ''));
  }
  const ok = (n) =>
    text !== '' && Number.isFinite(n) && (min == null || n >= min) && (max == null || n <= max);

  return (
    <label htmlFor={id} className={cn('flex flex-col gap-1.5 text-sm', className)}>
      <span className="font-medium">{label}</span>
      <span className="flex items-center gap-2">
        <Input
          id={id}
          type="number"
          inputMode="decimal"
          min={min}
          max={max}
          step={step}
          value={text}
          disabled={disabled}
          className={cn('w-28 text-right font-mono', inputClassName)}
          onChange={(e) => {
            setText(e.target.value);
            const n = Number(e.target.value);
            if (
              e.target.value !== '' &&
              Number.isFinite(n) &&
              (min == null || n >= min) &&
              (max == null || n <= max)
            )
              onChange(n);
          }}
          onBlur={() => !ok(Number(text)) && setText(String(value ?? ''))}
        />
        {unit && <span className="text-muted-foreground">{unit}</span>}
      </span>
    </label>
  );
}
