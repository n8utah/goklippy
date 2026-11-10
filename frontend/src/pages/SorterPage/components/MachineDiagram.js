import { CHANNEL_IDS } from '../SorterPage.logic';

import styles from '../SorterPage.module.css';

const CY = 130;
const RINGS = {
  c1: { cx: 105, radius: 62 },
  c2: { cx: 290, radius: 62 },
  c3: { cx: 475, radius: 62 },
  c4: { cx: 700, radius: 100 },
};
const GAP_DEGREES = 50; // opening of the C, facing the next stage

function polar(cx, radius, degrees) {
  const rad = (degrees * Math.PI) / 180;
  return [cx + radius * Math.cos(rad), CY + radius * Math.sin(rad)];
}

function statorPath(cx, radius) {
  const [sx, sy] = polar(cx, radius, GAP_DEGREES / 2);
  const [ex, ey] = polar(cx, radius, -GAP_DEGREES / 2);
  return `M ${sx} ${sy} A ${radius} ${radius} 0 1 1 ${ex} ${ey}`;
}

function facetPoints(cx, radius, facets) {
  return Array.from({ length: facets }, (_, i) => polar(cx, radius, (360 / facets) * i).join(',')).join(' ');
}

function directionLabel(direction) {
  if (direction > 0) return '↻';
  if (direction < 0) return '↺';
  return '■';
}

function FeederRotor({ cx, radius }) {
  return (
    <>
      <polygon points={facetPoints(cx, radius - 14, 8)} className={styles.rotor} />
      <line x1={cx} y1={CY} x2={cx + radius - 14} y2={CY} className={styles.rotorMark} />
    </>
  );
}

// Dividers sit between slots, so at angle 0 slot 1 is centered in the gap. The dot marks slot 1.
function CarouselRotor({ cx, radius, slots }) {
  const [dotX, dotY] = polar(cx, radius - 36, 0);
  return (
    <>
      <circle cx={cx} cy={CY} r={radius - 12} className={styles.rotor} />
      {Array.from({ length: slots }, (_, i) => {
        const [x, y] = polar(cx, radius - 12, (360 / slots) * (i + 0.5));
        return <line key={i} x1={cx} y1={CY} x2={x} y2={y} className={styles.rotorSpoke} />;
      })}
      <circle cx={dotX} cy={dotY} r={7} className={styles.rotorSlotDot} />
    </>
  );
}

function Stage({ channel, ring, slots }) {
  const { cx, radius } = ring;
  const moving = channel.direction !== 0;
  const missing = channel.oid == null;
  const isCarousel = channel.id === 'c4';
  const stateClass = missing ? styles.stageMissing : moving ? styles.stageMoving : styles.stageIdle;

  return (
    <g className={stateClass}>
      <path d={statorPath(cx, radius)} className={styles.stator} />
      <g
        style={{
          transform: `rotate(${channel.angle}deg)`,
          transformOrigin: `${cx}px ${CY}px`,
          transition: 'transform 220ms linear',
        }}
      >
        {isCarousel ? <CarouselRotor cx={cx} radius={radius} slots={slots} /> : <FeederRotor cx={cx} radius={radius} />}
      </g>
      <circle cx={cx} cy={CY} r={22} className={styles.hub} />
      <text x={cx} y={CY - 2} className={styles.hubName}>
        {channel.id.toUpperCase()}
      </text>
      <text x={cx} y={CY + 13} className={styles.hubDir}>
        {directionLabel(channel.direction)}
      </text>
      <text x={cx} y={CY + radius + 26} className={styles.stageRpm}>
        {missing ? 'no motor' : `${channel.measuredRpm.toFixed(1)} rpm`}
      </text>
    </g>
  );
}

function Drop({ from, to }) {
  const x1 = RINGS[from].cx + RINGS[from].radius + 6;
  const x2 = RINGS[to].cx - RINGS[to].radius - 8;
  return <path d={`M ${x1} ${CY} L ${x2} ${CY}`} className={styles.drop} markerEnd="url(#sorter-arrow)" />;
}

const MachineDiagram = ({ channels, indexDegrees }) => {
  const slots = Math.max(1, Math.round(360 / indexDegrees));
  return (
    <div className={styles.diagramWrap}>
      <svg viewBox="0 0 830 275" className={styles.diagram} role="img" aria-label="Top view of C1, C2, C3 feeder channels and the C4 carousel">
        <defs>
          <marker id="sorter-arrow" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse">
            <path d="M 0 0 L 10 5 L 0 10 z" className={styles.dropHead} />
          </marker>
        </defs>
        <Drop from="c1" to="c2" />
        <Drop from="c2" to="c3" />
        <Drop from="c3" to="c4" />
        {CHANNEL_IDS.map((id) => (
          <Stage key={id} channel={channels[id]} ring={RINGS[id]} slots={slots} />
        ))}
      </svg>
    </div>
  );
};

export default MachineDiagram;
