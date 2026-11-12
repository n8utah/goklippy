import { useEffect, useRef, useState } from 'react';

import { getSorterCameraObservation, sorterCameraStreamURL } from '@/helpers/api';
import { cn } from '@/lib/utils';

import styles from './CameraView.module.css';
import { dragTo, handles, hitTest, wheel as wheelZones } from './ringEdit';

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

const CURSORS = { center: 'move', radius: 'ew-resize', rotate: 'grab', end: 'col-resize' };

/**
 * A channel's live camera with its zones and what perception sees drawn over it.
 * zonesOverride shows zones being edited instead of the saved ones. With
 * onZonesChange they're edited by hand (see ringEdit.js): drag inside the ring to move
 * it, a circle's edge to size it, a section to slide it round, a section's end handle
 * to stretch it; the wheel sizes the nearest circle (Shift: all of them); double-click
 * puts the center there.
 */
const CameraView = ({ channel, zonesOverride, showOverlay = true, onZonesChange, compact }) => {
  const [observation, setObservation] = useState(null);
  const [savedZones, setSavedZones] = useState(null);
  const [failed, setFailed] = useState(false);
  const [size, setSize] = useState(null); // the stream's real frame size
  const [hover, setHover] = useState(null); // what the pointer would grab
  const [drag, setDrag] = useState(null); // { grab, p0, z0 }
  const viewRef = useRef(null);
  const imgRef = useRef(null);

  // A live stream fires load once, so a size changed mid-stream (a new camera size)
  // is caught by looking again every second.
  useEffect(() => {
    const timer = setInterval(() => {
      const img = imgRef.current;
      if (!img?.naturalWidth) return;
      setSize((cur) =>
        cur?.w === img.naturalWidth && cur?.h === img.naturalHeight
          ? cur
          : { w: img.naturalWidth, h: img.naturalHeight },
      );
    }, 1000);
    return () => clearInterval(timer);
  }, []);

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

  const editing = Boolean(onZonesChange && zones?.ring && showOverlay);

  // A pointer's place in the zones' pixels, and a few screen pixels in those units.
  const toZone = (e) => {
    const rect = viewRef.current.getBoundingClientRect();
    return {
      p: {
        x: (((e.clientX - rect.left) / rect.width) * w) / sx,
        y: (((e.clientY - rect.top) / rect.height) * h) / sy,
      },
      tol: ((10 / rect.width) * w) / s,
    };
  };

  const onPointerDown = (e) => {
    if (!editing || e.button !== 0) return;
    const { p, tol } = toZone(e);
    e.currentTarget.setPointerCapture(e.pointerId);
    setDrag({ grab: hitTest(zones, p, tol), p0: p, z0: zones });
  };
  const onPointerMove = (e) => {
    if (!editing) return;
    const { p, tol } = toZone(e);
    if (drag) onZonesChange(dragTo(drag.z0, drag.grab, drag.p0, p));
    else setHover(hitTest(zones, p, tol));
  };
  const onPointerUp = () => setDrag(null);
  const onDoubleClick = (e) => {
    if (!editing) return;
    const { p } = toZone(e);
    onZonesChange({
      ...zones,
      ring: { ...zones.ring, center: { x: Math.round(p.x), y: Math.round(p.y) } },
    });
  };

  // The wheel needs a non-passive listener to keep the page from scrolling.
  const wheelRef = useRef(null);
  useEffect(() => {
    wheelRef.current = editing
      ? (e) => {
          e.preventDefault();
          const { p } = toZone(e);
          onZonesChange(wheelZones(zones, p, e.deltaY, e.shiftKey));
        }
      : null;
  });
  useEffect(() => {
    const el = viewRef.current;
    if (!el) return undefined;
    const handler = (e) => wheelRef.current?.(e);
    el.addEventListener('wheel', handler, { passive: false });
    return () => el.removeEventListener('wheel', handler);
  }, [failed]);

  const active = drag?.grab ?? hover;
  const lit = (kind, key) =>
    editing &&
    active?.kind === kind &&
    (key === undefined || active.key === key || active.arc === key);

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
    const precise =
      zones.precise_width > 0
        ? { start: zones.exit.start - zones.precise_width, end: zones.exit.start }
        : null;
    const ring = (key) => cn(styles.ring, lit('radius', key) && styles.lit);
    const arc = (cls, key) => cn(cls, lit('rotate', key) && styles.lit);
    overlay = (
      <>
        <circle cx={cx} cy={cy} r={r1} className={ring('inner_radius')} />
        <circle cx={cx} cy={cy} r={r2} className={ring('outer_radius')} />
        {editing && (
          <circle cx={cx} cy={cy} r={re} className={cn(ring('exit_radius'), styles.exitRing)} />
        )}
        <path
          d={arcPath(cx, cy, r1, r2, zones.drop.start, zones.drop.end)}
          className={arc(styles.drop, 'drop')}
        />
        {precise && (
          <path
            d={arcPath(cx, cy, r1, r2, precise.start, precise.end)}
            className={arc(styles.precise, 'exit')}
          />
        )}
        <path
          d={arcPath(cx, cy, r1, re, zones.exit.start, zones.exit.end)}
          className={arc(styles.exit, 'exit')}
        />
        <circle cx={cx} cy={cy} r={lit('center') ? 7 : 4} className={styles.center} />
        {editing &&
          handles(zones).map((hd) => (
            <circle
              key={`${hd.arc}-${hd.end}`}
              cx={hd.x * sx}
              cy={hd.y * sy}
              r={
                (active?.kind === 'end' && active.arc === hd.arc && active.end === hd.end ? 9 : 6) *
                Math.max(1, w / 640)
              }
              className={cn(styles.handle, styles[`handle_${hd.arc}`])}
            />
          ))}
      </>
    );
  }

  return (
    <div
      ref={viewRef}
      className={cn(compact ? styles.viewCompact : styles.view, editing && styles.editing)}
      style={{
        // The frame's own shape, so the picture and its overlay aren't stretched.
        aspectRatio: `${w} / ${h}`,
        cursor: editing
          ? drag?.grab.kind === 'rotate'
            ? 'grabbing'
            : CURSORS[active?.kind]
          : undefined,
      }}
      onPointerDown={onPointerDown}
      onPointerMove={onPointerMove}
      onPointerUp={onPointerUp}
      onPointerCancel={onPointerUp}
      onPointerLeave={() => !drag && setHover(null)}
      onDoubleClick={onDoubleClick}
    >
      <img
        ref={imgRef}
        src={sorterCameraStreamURL(channel)}
        alt={`Camera over ${channel}`}
        className={styles.stream}
        onLoad={(e) =>
          setSize({ w: e.currentTarget.naturalWidth, h: e.currentTarget.naturalHeight })
        }
        onError={() => setFailed(true)}
      />
      {showOverlay && (
        <svg viewBox={`0 0 ${w} ${h}`} className={styles.overlay} preserveAspectRatio="none">
          {overlay}
          {/* Stuck pieces the sort is working around: left out of what it sees. */}
          {observation?.ignored?.map((b, i) => (
            <g key={`ignored-${i}`}>
              <rect
                x={b.x1}
                y={b.y1}
                width={b.x2 - b.x1}
                height={b.y2 - b.y1}
                className={styles.ignored}
              />
              <text x={b.x1} y={b.y1 - 4} className={styles.ignoredLabel}>
                left out
              </text>
            </g>
          ))}
          {observation?.pieces?.map((p, i) => (
            <g key={i}>
              <rect
                x={p.box.x1}
                y={p.box.y1}
                width={p.box.x2 - p.box.x1}
                height={p.box.y2 - p.box.y1}
                className={styles.box}
              />
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
