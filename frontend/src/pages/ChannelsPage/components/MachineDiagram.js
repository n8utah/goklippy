import { CHANNEL_IDS } from '@/helpers/useMachineMotors';
import { cn } from '@/lib/utils';

const CY = 130;
const RINGS = {
  c1: { cx: 105, radius: 62 },
  c2: { cx: 290, radius: 62 },
  c3: { cx: 475, radius: 62 },
  c4: { cx: 700, radius: 100 },
};
const GAP_DEGREES = 50; // the C's opening, facing the next stage

function polar(cx, radius, degrees) {
  const rad = (degrees * Math.PI) / 180;
  return [cx + radius * Math.cos(rad), CY + radius * Math.sin(rad)];
}

function statorPath(cx, radius) {
  const [sx, sy] = polar(cx, radius, GAP_DEGREES / 2);
  const [ex, ey] = polar(cx, radius, -GAP_DEGREES / 2);
  return `M ${sx} ${sy} A ${radius} ${radius} 0 1 1 ${ex} ${ey}`;
}

const facetPoints = (cx, radius, facets) =>
  Array.from({ length: facets }, (_, i) => polar(cx, radius, (360 / facets) * i).join(',')).join(
    ' ',
  );

const arrow = (direction) => (direction > 0 ? '↻' : direction < 0 ? '↺' : '■');

function Stage({ channel, ring, slots, current, onPick }) {
  const { cx, radius } = ring;
  const moving = channel.direction !== 0;
  const missing = channel.oid == null;
  const c4 = channel.id === 'c4';
  const [dotX, dotY] = polar(cx, radius - 36, 0);

  return (
    <g
      role="link"
      tabIndex={0}
      aria-label={`${channel.id.toUpperCase()}: ${missing ? 'no motor' : `${channel.measuredRpm.toFixed(1)} rpm`}`}
      onClick={() => onPick(channel.id)}
      onKeyDown={(e) => (e.key === 'Enter' || e.key === ' ') && onPick(channel.id)}
      className={cn(
        'cursor-pointer outline-none focus-visible:[&>path]:stroke-ring',
        missing && 'opacity-50',
      )}
    >
      <path
        d={statorPath(cx, radius)}
        fill="none"
        strokeWidth={current ? 10 : 8}
        strokeLinecap="round"
        className={current ? 'stroke-primary' : moving ? 'stroke-running' : 'stroke-border-strong'}
      />
      <g
        style={{
          transform: `rotate(${channel.angle}deg)`,
          transformOrigin: `${cx}px ${CY}px`,
          transition: 'transform 220ms linear',
        }}
      >
        {c4 ? (
          <>
            <circle cx={cx} cy={CY} r={radius - 12} className="fill-muted stroke-border-strong" />
            {Array.from({ length: slots }, (_, i) => {
              const [x, y] = polar(cx, radius - 12, (360 / slots) * (i + 0.5));
              return (
                <line
                  key={i}
                  x1={cx}
                  y1={CY}
                  x2={x}
                  y2={y}
                  className="stroke-border-strong"
                  strokeWidth={2}
                />
              );
            })}
            <circle cx={dotX} cy={dotY} r={7} className="fill-primary" />
          </>
        ) : (
          <>
            <polygon
              points={facetPoints(cx, radius - 14, 8)}
              className="fill-muted stroke-border-strong"
            />
            <line
              x1={cx}
              y1={CY}
              x2={cx + radius - 14}
              y2={CY}
              className="stroke-primary"
              strokeWidth={3}
            />
          </>
        )}
      </g>
      <circle cx={cx} cy={CY} r={22} className="fill-card stroke-border-strong" />
      <text
        x={cx}
        y={CY - 2}
        textAnchor="middle"
        className="fill-foreground font-display text-[15px] font-semibold"
      >
        {channel.id.toUpperCase()}
      </text>
      <text x={cx} y={CY + 13} textAnchor="middle" className="fill-muted-foreground text-[11px]">
        {arrow(channel.direction)}
      </text>
      <text
        x={cx}
        y={CY + radius + 26}
        textAnchor="middle"
        className="fill-muted-foreground font-mono text-[13px]"
      >
        {missing ? 'no motor' : `${channel.measuredRpm.toFixed(1)} rpm`}
      </text>
    </g>
  );
}

function Drop({ from, to }) {
  const x1 = RINGS[from].cx + RINGS[from].radius + 6;
  const x2 = RINGS[to].cx - RINGS[to].radius - 8;
  return (
    <path
      d={`M ${x1} ${CY} L ${x2} ${CY}`}
      className="stroke-muted-foreground"
      strokeWidth={2}
      strokeDasharray="4 4"
      markerEnd="url(#channels-arrow)"
    />
  );
}

/** C1 → C2 → C3 → C4 from above, turning live. A channel opens its tab. */
export default function MachineDiagram({ channels, indexDegrees, current, onPick }) {
  const slots = Math.max(1, Math.round(360 / indexDegrees));
  return (
    <svg
      viewBox="0 0 830 275"
      className="h-auto w-full max-w-4xl"
      role="group"
      aria-label="C1, C2 and C3 feed C4, seen from above"
    >
      <defs>
        <marker
          id="channels-arrow"
          viewBox="0 0 10 10"
          refX="8"
          refY="5"
          markerWidth="7"
          markerHeight="7"
          orient="auto-start-reverse"
        >
          <path d="M 0 0 L 10 5 L 0 10 z" className="fill-muted-foreground" />
        </marker>
      </defs>
      <Drop from="c1" to="c2" />
      <Drop from="c2" to="c3" />
      <Drop from="c3" to="c4" />
      {CHANNEL_IDS.map((id) => (
        <Stage
          key={id}
          channel={channels[id]}
          ring={RINGS[id]}
          slots={slots}
          current={current === id}
          onPick={onPick}
        />
      ))}
    </svg>
  );
}
