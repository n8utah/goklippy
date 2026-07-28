const SIZE_BY_TYPE = {
  stepper: { width: 54, height: 20 },
  endstop: { width: 22, height: 62 },
  thermistor: { width: 28, height: 54 },
  heater: { width: 67, height: 72 },
  fan: { width: 67, height: 72 },
  servo: { width: 72, height: 38 },
};

export function acceptsForSlotType(type) {
  switch (type) {
    case 'stepper':
      return ['stepper'];
    case 'endstop':
      return ['endstop'];
    case 'thermistor':
      return ['thermistor'];
    case 'heater':
      return ['heater', 'fan', 'relay'];
    case 'fan':
      return ['fan', 'heater', 'relay'];
    case 'servo':
      return ['servo'];
    default:
      return [type].filter(Boolean);
  }
}

function parseViewBox(diagram) {
  const viewBox = diagram?.viewBox || `0 0 ${diagram?.width || 858} ${diagram?.height || 498}`;
  const parts = viewBox.split(/\s+/).map(Number);
  return { width: parts[2] || 858, height: parts[3] || 498 };
}

function uniqueHotspotId(slotId, existingHotspots) {
  let base = `hs_${slotId}`;
  let candidate = base;
  let n = 2;
  const ids = new Set(existingHotspots.map((h) => h.id));
  while (ids.has(candidate)) {
    candidate = `${base}_${n}`;
    n += 1;
  }
  return candidate;
}

export function defaultHotspotForSlot(slotId, slot, diagram, existingHotspots) {
  const { width: canvasW, height: canvasH } = parseViewBox(diagram);
  const size = SIZE_BY_TYPE[slot.type] || { width: 48, height: 32 };
  const index = existingHotspots.length;

  return {
    id: uniqueHotspotId(slotId, existingHotspots),
    slot_ids: [slotId],
    label: slot.label || slotId,
    shape: 'rect',
    accepts: acceptsForSlotType(slot.type),
    x: Math.round(canvasW * 0.35 + (index % 4) * 24),
    y: Math.round(canvasH * 0.25 + Math.floor(index / 4) * 28),
    width: size.width,
    height: size.height,
  };
}

export function clientPointToSvg(svg, clientX, clientY) {
  if (!svg) {
    return { x: 0, y: 0 };
  }
  const pt = svg.createSVGPoint();
  pt.x = clientX;
  pt.y = clientY;
  const matrix = svg.getScreenCTM();
  if (!matrix) {
    return { x: 0, y: 0 };
  }
  const svgPt = pt.matrixTransform(matrix.inverse());
  return { x: svgPt.x, y: svgPt.y };
}

export const MIN_HOTSPOT_SIZE = 8;

export function resizeRect(handle, start, pointerX, pointerY) {
  const { x, y, width, height } = start;
  let nx = x;
  let ny = y;
  let nw = width;
  let nh = height;

  switch (handle) {
    case 'se':
      nw = Math.max(MIN_HOTSPOT_SIZE, pointerX - x);
      nh = Math.max(MIN_HOTSPOT_SIZE, pointerY - y);
      break;
    case 'sw':
      nw = Math.max(MIN_HOTSPOT_SIZE, x + width - pointerX);
      nh = Math.max(MIN_HOTSPOT_SIZE, pointerY - y);
      nx = x + width - nw;
      break;
    case 'ne':
      nw = Math.max(MIN_HOTSPOT_SIZE, pointerX - x);
      nh = Math.max(MIN_HOTSPOT_SIZE, y + height - pointerY);
      ny = y + height - nh;
      break;
    case 'nw':
      nw = Math.max(MIN_HOTSPOT_SIZE, x + width - pointerX);
      nh = Math.max(MIN_HOTSPOT_SIZE, y + height - pointerY);
      nx = x + width - nw;
      ny = y + height - nh;
      break;
    default:
      break;
  }

  return {
    x: Math.max(0, nx),
    y: Math.max(0, ny),
    width: nw,
    height: nh,
  };
}

export function resizeCircle(start, pointerX, pointerY) {
  const dx = pointerX - start.cx;
  const dy = pointerY - start.cy;
  return {
    cx: start.cx,
    cy: start.cy,
    r: Math.max(MIN_HOTSPOT_SIZE / 2, Math.hypot(dx, dy)),
  };
}
