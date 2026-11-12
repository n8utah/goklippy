import { cn } from '@/lib/utils';

/**
 * A small set of mutually exclusive choices as joined buttons (Forward / Reverse,
 * Continuous / Distance). `options` is [{ value, label }].
 */
export default function Segmented({ label, value, options, onChange, disabled, className }) {
  return (
    <div
      role="radiogroup"
      aria-label={label}
      className={cn('inline-flex rounded-md border border-border p-0.5', className)}
    >
      {options.map((o) => (
        <button
          key={String(o.value)}
          type="button"
          role="radio"
          aria-checked={value === o.value}
          disabled={disabled}
          onClick={() => onChange(o.value)}
          className={cn(
            'h-8 rounded px-3 text-sm font-medium whitespace-nowrap disabled:opacity-50 pointer-coarse:h-10',
            value === o.value
              ? 'bg-primary text-primary-foreground'
              : 'text-muted-foreground hover:bg-accent hover:text-foreground',
          )}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}
