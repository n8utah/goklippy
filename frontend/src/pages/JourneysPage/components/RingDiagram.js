import { KEPT_COLOR, reasonInfo } from '../JourneysPage.logic';

import styles from '../JourneysPage.module.css';

const SIZE = 160;
const C = SIZE / 2;
const R = 62;

/** The channel seen from above: each frame's position around the ring, colored by decision. */
const RingDiagram = ({ track, onPick }) => (
  <svg
    className={styles.ring}
    viewBox={`0 0 ${SIZE} ${SIZE}`}
    role="img"
    aria-label="Where each frame was taken around the channel"
  >
    <circle cx={C} cy={C} r={R} fill="none" stroke="#e2e8f0" strokeWidth="14" />
    {track.frames.map((f, i) => {
      const a = (f.angle * Math.PI) / 180;
      return (
        <circle
          key={i}
          cx={C + R * Math.cos(a)}
          cy={C + R * Math.sin(a)}
          r={f.kept ? 5 : 3}
          fill={f.kept ? KEPT_COLOR : reasonInfo(f.reason).color}
          stroke={f.kept ? '#ffffff' : 'none'}
          className={styles.ringDot}
          onClick={() => onPick(i)}
        >
          <title>
            {f.angle.toFixed(0)}° · {f.kept ? 'kept' : f.reason}
          </title>
        </circle>
      );
    })}
    <text x={C} y={C + 4} textAnchor="middle" className={styles.ringLabel}>
      {track.channel.toUpperCase()}
    </text>
  </svg>
);

export default RingDiagram;
