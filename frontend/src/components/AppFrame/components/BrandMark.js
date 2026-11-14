import { cn } from '@/lib/utils';

/**
 * The logo: three 1×1 bricks side by side, drawn in the current text colour (on the
 * rail it sits on the accent). It's wider than tall, so size it by width (`w-7`).
 */
export default function BrandMark({ className }) {
  return (
    <svg viewBox="6 16 52 32" className={cn('h-auto shrink-0 fill-current', className)} aria-hidden>
      {[0, 18, 36].map((x) => (
        <g key={x} transform={`translate(${x} 0)`}>
          <rect x="6" y="23" width="16" height="25" rx="2.5" />
          <rect x="9.5" y="16" width="9" height="9" rx="1.5" />
        </g>
      ))}
    </svg>
  );
}
