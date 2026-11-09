import { useState } from 'react';

import { stepsToRpm } from '../SorterPage.logic';

import styles from '../SorterPage.module.css';

const WIDTH = 240;
const HEIGHT = 48;
const PAD = 3;
const SAMPLES = 60;
const SECONDS_PER_SAMPLE = 0.2;

/** Rolling rotor speed over the last 12 seconds, with a hover readout. */
const SpeedTrace = ({ values, stepsPerRev }) => {
  const [hover, setHover] = useState(null);
  const rpms = values.map((v) => stepsToRpm(v, stepsPerRev));
  const peak = Math.max(...rpms, 0);
  const top = Math.max(peak, 0.1);
  const x = (i) => PAD + ((WIDTH - PAD * 2) * (i + SAMPLES - rpms.length)) / (SAMPLES - 1);
  const y = (rpm) => HEIGHT - PAD - ((HEIGHT - PAD * 2) * rpm) / top;
  const points = rpms.map((rpm, i) => `${x(i)},${y(rpm)}`).join(' ');

  const onMove = (e) => {
    if (rpms.length === 0) return;
    const box = e.currentTarget.getBoundingClientRect();
    const fromRight = Math.round((1 - (e.clientX - box.left) / box.width) * (SAMPLES - 1));
    const index = rpms.length - 1 - fromRight;
    setHover(index >= 0 ? index : null);
  };

  const hovered = hover != null && hover < rpms.length ? hover : null;

  return (
    <div className={styles.trace}>
      <div className={styles.traceHeader}>
        <span>Speed, last 12 s</span>
        <span>
          {hovered == null
            ? `peak ${peak.toFixed(1)} rpm`
            : `${rpms[hovered].toFixed(2)} rpm, ${((rpms.length - 1 - hovered) * SECONDS_PER_SAMPLE).toFixed(1)} s ago`}
        </span>
      </div>
      <svg
        viewBox={`0 0 ${WIDTH} ${HEIGHT}`}
        className={styles.traceSvg}
        preserveAspectRatio="none"
        role="img"
        aria-label={`Rotor speed trace, latest ${rpms.length ? rpms[rpms.length - 1].toFixed(1) : 0} rpm`}
        onMouseMove={onMove}
        onMouseLeave={() => setHover(null)}
      >
        <line x1={PAD} y1={HEIGHT - PAD} x2={WIDTH - PAD} y2={HEIGHT - PAD} className={styles.traceBase} />
        {rpms.length > 1 && <polyline points={points} className={styles.traceLine} vectorEffect="non-scaling-stroke" />}
        {hovered != null && (
          <line x1={x(hovered)} y1={PAD} x2={x(hovered)} y2={HEIGHT - PAD} className={styles.traceCursor} vectorEffect="non-scaling-stroke" />
        )}
      </svg>
    </div>
  );
};

export default SpeedTrace;
