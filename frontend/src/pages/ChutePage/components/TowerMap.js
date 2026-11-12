import { cn } from '@/lib/utils';

const CENTER = 150;
const OUTER = 138;
const INNER = 92;

/** A point on the ring; 0° (home) is at the top and angles run clockwise. */
function point(radius, degrees) {
  const rad = ((degrees - 90) * Math.PI) / 180;
  return [CENTER + radius * Math.cos(rad), CENTER + radius * Math.sin(rad)];
}

function arcPath(start, end, inner = INNER, outer = OUTER) {
  const large = end - start > 180 ? 1 : 0;
  const [ox1, oy1] = point(outer, start);
  const [ox2, oy2] = point(outer, end);
  const [ix2, iy2] = point(inner, end);
  const [ix1, iy1] = point(inner, start);
  return `M ${ox1} ${oy1} A ${outer} ${outer} 0 ${large} 1 ${ox2} ${oy2} L ${ix2} ${iy2} A ${inner} ${inner} 0 ${large} 0 ${ix1} ${iy1} Z`;
}

/** The bin the chute is over, if it's within half a bin of one. */
function pointedBin(bins, angle, binWidth) {
  if (angle == null) return null;
  return bins.find((b) => Math.abs(b.angle - angle) <= binWidth / 2)?.bin ?? null;
}

/**
 * One layer from above: its bins where the chute aims for them, the wedge past the travel
 * limit, and the chute. A bin is a button that sends the chute there.
 */
export default function TowerMap({ bins, geometry, angle, disabled, onBin }) {
  if (!geometry || bins.length === 0) {
    return (
      <p className="text-sm text-muted-foreground">
        This layer’s bin count doesn’t split evenly into the sections.
      </p>
    );
  }
  const perSection = bins.length / geometry.sections;
  const binWidth = geometry.section_width / perSection;
  const pointed = pointedBin(bins, angle, binWidth);

  return (
    <svg
      viewBox="0 0 300 300"
      className="mx-auto h-auto w-full max-w-sm"
      role="group"
      aria-label="This layer's bins from above"
    >
      <path
        d={arcPath(geometry.max_angle, 360, INNER - 6, OUTER + 6)}
        className="fill-destructive/15"
      >
        <title>{`No-go: the chute can't pass ${geometry.max_angle}°`}</title>
      </path>
      {bins.map((b) => {
        const clickable = b.reachable && !disabled;
        const here = b.bin === pointed;
        const [x, y] = point((OUTER + INNER) / 2, b.angle);
        return (
          <g
            key={b.bin}
            role={clickable ? 'button' : undefined}
            tabIndex={clickable ? 0 : undefined}
            aria-label={
              b.reachable
                ? `Bin ${b.bin}, section ${b.section}, ${b.angle.toFixed(1)}°`
                : `Bin ${b.bin}, out of reach`
            }
            onClick={clickable ? () => onBin(b.bin) : undefined}
            onKeyDown={
              clickable ? (e) => (e.key === 'Enter' || e.key === ' ') && onBin(b.bin) : undefined
            }
            className={cn(
              clickable &&
                'cursor-pointer outline-none [&:hover>path]:fill-accent [&:focus-visible>path]:stroke-ring',
            )}
          >
            <path
              d={arcPath(b.angle - binWidth / 2 + 0.6, b.angle + binWidth / 2 - 0.6)}
              strokeWidth={1.5}
              className={cn(
                !b.reachable
                  ? 'fill-muted stroke-border opacity-50'
                  : here
                    ? 'fill-primary stroke-primary'
                    : 'fill-card stroke-border-strong',
              )}
            />
            <text
              x={x}
              y={y}
              textAnchor="middle"
              dominantBaseline="central"
              className={cn(
                'font-mono text-[11px]',
                here ? 'fill-primary-foreground' : 'fill-foreground',
              )}
            >
              {b.bin}
            </text>
          </g>
        );
      })}
      <text x={CENTER} y={8} textAnchor="middle" className="fill-muted-foreground text-[10px]">
        home
      </text>
      {angle != null && (
        <g
          style={{
            transform: `rotate(${angle}deg)`,
            transformOrigin: `${CENTER}px ${CENTER}px`,
            transition: 'transform 300ms linear',
          }}
        >
          <line
            x1={CENTER}
            y1={CENTER}
            x2={CENTER}
            y2={CENTER - INNER + 8}
            strokeWidth={6}
            strokeLinecap="round"
            className="stroke-foreground"
          />
          <path
            d={`M ${CENTER - 11} ${CENTER - INNER + 18} L ${CENTER} ${CENTER - INNER + 1} L ${CENTER + 11} ${CENTER - INNER + 18} Z`}
            className="fill-foreground"
          />
        </g>
      )}
      <circle cx={CENTER} cy={CENTER} r={20} className="fill-card stroke-border-strong" />
    </svg>
  );
}
