import { useState } from 'react';

import { Input } from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { checkSettingValue } from '@/helpers/settingsRegistry';

/**
 * The control for one registry setting, by its kind. `onChange` gets only values that can
 * be saved; a number being typed that can't (empty, out of range) shows why and waits.
 * `onProblem` hears that reason ('' once it's fine), for a Save button to wait on.
 */
export default function SettingInput({ id, setting, value, onChange, onProblem, autoFocus }) {
  switch (setting.kind) {
    case 'boolean':
      return (
        <Switch
          id={id}
          checked={Boolean(value)}
          onCheckedChange={onChange}
          aria-label={setting.label}
        />
      );
    case 'enum':
      return (
        <Select value={value} onValueChange={onChange}>
          <SelectTrigger id={id} className="w-56" aria-label={setting.label}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {setting.options.map((o) => (
              <SelectItem key={o.value} value={o.value}>
                {o.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      );
    case 'text':
      return (
        <Input
          id={id}
          className="w-full sm:w-64"
          value={value ?? ''}
          autoFocus={autoFocus}
          onChange={(e) => onChange(e.target.value)}
        />
      );
    default:
      return (
        <NumberInput
          id={id}
          setting={setting}
          value={value}
          onChange={onChange}
          onProblem={onProblem}
          autoFocus={autoFocus}
        />
      );
  }
}

function NumberInput({ id, setting, value, onChange, onProblem, autoFocus }) {
  const [text, setText] = useState(String(value ?? ''));
  const [problem, setProblem] = useState('');
  // When the value changes from outside (reset, another page), show it, unless it's what
  // is being typed ("2." is 2).
  const [shown, setShown] = useState(value);
  if (value !== shown) {
    setShown(value);
    if (Number(text) !== value) {
      setText(String(value ?? ''));
      setProblem('');
    }
  }

  const report = (p) => {
    setProblem(p);
    onProblem?.(p);
  };

  const errorId = `${id}-problem`;
  return (
    <div className="flex flex-col items-start gap-1 sm:items-end">
      <div className="flex items-center gap-2">
        <Input
          id={id}
          type="number"
          inputMode="decimal"
          className="w-28 text-right font-mono"
          value={text}
          min={setting.min}
          max={setting.max}
          step={setting.step}
          autoFocus={autoFocus}
          aria-invalid={problem ? true : undefined}
          aria-describedby={problem ? errorId : undefined}
          onChange={(e) => {
            setText(e.target.value);
            const n = e.target.value === '' ? NaN : Number(e.target.value);
            const p = checkSettingValue(setting, n);
            report(p);
            if (!p) onChange(n);
          }}
          onBlur={() => {
            // Leaving a value that can't be saved puts back the saved one.
            if (problem) {
              setText(String(value ?? ''));
              report('');
            }
          }}
        />
        {setting.unit && (
          <span className="min-w-8 text-sm text-muted-foreground">{setting.unit}</span>
        )}
      </div>
      {problem && (
        <p id={errorId} className="text-xs text-destructive">
          {problem}
        </p>
      )}
    </div>
  );
}
