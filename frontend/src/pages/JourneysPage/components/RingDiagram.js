import { KEPT_COLOR, reasonInfo } from '../JourneysPage.logic';

const SIZE = 160;
const C = SIZE / 2;
const R = 62;

/** The channel from above: where each frame was taken around the ring, colored by decision. */
export default function RingDiagram({ track, onPick }) {
  return (
    <svg
      className="size-40 shrink-0"
      viewBox={`0 0 ${SIZE} ${SIZE}`}
      role="group"
      aria-label="Where each frame was taken around the channel"
    >
      <circle cx={C} cy={C} r={R} fill="none" strokeWidth="14" className="stroke-muted" />
      {track.frames.map((f, i) => {
        const a = (f.angle * Math.PI) / 180;
        return (
          <circle
            key={i}
            cx={C + R * Math.cos(a)}
            cy={C + R * Math.sin(a)}
            r={f.kept ? 5 : 3}
            fill={f.kept ? KEPT_COLOR : reasonInfo(f.reason).color}
            className={f.kept ? 'cursor-pointer stroke-card' : 'cursor-pointer'}
            strokeWidth={f.kept ? 1.5 : 0}
            onClick={() => onPick(i)}
          >
            <title>
              {f.angle.toFixed(0)}° · {f.kept ? 'kept' : f.reason}
            </title>
          </circle>
        );
      })}
      <text
        x={C}
        y={C + 5}
        textAnchor="middle"
        className="fill-foreground font-display text-sm font-semibold"
      >
        {track.channel.toUpperCase()}
      </text>
    </svg>
  );
}
