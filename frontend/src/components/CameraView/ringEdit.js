// Editing a channel's ring and zones by hand on the camera picture (CameraView's
// onZonesChange). Everything is in the zones' own pixels and image degrees (0° right,
// clockwise on screen).

const RADII = ['inner_radius', 'outer_radius', 'exit_radius'];

/** Image angle of a point around the center, 0–360. */
export function angleAt(center, p) {
  const a = (Math.atan2(p.y - center.y, p.x - center.x) * 180) / Math.PI;
  return (a + 360) % 360;
}

const norm = (a) => ((Math.round(a) % 360) + 360) % 360;
const forward = (from, to) => (((to - from) % 360) + 360) % 360;
const within = (a, start, end) => forward(start, a) <= (forward(start, end) || 360);

/** The precise arc runs from exit.start − precise_width up to exit.start. */
function arcs(z) {
  const list = [
    {
      id: 'drop',
      start: z.drop.start,
      end: z.drop.end,
      r1: z.ring.inner_radius,
      r2: z.ring.outer_radius,
    },
    {
      id: 'exit',
      start: z.exit.start,
      end: z.exit.end,
      r1: z.ring.inner_radius,
      r2: z.ring.exit_radius,
    },
  ];
  if (z.precise_width > 0) {
    list.push({
      id: 'precise',
      start: z.exit.start - z.precise_width,
      end: z.exit.start,
      r1: z.ring.inner_radius,
      r2: z.ring.outer_radius,
    });
  }
  return list;
}

/** Where an arc's end handle sits: halfway across the ring at that angle. */
export function handles(z) {
  const c = z.ring.center;
  const at = (arc, deg, end) => {
    const r = (arc.r1 + arc.r2) / 2;
    const a = (deg * Math.PI) / 180;
    return { arc: arc.id, end, x: c.x + r * Math.cos(a), y: c.y + r * Math.sin(a) };
  };
  const out = [];
  for (const arc of arcs(z)) {
    if (arc.id === 'precise')
      out.push(at(arc, arc.start, 'start')); // its end is the exit's start
    else out.push(at(arc, arc.start, 'start'), at(arc, arc.end, 'end'));
  }
  return out;
}

/**
 * What a pointer at p (zone pixels) would grab; tol is a few screen pixels in zone
 * pixels. In order: an arc's end handle, a circle's edge, a section, else the ring.
 */
export function hitTest(z, p, tol) {
  const c = z.ring.center;
  const d = Math.hypot(p.x - c.x, p.y - c.y);
  for (const h of handles(z)) {
    if (Math.hypot(p.x - h.x, p.y - h.y) <= tol * 1.6)
      return { kind: 'end', arc: h.arc, end: h.end };
  }
  let edge = null;
  for (const key of RADII) {
    const gap = Math.abs(d - z.ring[key]);
    if (gap <= tol && (!edge || gap < edge.gap)) edge = { kind: 'radius', key, gap };
  }
  if (edge) return { kind: 'radius', key: edge.key };
  const a = angleAt(c, p);
  // Precise is drawn over the exit's neighbour, so it's checked first.
  for (const arc of [...arcs(z)].reverse()) {
    if (d >= arc.r1 && d <= arc.r2 && within(a, arc.start, arc.end)) {
      return { kind: 'rotate', arc: arc.id === 'precise' ? 'exit' : arc.id };
    }
  }
  return { kind: 'center' };
}

/** The circle nearest p, for the wheel. */
export function nearestRadius(z, p) {
  const d = Math.hypot(p.x - z.ring.center.x, p.y - z.ring.center.y);
  return RADII.reduce((best, key) =>
    Math.abs(d - z.ring[key]) < Math.abs(d - z.ring[best]) ? key : best,
  );
}

const clampRadius = (z, key, r) => {
  const v = Math.max(1, Math.round(r));
  if (key === 'inner_radius') return Math.min(v, z.ring.outer_radius - 2);
  if (key === 'outer_radius') return Math.max(v, z.ring.inner_radius + 2);
  return Math.max(v, z.ring.inner_radius + 2);
};

/** The zones after dragging grab from p0 (on zones z0) to p. */
export function dragTo(z0, grab, p0, p) {
  const z = structuredClone(z0);
  const c = z0.ring.center;
  switch (grab.kind) {
    case 'center':
      z.ring.center = { x: Math.round(c.x + p.x - p0.x), y: Math.round(c.y + p.y - p0.y) };
      return z;
    case 'radius':
      z.ring[grab.key] = clampRadius(z0, grab.key, Math.hypot(p.x - c.x, p.y - c.y));
      return z;
    case 'rotate': {
      const delta = angleAt(c, p) - angleAt(c, p0);
      z[grab.arc].start = norm(z0[grab.arc].start + delta);
      z[grab.arc].end = norm(z0[grab.arc].end + delta);
      return z;
    }
    case 'end': {
      const a = norm(angleAt(c, p));
      if (grab.arc === 'precise') {
        // Its start sets how wide it is, back from the exit's start.
        z.precise_width = Math.max(1, Math.min(180, forward(a, z0.exit.start) || 1));
      } else {
        z[grab.arc][grab.end] = a;
      }
      return z;
    }
    default:
      return z;
  }
}

/** The zones after a wheel step over p: deltaY < 0 grows. all scales every circle. */
export function wheel(z0, p, deltaY, all) {
  const z = structuredClone(z0);
  const factor = Math.exp(-deltaY * 0.001);
  const grow = (r) => {
    const next = r * factor;
    // Always move at least a pixel, so slow wheels and small circles still change.
    return Math.abs(next - r) < 1 ? r + Math.sign(-deltaY) : next;
  };
  if (all) {
    for (const key of RADII) z.ring[key] = Math.max(1, Math.round(grow(z0.ring[key])));
    if (z.ring.inner_radius >= z.ring.outer_radius) z.ring.inner_radius = z.ring.outer_radius - 2;
    return z;
  }
  const key = nearestRadius(z0, p);
  z.ring[key] = clampRadius(z0, key, grow(z0.ring[key]));
  return z;
}
