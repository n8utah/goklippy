import { useRef } from 'react';

import ramps14Image from '@/assets/boards/ramps14.png';
import { resolveBoardImageUrl } from '@/helpers/api';
import { clientPointToSvg } from '@/helpers/boardHotspots';
import { cn } from '@/lib/utils';

const BOARD_IMAGE_FALLBACKS = { ramps14: ramps14Image };
const HANDLE_SIZE = 8;
const CORNER_HANDLES = [
  { handle: 'nw', cursor: 'nwse-resize' },
  { handle: 'ne', cursor: 'nesw-resize' },
  { handle: 'sw', cursor: 'nesw-resize' },
  { handle: 'se', cursor: 'nwse-resize' },
];
const HANDLE_CLASS = 'fill-white stroke-[#c2410c] [stroke-width:1.5] hover:fill-[#fed7aa]';

function cornerPoint({ x, y, width, height }, handle) {
  switch (handle) {
    case 'nw':
      return { x, y };
    case 'ne':
      return { x: x + width, y };
    case 'sw':
      return { x, y: y + height };
    default:
      return { x: x + width, y: y + height };
  }
}

/**
 * The board's photo with every overlay: drag one to move it, its corner handles to resize.
 * The colors sit on the photo, so they're the same in both themes.
 */
export default function EditableBoardDiagram({
  diagram,
  profileId,
  hotspots,
  selectedId,
  onSelect,
  onBeginDrag,
  onBeginResize,
  onMoveDrag,
  onEndDrag,
}) {
  const svgRef = useRef(null);

  if (!diagram) {
    return (
      <div className="flex aspect-[4/3] items-center justify-center rounded-xl border border-dashed border-border text-sm text-muted-foreground">
        Loading the board picture…
      </div>
    );
  }

  const viewBox = diagram.viewBox || `0 0 ${diagram.width || 800} ${diagram.height || 480}`;
  const imageSrc = resolveBoardImageUrl(diagram) || BOARD_IMAGE_FALLBACKS[profileId] || null;

  const capture = (e) => e.currentTarget.setPointerCapture?.(e.pointerId);
  const release = (e) => {
    if (e.currentTarget.hasPointerCapture?.(e.pointerId))
      e.currentTarget.releasePointerCapture(e.pointerId);
  };
  const point = (e) => clientPointToSvg(svgRef.current, e.clientX, e.clientY);

  const moveStart = (e, hotspot) => {
    e.stopPropagation();
    e.preventDefault();
    const pt = point(e);
    if (hotspot.shape === 'circle')
      onBeginDrag(hotspot.id, pt.x - hotspot.cx, pt.y - hotspot.cy, 'circle');
    else onBeginDrag(hotspot.id, pt.x - hotspot.x, pt.y - hotspot.y);
    capture(e);
  };
  const resizeStart = (e, hotspot, handle) => {
    e.stopPropagation();
    e.preventDefault();
    onBeginResize(hotspot.id, handle, hotspot);
    capture(e);
  };

  return (
    <svg
      ref={svgRef}
      className="block h-auto w-full cursor-crosshair touch-none rounded-xl border border-border bg-[#0b1220]"
      viewBox={viewBox}
      onPointerMove={(e) => e.buttons === 1 && onMoveDrag(point(e).x, point(e).y)}
      onPointerUp={(e) => {
        onEndDrag();
        release(e);
      }}
      onPointerLeave={onEndDrag}
      role="group"
      aria-label="The board picture's overlays"
    >
      {imageSrc && (
        <image href={imageSrc} x="0" y="0" width={diagram.width} height={diagram.height} />
      )}
      {hotspots.map((hotspot) => {
        const selected = hotspot.id === selectedId;
        const className = cn(
          'cursor-grab active:cursor-grabbing',
          selected
            ? 'fill-[#c2410c]/35 stroke-[#fb923c] [stroke-width:3]'
            : 'fill-[#38bdf8]/20 stroke-[#38bdf8] [stroke-width:2]',
        );
        const common = {
          className,
          onPointerDown: (e) => moveStart(e, hotspot),
          onClick: () => onSelect(hotspot.id),
        };
        if (hotspot.shape === 'circle') {
          return (
            <g key={hotspot.id}>
              <circle {...common} cx={hotspot.cx} cy={hotspot.cy} r={hotspot.r}>
                <title>{hotspot.label}</title>
              </circle>
              {selected && (
                <rect
                  className={HANDLE_CLASS}
                  x={hotspot.cx + hotspot.r - HANDLE_SIZE / 2}
                  y={hotspot.cy - HANDLE_SIZE / 2}
                  width={HANDLE_SIZE}
                  height={HANDLE_SIZE}
                  style={{ cursor: 'ew-resize' }}
                  onPointerDown={(e) => resizeStart(e, hotspot, 'se')}
                />
              )}
            </g>
          );
        }
        return (
          <g key={hotspot.id}>
            <rect
              {...common}
              x={hotspot.x}
              y={hotspot.y}
              width={hotspot.width}
              height={hotspot.height}
              rx={4}
            >
              <title>{hotspot.label}</title>
            </rect>
            {selected &&
              CORNER_HANDLES.map(({ handle, cursor }) => {
                const p = cornerPoint(hotspot, handle);
                return (
                  <rect
                    key={handle}
                    className={HANDLE_CLASS}
                    x={p.x - HANDLE_SIZE / 2}
                    y={p.y - HANDLE_SIZE / 2}
                    width={HANDLE_SIZE}
                    height={HANDLE_SIZE}
                    style={{ cursor }}
                    onPointerDown={(e) => resizeStart(e, hotspot, handle)}
                  />
                );
              })}
          </g>
        );
      })}
    </svg>
  );
}
