import styles from '../ChutePage.module.css';

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
 * One layer seen from above: its bins where the chute aims for them, the wedge before
 * home the chute can't reach, and the chute itself. Click a bin to send the chute there.
 */
const TowerMap = ({ bins, geometry, angle, disabled, onBin }) => {
  if (!geometry || bins.length === 0) {
    return <p className={styles.muted}>This layer&apos;s bin count doesn&apos;t split into the sections.</p>;
  }
  const perSection = bins.length / geometry.sections;
  const binWidth = geometry.section_width / perSection;
  const pointed = pointedBin(bins, angle, binWidth);

  return (
    <svg viewBox="0 0 300 300" className={styles.towerMap} role="img" aria-label="Bins on this layer, seen from above">
      <path d={arcPath(geometry.max_angle, 360, INNER - 6, OUTER + 6)} className={styles.deadWedge}>
        <title>{`No-go: the chute can't pass ${geometry.max_angle}°`}</title>
      </path>
      {bins.map((b) => {
        const cls = !b.reachable ? styles.binUnreachable : b.bin === pointed ? styles.binPointed : styles.bin;
        const [x, y] = point((OUTER + INNER) / 2, b.angle);
        const clickable = b.reachable && !disabled;
        return (
          <g
            key={b.bin}
            className={clickable ? styles.binGroup : undefined}
            onClick={clickable ? () => onBin(b.bin) : undefined}
          >
            <title>
              {b.reachable
                ? `Bin ${b.bin} (section ${b.section}) at ${b.angle.toFixed(1)}°`
                : `Bin ${b.bin} can't be reached`}
            </title>
            <path d={arcPath(b.angle - binWidth / 2 + 0.6, b.angle + binWidth / 2 - 0.6)} className={cls} />
            <text x={x} y={y} className={b.bin === pointed ? styles.binLabelPointed : styles.binLabel}>
              {b.bin}
            </text>
          </g>
        );
      })}
      <text x={CENTER} y={8} className={styles.homeLabel}>
        home
      </text>
      {angle != null && (
        <g style={{ transform: `rotate(${angle}deg)`, transformOrigin: `${CENTER}px ${CENTER}px`, transition: 'transform 300ms linear' }}>
          <line x1={CENTER} y1={CENTER} x2={CENTER} y2={CENTER - INNER + 8} className={styles.chute} />
          <path d={`M ${CENTER - 11} ${CENTER - INNER + 18} L ${CENTER} ${CENTER - INNER + 1} L ${CENTER + 11} ${CENTER - INNER + 18} Z`} className={styles.chuteHead} />
        </g>
      )}
      <circle cx={CENTER} cy={CENTER} r={20} className={styles.hub} />
    </svg>
  );
};

export default TowerMap;
