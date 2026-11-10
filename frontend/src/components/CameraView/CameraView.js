import { useEffect, useState } from 'react';

import { getSorterCameraObservation, sorterCameraStreamURL } from '../../helpers/api';

import styles from './CameraView.module.css';

const OVERLAY_POLL_MS = 250;

/** A point on the ring: angles are image angles (0° right, clockwise on screen). */
function polar(cx, cy, r, deg) {
  const a = (deg * Math.PI) / 180;
  return [cx + r * Math.cos(a), cy + r * Math.sin(a)];
}

function forward(from, to) {
  return (((to - from) % 360) + 360) % 360;
}

/** An annular sector from start forward to end. */
function arcPath(cx, cy, r1, r2, start, end) {
  const width = forward(start, end) || 360;
  const stop = start + width;
  const large = width > 180 ? 1 : 0;
  const [ax, ay] = polar(cx, cy, r2, start);
  const [bx, by] = polar(cx, cy, r2, stop);
  const [ix, iy] = polar(cx, cy, r1, stop);
  const [jx, jy] = polar(cx, cy, r1, start);
  return `M ${ax} ${ay} A ${r2} ${r2} 0 ${large} 1 ${bx} ${by} L ${ix} ${iy} A ${r1} ${r1} 0 ${large} 0 ${jx} ${jy} Z`;
}

/**
 * A channel's live camera with its zones and what perception sees drawn over it.
 * zonesOverride shows zones being edited instead of the saved ones. onPick gets the
 * frame position of a click.
 */
const CameraView = ({ channel, zonesOverride, showOverlay = true, onPick, compact }) => {
  const [observation, setObservation] = useState(null);
  const [savedZones, setSavedZones] = useState(null);
  const [failed, setFailed] = useState(false);
  const [size, setSize] = useState(null); // the stream's real frame size

  useEffect(() => {
    if (!showOverlay) return undefined;
    let cancelled = false;
    const poll = () =>
      getSorterCameraObservation(channel).then(
        (data) => {
          if (cancelled) return;
          setObservation(data.observation);
          setSavedZones(data.zones ?? null);
        },
        () => {},
      );
    poll();
    const timer = setInterval(poll, OVERLAY_POLL_MS);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, [channel, showOverlay]);

  const zones = zonesOverride ?? savedZones;
  // Draw in the frame's pixels; zones saved for another frame size are scaled.
  const w = size?.w || zones?.frame_w || 640;
  const h = size?.h || zones?.frame_h || 480;
  const sx = zones?.frame_w ? w / zones.frame_w : 1;
  const sy = zones?.frame_h ? h / zones.frame_h : 1;
  const s = (sx + sy) / 2;

  const pick = (e) => {
    if (!onPick) return;
    const rect = e.currentTarget.getBoundingClientRect();
    onPick({
      x: Math.round(((e.clientX - rect.left) / rect.width) * w / sx),
      y: Math.round(((e.clientY - rect.top) / rect.height) * h / sy),
    });
  };

  if (failed) {
    return <div className={compact ? styles.emptyCompact : styles.empty}>No picture</div>;
  }

  let overlay = null;
  if (showOverlay && zones?.ring) {
    const cx = zones.ring.center.x * sx;
    const cy = zones.ring.center.y * sy;
    const r1 = zones.ring.inner_radius * s;
    const r2 = zones.ring.outer_radius * s;
    const re = zones.ring.exit_radius * s;
    const precise = zones.precise_width > 0 ? { start: zones.exit.start - zones.precise_width, end: zones.exit.start } : null;
    overlay = (
      <>
        <circle cx={cx} cy={cy} r={r1} className={styles.ring} />
        <circle cx={cx} cy={cy} r={r2} className={styles.ring} />
        <path d={arcPath(cx, cy, r1, r2, zones.drop.start, zones.drop.end)} className={styles.drop} />
        {precise && <path d={arcPath(cx, cy, r1, r2, precise.start, precise.end)} className={styles.precise} />}
        <path d={arcPath(cx, cy, r1, re, zones.exit.start, zones.exit.end)} className={styles.exit} />
        <circle cx={cx} cy={cy} r={4} className={styles.center} />
      </>
    );
  }

  return (
    <div className={compact ? styles.viewCompact : styles.view} onClick={pick} role={onPick ? 'button' : undefined}>
      <img
        src={sorterCameraStreamURL(channel)}
        alt={`Camera over ${channel}`}
        className={styles.stream}
        onLoad={(e) => setSize({ w: e.currentTarget.naturalWidth, h: e.currentTarget.naturalHeight })}
        onError={() => setFailed(true)}
      />
      {showOverlay && (
        <svg viewBox={`0 0 ${w} ${h}`} className={styles.overlay} preserveAspectRatio="none">
          {overlay}
          {observation?.pieces?.map((p, i) => (
            <g key={i}>
              <rect x={p.box.x1} y={p.box.y1} width={p.box.x2 - p.box.x1} height={p.box.y2 - p.box.y1} className={styles.box} />
              {p.track_id ? (
                <text x={p.box.x1} y={p.box.y1 - 4} className={styles.label}>
                  #{p.track_id}
                </text>
              ) : null}
            </g>
          ))}
        </svg>
      )}
    </div>
  );
};

export default CameraView;
