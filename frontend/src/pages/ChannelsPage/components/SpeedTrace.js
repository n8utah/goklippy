import { useState } from 'react';

import { stepsToRpm } from '@/helpers/useMachineMotors';

const WIDTH = 240;
const HEIGHT = 48;
const PAD = 3;
const SAMPLES = 60;
const SECONDS_PER_SAMPLE = 0.2;

/** The rotor's speed over the last 12 seconds, with a readout where the pointer is. */
export default function SpeedTrace({ values, stepsPerRev }) {
  const [hover, setHover] = useState(null);
  const rpms = values.map((v) => stepsToRpm(v, stepsPerRev));
  const peak = Math.max(...rpms, 0);
  const top = Math.max(peak, 0.1);
  const x = (i) => PAD + ((WIDTH - PAD * 2) * (i + SAMPLES - rpms.length)) / (SAMPLES - 1);
  const y = (rpm) => HEIGHT - PAD - ((HEIGHT - PAD * 2) * rpm) / top;
  const points = rpms.map((rpm, i) => `${x(i)},${y(rpm)}`).join(' ');
  const hovered = hover != null && hover < rpms.length ? hover : null;

  const onMove = (e) => {
    if (rpms.length === 0) return;
    const box = e.currentTarget.getBoundingClientRect();
    const fromRight = Math.round((1 - (e.clientX - box.left) / box.width) * (SAMPLES - 1));
    const index = rpms.length - 1 - fromRight;
    setHover(index >= 0 ? index : null);
  };

  return (
    <div className="flex flex-col gap-1">
      <div className="flex justify-between text-xs text-muted-foreground">
        <span>Speed, last 12 s</span>
        <span className="font-mono">
          {hovered == null
            ? `peak ${peak.toFixed(1)} rpm`
            : `${rpms[hovered].toFixed(2)} rpm, ${((rpms.length - 1 - hovered) * SECONDS_PER_SAMPLE).toFixed(1)} s ago`}
        </span>
      </div>
      <svg
        viewBox={`0 0 ${WIDTH} ${HEIGHT}`}
        preserveAspectRatio="none"
        className="h-14 w-full rounded-md bg-muted"
        role="img"
        aria-label={`Rotor speed, latest ${rpms.length ? rpms[rpms.length - 1].toFixed(1) : 0} rpm`}
        onMouseMove={onMove}
        onMouseLeave={() => setHover(null)}
      >
        <line
          x1={PAD}
          y1={HEIGHT - PAD}
          x2={WIDTH - PAD}
          y2={HEIGHT - PAD}
          className="stroke-border"
        />
        {rpms.length > 1 && (
          <polyline
            points={points}
            fill="none"
            strokeWidth={2}
            className="stroke-primary-text"
            vectorEffect="non-scaling-stroke"
          />
        )}
        {hovered != null && (
          <line
            x1={x(hovered)}
            y1={PAD}
            x2={x(hovered)}
            y2={HEIGHT - PAD}
            className="stroke-muted-foreground"
            vectorEffect="non-scaling-stroke"
          />
        )}
      </svg>
    </div>
  );
}
