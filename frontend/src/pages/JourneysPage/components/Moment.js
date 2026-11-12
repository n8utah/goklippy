import { sorterJourneyFileURL } from '@/helpers/api';

/** The whole frame at a moment, with every piece's box; this piece's in green. */
export default function Moment({ journeyId, moment, frameW, frameH }) {
  return (
    <figure className="flex flex-col gap-1">
      <div className="relative overflow-hidden rounded-lg bg-media">
        <img
          src={sorterJourneyFileURL(journeyId, moment.image)}
          alt={moment.label}
          className="w-full"
        />
        <svg
          viewBox={`0 0 ${frameW} ${frameH}`}
          preserveAspectRatio="none"
          className="pointer-events-none absolute inset-0 size-full"
          aria-hidden
        >
          {moment.boxes.map((b, i) => (
            <rect
              key={i}
              x={b.x1}
              y={b.y1}
              width={b.x2 - b.x1}
              height={b.y2 - b.y1}
              fill="none"
              stroke={i === moment.piece ? '#22c55e' : '#f97316'}
              strokeWidth={Math.max(2, frameW / 320)}
            />
          ))}
        </svg>
      </div>
      <figcaption className="text-xs text-muted-foreground">
        {moment.label} · {new Date(moment.at).toLocaleTimeString()}
      </figcaption>
    </figure>
  );
}
