import { useRef } from 'react';

import { resolveBoardImageUrl } from '../../../../helpers/api';
import { clientPointToSvg } from '../../../../helpers/boardHotspots';
import ramps14Image from '../../../../assets/boards/ramps14.png';

import styles from './EditableBoardDiagram.module.css';

const BOARD_IMAGE_FALLBACKS = {
  ramps14: ramps14Image,
};

const HANDLE_SIZE = 8;

const CORNER_HANDLES = [
  { handle: 'nw', cursor: 'nwse-resize' },
  { handle: 'ne', cursor: 'nesw-resize' },
  { handle: 'sw', cursor: 'nesw-resize' },
  { handle: 'se', cursor: 'nwse-resize' },
];

function cornerPoint(hotspot, handle) {
  const { x, y, width, height } = hotspot;
  switch (handle) {
    case 'nw':
      return { x, y };
    case 'ne':
      return { x: x + width, y };
    case 'sw':
      return { x, y: y + height };
    case 'se':
    default:
      return { x: x + width, y: y + height };
  }
}

const ResizeHandles = ({ hotspot, onResizeStart }) =>
  CORNER_HANDLES.map(({ handle, cursor }) => {
    const point = cornerPoint(hotspot, handle);
    return (
      <rect
        key={handle}
        className={styles.resizeHandle}
        x={point.x - HANDLE_SIZE / 2}
        y={point.y - HANDLE_SIZE / 2}
        width={HANDLE_SIZE}
        height={HANDLE_SIZE}
        style={{ cursor }}
        onPointerDown={(event) => onResizeStart(event, hotspot, handle)}
      />
    );
  });

const CircleResizeHandle = ({ hotspot, onResizeStart }) => (
  <rect
    className={styles.resizeHandle}
    x={hotspot.cx + hotspot.r - HANDLE_SIZE / 2}
    y={hotspot.cy - HANDLE_SIZE / 2}
    width={HANDLE_SIZE}
    height={HANDLE_SIZE}
    style={{ cursor: 'ew-resize' }}
    onPointerDown={(event) => onResizeStart(event, hotspot, 'se')}
  />
);

const EditableBoardDiagram = ({
  diagram,
  profileId,
  hotspots,
  selectedId,
  onSelect,
  onBeginDrag,
  onBeginResize,
  onMoveDrag,
  onEndDrag,
}) => {
  const svgRef = useRef(null);

  if (!diagram) {
    return <div className={styles.placeholder}>Loading board image…</div>;
  }

  const viewBox = diagram.viewBox || `0 0 ${diagram.width || 800} ${diagram.height || 480}`;
  const imageSrc = resolveBoardImageUrl(diagram) || BOARD_IMAGE_FALLBACKS[profileId] || null;

  const capturePointer = (event) => {
    if (event.currentTarget.setPointerCapture) {
      event.currentTarget.setPointerCapture(event.pointerId);
    }
  };

  const releasePointer = (event) => {
    if (event.currentTarget.hasPointerCapture?.(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
  };

  const handleMoveStart = (event, hotspot) => {
    event.stopPropagation();
    event.preventDefault();
    const svg = svgRef.current;
    const pt = clientPointToSvg(svg, event.clientX, event.clientY);
    onBeginDrag(hotspot.id, pt.x - hotspot.x, pt.y - hotspot.y);
    capturePointer(event);
  };

  const handleCircleMoveStart = (event, hotspot) => {
    event.stopPropagation();
    event.preventDefault();
    const svg = svgRef.current;
    const pt = clientPointToSvg(svg, event.clientX, event.clientY);
    onBeginDrag(hotspot.id, pt.x - hotspot.cx, pt.y - hotspot.cy, 'circle');
    capturePointer(event);
  };

  const handleResizeStart = (event, hotspot, handle) => {
    event.stopPropagation();
    event.preventDefault();
    onBeginResize(hotspot.id, handle, hotspot);
    capturePointer(event);
  };

  const handlePointerMove = (event) => {
    if (event.buttons !== 1) {
      return;
    }
    const svg = svgRef.current;
    const pt = clientPointToSvg(svg, event.clientX, event.clientY);
    onMoveDrag(pt.x, pt.y);
  };

  const handlePointerUp = (event) => {
    onEndDrag();
    releasePointer(event);
  };

  return (
    <svg
      ref={svgRef}
      className={styles.diagramSvg}
      viewBox={viewBox}
      onPointerMove={handlePointerMove}
      onPointerUp={handlePointerUp}
      onPointerLeave={onEndDrag}
    >
      {imageSrc ? <image href={imageSrc} x="0" y="0" width={diagram.width} height={diagram.height} /> : null}
      {hotspots.map((hotspot) => {
        const selected = hotspot.id === selectedId;
        const className = [styles.hotspot, selected ? styles.hotspotSelected : ''].join(' ');

        if (hotspot.shape === 'circle') {
          return (
            <g key={hotspot.id}>
              <circle
                className={className}
                cx={hotspot.cx}
                cy={hotspot.cy}
                r={hotspot.r}
                onPointerDown={(event) => handleCircleMoveStart(event, hotspot)}
                onClick={() => onSelect(hotspot.id)}
              >
                <title>{hotspot.label}</title>
              </circle>
              {selected && (
                <CircleResizeHandle hotspot={hotspot} onResizeStart={handleResizeStart} />
              )}
            </g>
          );
        }

        return (
          <g key={hotspot.id}>
            <rect
              className={className}
              x={hotspot.x}
              y={hotspot.y}
              width={hotspot.width}
              height={hotspot.height}
              rx={4}
              onPointerDown={(event) => handleMoveStart(event, hotspot)}
              onClick={() => onSelect(hotspot.id)}
            >
              <title>{hotspot.label}</title>
            </rect>
            {selected && <ResizeHandles hotspot={hotspot} onResizeStart={handleResizeStart} />}
          </g>
        );
      })}
    </svg>
  );
};

export default EditableBoardDiagram;
