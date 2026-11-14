import { cn } from '@/lib/utils';

const BRICKS = ['fill-series-1', 'fill-primary', 'fill-series-3'];

/**
 * The logo: three 1×1 bricks side by side, one colour each (sorted). It's wider than
 * tall, so size it by height (`h-5`); the width follows.
 */
export default function BrandMark({ className }) {
  return (
    <svg viewBox="6 16 52 32" className={cn('w-auto shrink-0', className)} aria-hidden>
      {BRICKS.map((fill, i) => (
        <g key={fill} className={fill} transform={`translate(${i * 18} 0)`}>
          <rect x="6" y="23" width="16" height="25" rx="2.5" />
          <rect x="9.5" y="16" width="9" height="9" rx="1.5" />
        </g>
      ))}
    </svg>
  );
}
