import { useState } from 'react';

import { resolveBoardImageUrl } from '../../../../helpers/api';
import ramps14Image from '../../../../assets/boards/ramps14.png';

import styles from './BoardDiagram.module.css';

const BOARD_IMAGE_FALLBACKS = {
  ramps14: ramps14Image,
};

function formatReading(reading) {
  if (!reading) {
    return null;
  }
  if (reading.kind === 'analog_in') {
    return Number(reading.value).toFixed(2);
  }
  if (reading.kind === 'endstop') {
    return reading.value ? '!' : 'ok';
  }
  if (reading.kind === 'stepper') {
    return `${reading.value} st`;
  }
  return String(reading.value ?? '');
}

const BoardDiagram = ({ diagram, profileId, slotStates, selectedHotspot, onSelectHotspot, readingForSlot }) => {
  const [imageFailed, setImageFailed] = useState(false);

  if (!diagram) {
    return <div className={styles.placeholder}>Loading board diagram…</div>;
  }

  const viewBox = diagram.viewBox || `0 0 ${diagram.width || 800} ${diagram.height || 480}`;
  const [, , vbW, vbH] = viewBox.split(/\s+/).map(Number);
  const imageSrc =
    (!imageFailed && resolveBoardImageUrl(diagram)) || BOARD_IMAGE_FALLBACKS[profileId] || null;

  return (
    <div className={styles.wrap}>
      <svg
        className={styles.diagramSvg}
        viewBox={viewBox}
        role="img"
        aria-label={`${profileId} board diagram`}
      >
        {imageSrc ? (
          <image
            href={imageSrc}
            x="0"
            y="0"
            width={vbW}
            height={vbH}
            // eslint-disable-next-line react/no-unknown-property -- React supports onError on SVG <image>
            onError={() => setImageFailed(true)}
          />
        ) : (
          <rect width={vbW} height={vbH} fill="#0b1220" />
        )}
        {(diagram.hotspots || []).map((hotspot) => {
          const slotId = hotspot.slot_ids?.[0];
          const connected = slotId && slotStates[slotId]?.connected;
          const selected = selectedHotspot?.id === hotspot.id;
          const reading = readingForSlot && slotId ? readingForSlot(slotId) : null;
          const readingText = formatReading(reading);
          const className = [
            styles.hotspot,
            connected ? styles.hotspotConnected : styles.hotspotEmpty,
            selected ? styles.hotspotSelected : '',
            readingText ? styles.hotspotLive : '',
          ].join(' ');

          const centerX =
            hotspot.shape === 'circle' ? hotspot.cx : (hotspot.x || 0) + (hotspot.width || 0) / 2;
          const centerY =
            hotspot.shape === 'circle' ? hotspot.cy : (hotspot.y || 0) + (hotspot.height || 0) / 2;

          if (hotspot.shape === 'circle') {
            return (
              <g key={hotspot.id}>
                <circle
                  className={className}
                  cx={hotspot.cx}
                  cy={hotspot.cy}
                  r={hotspot.r}
                  onClick={() => onSelectHotspot(hotspot)}
                >
                  <title>{hotspot.label}</title>
                </circle>
                {readingText && (
                  <text x={centerX} y={centerY} className={styles.readingLabel} textAnchor="middle" dominantBaseline="middle">
                    {readingText}
                  </text>
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
                onClick={() => onSelectHotspot(hotspot)}
              >
                <title>{hotspot.label}</title>
              </rect>
              {readingText && (
                <text x={centerX} y={centerY} className={styles.readingLabel} textAnchor="middle" dominantBaseline="middle">
                  {readingText}
                </text>
              )}
            </g>
          );
        })}
      </svg>
      <div className={styles.legend}>
        <span className={styles.legendDotConnected} /> Connected
        <span className={styles.legendDotEmpty} /> Not configured
        {readingForSlot && <span className={styles.legendLive}>Live values shown on connected pins</span>}
      </div>
    </div>
  );
};

export default BoardDiagram;
