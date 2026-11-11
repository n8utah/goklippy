import { sorterJourneyFileURL } from '../../../helpers/api';

import styles from '../JourneysPage.module.css';

/** The whole frame at a moment, with every piece's box; this piece's in green. */
const Moment = ({ journeyId, moment, frameW, frameH }) => (
  <figure className={styles.moment}>
    <div className={styles.momentPicture}>
      <img src={sorterJourneyFileURL(journeyId, moment.image)} alt={moment.label} />
      <svg
        viewBox={`0 0 ${frameW} ${frameH}`}
        preserveAspectRatio="none"
        className={styles.momentBoxes}
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
    <figcaption className={styles.hint}>
      {moment.label} · {new Date(moment.at).toLocaleTimeString()}
    </figcaption>
  </figure>
);

export default Moment;
