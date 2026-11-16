import { Maximize, Minus, Plus, X } from 'lucide-react';
import { useCallback, useEffect, useRef, useState } from 'react';

import CameraView from '@/components/CameraView/CameraView';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from '@/components/ui/dialog';

import { MAX_ZOOM, useZoomPan } from './useZoomPan';

/** The stage's size in pixels, kept up to date. */
function useSize(ref) {
  const [size, setSize] = useState(null);
  useEffect(() => {
    const el = ref.current;
    if (!el) return undefined;
    const ro = new ResizeObserver(([entry]) => {
      const { width, height } = entry.contentRect;
      setSize({ w: width, h: height });
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, [ref]);
  return size;
}

function Stage({ channel }) {
  const stageRef = useRef(null);
  const stage = useSize(stageRef);
  const [frame, setFrame] = useState(null);
  const onFrameSize = useCallback((s) => setFrame(s), []);
  const zp = useZoomPan(stageRef);

  // The picture as large as fits, in its own shape.
  const aspect = frame ? frame.w / frame.h : 4 / 3;
  const width = stage ? Math.min(stage.w, stage.h * aspect) : 0;

  return (
    <>
      <div
        ref={stageRef}
        className="relative min-h-0 flex-1 touch-none overflow-hidden select-none"
        style={{ cursor: zp.zoom > 1 ? 'grab' : 'zoom-in' }}
        {...zp.handlers}
      >
        <div
          className="absolute inset-0 flex items-center justify-center"
          style={{ transform: zp.transform, transformOrigin: '0 0' }}
        >
          {stage && (
            <div style={{ width }}>
              <CameraView channel={channel.id} onFrameSize={onFrameSize} />
            </div>
          )}
        </div>
      </div>

      <div className="flex flex-wrap items-center justify-center gap-2 px-4 py-3">
        <Button
          variant="secondary"
          size="icon"
          onClick={zp.zoomOut}
          disabled={zp.zoom <= 1}
          aria-label="Zoom out"
        >
          <Minus />
        </Button>
        <span className="w-14 text-center font-mono text-sm text-white">{zp.zoom.toFixed(1)}×</span>
        <Button
          variant="secondary"
          size="icon"
          onClick={zp.zoomIn}
          disabled={zp.zoom >= MAX_ZOOM}
          aria-label="Zoom in"
        >
          <Plus />
        </Button>
        <Button variant="secondary" onClick={zp.reset} disabled={zp.zoom <= 1}>
          <Maximize /> Fit
        </Button>
      </div>
    </>
  );
}

/** Every camera small and live, in order, the one seen big marked: another clicked is
 * seen big instead. */
function Thumbnails({ channels, current, onSwitch }) {
  return (
    <div className="flex justify-center gap-2 px-4 pb-3">
      {channels.map((ch) => (
        <button
          key={ch.id}
          type="button"
          onClick={() => ch.id !== current.id && onSwitch(ch.id)}
          aria-current={ch.id === current.id}
          className={cn(
            'relative w-28 overflow-hidden rounded-md sm:w-40',
            ch.id === current.id
              ? 'ring-3 ring-primary'
              : 'opacity-70 ring-1 ring-white/20 hover:opacity-100 hover:ring-white/60',
          )}
          aria-label={ch.id === current.id ? `${ch.label}, seen big` : `See ${ch.label} big`}
        >
          <CameraView channel={ch.id} compact showOverlay={false} />
          <span className="absolute top-1 left-1 rounded bg-black/60 px-1.5 font-mono text-xs text-white">
            {ch.label}
          </span>
        </button>
      ))}
    </div>
  );
}

/**
 * One camera filling the screen, its zones and pieces still drawn: the wheel or a
 * pinch zooms (up to 8×), a drag pans, a double-click or double-tap zooms in there or
 * back out. Every camera shows small below it, in order, the one seen big marked;
 * clicking another switches to it. Esc or × closes it.
 */
export default function CameraFullscreen({ channel, channels = [], onSwitch, onClose }) {
  return (
    <Dialog open={Boolean(channel)} onOpenChange={(open) => !open && onClose()}>
      {channel && (
        <DialogContent
          showCloseButton={false}
          // Over the photo, so it's dark in either theme, like the camera tiles' frame.
          className="top-0 left-0 flex h-dvh w-screen max-w-none translate-x-0 translate-y-0 flex-col gap-0 rounded-none border-0 bg-black p-0 sm:max-w-none"
        >
          <div className="flex items-center gap-3 px-4 py-3 text-white">
            <DialogTitle className="text-base">
              {channel.label} <span className="font-normal text-white/70">· {channel.name}</span>
            </DialogTitle>
            <DialogDescription className="sr-only">
              Live camera. Scroll or pinch to zoom, drag to move around, Escape to close.
            </DialogDescription>
            <span className="ml-auto hidden text-sm text-white/70 sm:inline">
              Scroll or pinch to zoom · drag to move · double-click to zoom there
            </span>
            <DialogClose asChild>
              <Button
                variant="secondary"
                size="icon"
                aria-label="Close"
                className="ml-auto sm:ml-0"
              >
                <X />
              </Button>
            </DialogClose>
          </div>
          <Stage key={channel.id} channel={channel} />
          {onSwitch && <Thumbnails channels={channels} current={channel} onSwitch={onSwitch} />}
        </DialogContent>
      )}
    </Dialog>
  );
}
