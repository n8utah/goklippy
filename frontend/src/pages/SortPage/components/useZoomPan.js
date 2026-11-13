import { useCallback, useEffect, useRef, useState } from 'react';

export const MAX_ZOOM = 8;
const STEP = 1.5;
const TAP_ZOOM = 3;

const clamp = (v, lo, hi) => Math.min(hi, Math.max(lo, v));

/**
 * Zoom and pan for a box that fills `stageRef`: the wheel or a pinch zooms about the
 * pointer, a drag pans once zoomed, a double-click (or double-tap) zooms to 3× there or
 * back out. The box always covers the stage: panning stops at its edges. Returns the
 * transform for the box and the handlers for the stage.
 */
export function useZoomPan(stageRef) {
  const [view, setView] = useState({ z: 1, x: 0, y: 0 });
  const viewRef = useRef(view); // the view as of now, for gestures
  const pointers = useRef(new Map()); // id → {x, y}, in stage pixels
  const gesture = useRef(null); // the pinch or drag under way
  const lastTap = useRef(0);

  // z, panned to (x, y), kept covering the stage.
  const fit = useCallback(
    (z, x, y) => {
      const r = stageRef.current?.getBoundingClientRect();
      const w = r?.width ?? 1;
      const h = r?.height ?? 1;
      const zz = clamp(z, 1, MAX_ZOOM);
      return { z: zz, x: clamp(x, w - w * zz, 0), y: clamp(y, h - h * zz, 0) };
    },
    [stageRef],
  );

  const show = useCallback((next) => {
    viewRef.current = next;
    setView(next);
  }, []);

  /** Zooms to z keeping the stage point (px, py) where it is. */
  const zoomAt = useCallback(
    (z, px, py) => {
      const v = viewRef.current;
      const k = clamp(z, 1, MAX_ZOOM) / v.z;
      show(fit(z, px - (px - v.x) * k, py - (py - v.y) * k));
    },
    [fit, show],
  );

  const atCenter = (z) => {
    const r = stageRef.current?.getBoundingClientRect();
    zoomAt(z, (r?.width ?? 0) / 2, (r?.height ?? 0) / 2);
  };
  const toggleAt = (px, py) =>
    viewRef.current.z > 1.01 ? show({ z: 1, x: 0, y: 0 }) : zoomAt(TAP_ZOOM, px, py);

  // The wheel needs a non-passive listener to keep the page from scrolling.
  useEffect(() => {
    const el = stageRef.current;
    if (!el) return undefined;
    const onWheel = (e) => {
      e.preventDefault();
      const r = el.getBoundingClientRect();
      zoomAt(
        viewRef.current.z * Math.exp(-e.deltaY * 0.0015),
        e.clientX - r.left,
        e.clientY - r.top,
      );
    };
    el.addEventListener('wheel', onWheel, { passive: false });
    return () => el.removeEventListener('wheel', onWheel);
  }, [stageRef, zoomAt]);

  // A new stage size (a turned phone, a resized window) keeps the view inside it.
  useEffect(() => {
    const onResize = () => {
      const v = viewRef.current;
      show(fit(v.z, v.x, v.y));
    };
    window.addEventListener('resize', onResize);
    return () => window.removeEventListener('resize', onResize);
  }, [fit, show]);

  const local = (e) => {
    const r = stageRef.current.getBoundingClientRect();
    return { x: e.clientX - r.left, y: e.clientY - r.top };
  };

  // Starts again from the pointers down now: one drags, two pinch.
  const restart = () => {
    const pts = [...pointers.current.values()];
    const v = viewRef.current;
    if (pts.length >= 2) {
      const [a, b] = pts;
      gesture.current = {
        kind: 'pinch',
        dist: Math.hypot(a.x - b.x, a.y - b.y) || 1,
        mid: { x: (a.x + b.x) / 2, y: (a.y + b.y) / 2 },
        v,
      };
    } else if (pts.length === 1) {
      gesture.current = { kind: 'drag', p: pts[0], v };
    } else {
      gesture.current = null;
    }
  };

  const onPointerDown = (e) => {
    if (e.pointerType === 'mouse' && e.button !== 0) return;
    e.currentTarget.setPointerCapture(e.pointerId);
    pointers.current.set(e.pointerId, local(e));
    restart();
  };

  const onPointerMove = (e) => {
    if (!pointers.current.has(e.pointerId)) return;
    pointers.current.set(e.pointerId, local(e));
    const g = gesture.current;
    if (g?.kind === 'drag') {
      const p = pointers.current.get(e.pointerId);
      show(fit(g.v.z, g.v.x + p.x - g.p.x, g.v.y + p.y - g.p.y));
    } else if (g?.kind === 'pinch') {
      const [a, b] = [...pointers.current.values()];
      const z = clamp(g.v.z * (Math.hypot(a.x - b.x, a.y - b.y) / g.dist), 1, MAX_ZOOM);
      const k = z / g.v.z;
      const mid = { x: (a.x + b.x) / 2, y: (a.y + b.y) / 2 };
      // What was under the fingers' first midpoint follows their midpoint.
      show(fit(z, mid.x - (g.mid.x - g.v.x) * k, mid.y - (g.mid.y - g.v.y) * k));
    }
  };

  const onPointerUp = (e) => {
    const p = pointers.current.get(e.pointerId);
    if (!p) return;
    const g = gesture.current;
    pointers.current.delete(e.pointerId);
    // Touch has no double-click: a second quick tap in place toggles the zoom.
    if (e.pointerType !== 'mouse' && g?.kind === 'drag' && pointers.current.size === 0) {
      const now = performance.now();
      if (Math.hypot(p.x - g.p.x, p.y - g.p.y) < 10) {
        if (now - lastTap.current < 300) {
          lastTap.current = 0;
          toggleAt(p.x, p.y);
        } else {
          lastTap.current = now;
        }
      }
    }
    restart();
  };

  return {
    zoom: view.z,
    transform: `translate(${view.x}px, ${view.y}px) scale(${view.z})`,
    zoomIn: () => atCenter(viewRef.current.z * STEP),
    zoomOut: () => atCenter(viewRef.current.z / STEP),
    reset: () => show({ z: 1, x: 0, y: 0 }),
    handlers: {
      onPointerDown,
      onPointerMove,
      onPointerUp,
      onPointerCancel: onPointerUp,
      onDoubleClick: (e) => {
        const { x, y } = local(e);
        toggleAt(x, y);
      },
    },
  };
}
