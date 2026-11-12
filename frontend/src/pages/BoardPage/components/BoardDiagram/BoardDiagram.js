import { useState } from 'react';

import ramps14Image from '@/assets/boards/ramps14.png';
import { resolveBoardImageUrl } from '@/helpers/api';
import { cn } from '@/lib/utils';

const BOARD_IMAGE_FALLBACKS = { ramps14: ramps14Image };

function formatReading(reading) {
  if (!reading) return null;
  if (reading.kind === 'analog_in') return Number(reading.value).toFixed(2);
  if (reading.kind === 'endstop') return reading.value ? '!' : 'ok';
  if (reading.kind === 'stepper') return `${reading.value} st`;
  return String(reading.value ?? '');
}

/**
 * The board's photo with a target on each pin or connector (Board › Wiring). A pin is a
 * button: click or Enter picks it. The colors sit on the photo, so they're the same in
 * both themes; the picked pin takes the accent.
 */
export default function BoardDiagram({
  diagram,
  profileId,
  slotStates,
  selectedHotspot,
  onSelectHotspot,
  readingForSlot,
}) {
  const [imageFailed, setImageFailed] = useState(false);

  if (!diagram) {
    return (
      <div className="flex aspect-[4/3] items-center justify-center rounded-xl border border-dashed border-border text-sm text-muted-foreground">
        Loading the board…
      </div>
    );
  }

  const viewBox = diagram.viewBox || `0 0 ${diagram.width || 800} ${diagram.height || 480}`;
  const [, , vbW, vbH] = viewBox.split(/\s+/).map(Number);
  const imageSrc =
    (!imageFailed && resolveBoardImageUrl(diagram)) || BOARD_IMAGE_FALLBACKS[profileId] || null;

  return (
    <div className="flex flex-col gap-2">
      <svg
        className="block h-auto w-full rounded-xl border border-border bg-[#0b1220]"
        viewBox={viewBox}
        role="group"
        aria-label="The board: pick a pin to say what's wired to it"
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
          const connected = Boolean(slotId && slotStates[slotId]?.connected);
          const selected = selectedHotspot?.id === hotspot.id;
          const readingText = formatReading(
            readingForSlot && slotId ? readingForSlot(slotId) : null,
          );
          const circle = hotspot.shape === 'circle';
          const cx = circle ? hotspot.cx : (hotspot.x || 0) + (hotspot.width || 0) / 2;
          const cy = circle ? hotspot.cy : (hotspot.y || 0) + (hotspot.height || 0) / 2;
          const className = cn(
            'cursor-pointer outline-none transition-[fill,stroke] duration-150',
            selected
              ? 'fill-[#c2410c]/40 stroke-[#fb923c] [stroke-width:3]'
              : connected
                ? 'fill-[#22c55e]/25 stroke-[#22c55e] [stroke-width:2] hover:fill-[#22c55e]/40'
                : 'fill-[#94a3b8]/15 stroke-[#94a3b8]/80 [stroke-width:2] [stroke-dasharray:4_3] hover:fill-[#fb923c]/25 hover:stroke-[#fb923c]',
            readingText && !selected && 'stroke-[#f59e0b]',
            'focus-visible:stroke-white focus-visible:[stroke-width:3]',
          );
          const pick = () => onSelectHotspot(hotspot);
          const common = {
            className,
            role: 'button',
            tabIndex: 0,
            'aria-label': `${hotspot.label}${connected ? ', wired' : ''}`,
            'aria-pressed': selected,
            onClick: pick,
            onKeyDown: (e) => {
              if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault();
                pick();
              }
            },
          };
          return (
            <g key={hotspot.id}>
              {circle ? (
                <circle {...common} cx={hotspot.cx} cy={hotspot.cy} r={hotspot.r}>
                  <title>{hotspot.label}</title>
                </circle>
              ) : (
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
              )}
              {readingText && (
                <text
                  x={cx}
                  y={cy}
                  textAnchor="middle"
                  dominantBaseline="middle"
                  className="pointer-events-none fill-[#fef3c7] font-mono text-[11px] font-bold [text-shadow:0_1px_2px_rgb(0_0_0/0.8)]"
                >
                  {readingText}
                </text>
              )}
            </g>
          );
        })}
      </svg>
      <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-sm text-muted-foreground">
        <span className="flex items-center gap-1.5">
          <span className="size-2.5 rounded-full bg-[#22c55e]" aria-hidden /> Wired
        </span>
        <span className="flex items-center gap-1.5">
          <span
            className="size-2.5 rounded-full border border-dashed border-[#94a3b8]"
            aria-hidden
          />{' '}
          Not set up
        </span>
        <span className="flex items-center gap-1.5">
          <span className="size-2.5 rounded-full bg-primary" aria-hidden /> Picked
        </span>
        {readingForSlot && <span className="sm:ml-auto">Live values show on wired pins.</span>}
      </div>
    </div>
  );
}
