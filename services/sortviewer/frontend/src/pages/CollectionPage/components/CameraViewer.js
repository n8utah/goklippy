import { Maximize, Minus, Pause, Play, Plus, X } from 'lucide-react';
import { useCallback, useEffect, useRef, useState } from 'react';

import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from '@/components/ui/dialog';

import { MAX_ZOOM, useZoomPan } from './useZoomPan';

/**
 * The picture, zoomable. Live, it takes the camera's pictures as they come (frameRef);
 * paused, it shows the still taken when it was paused.
 */
function Stage({ camera, watch, paused, still }) {
  const stageRef = useRef(null);
  const zp = useZoomPan(stageRef);

  return (
    <>
      <div
        ref={stageRef}
        className="relative min-h-0 flex-1 touch-none overflow-hidden select-none"
        style={{ cursor: zp.zoom > 1 ? 'grab' : 'zoom-in' }}
        {...zp.handlers}
      >
        <div
          className="absolute inset-0"
          style={{ transform: zp.transform, transformOrigin: '0 0' }}
        >
          {paused ? (
            still && (
              <img src={still} alt="" className="size-full object-contain" draggable={false} />
            )
          ) : (
            <img
              ref={watch.frameRef(camera.id, 'big')}
              alt=""
              className="size-full object-contain"
              draggable={false}
            />
          )}
        </div>
        {paused && (
          <span className="absolute top-3 left-3 rounded-md bg-black/70 px-2 py-0.5 text-sm text-white">
            Paused
          </span>
        )}
      </div>
      <Controls zp={zp} />
    </>
  );
}

/** The other cameras, small and live: one clicked is seen big instead. */
function Others({ cameras, current, watch, onSwitch }) {
  const others = cameras.filter((c) => c.id !== current.id);
  if (!others.length) return null;
  return (
    <div className="flex justify-center gap-2 px-4 pb-3">
      {others.map((c) => (
        <button
          key={c.id}
          type="button"
          onClick={() => onSwitch(c)}
          className="relative aspect-video w-32 overflow-hidden rounded-md bg-media ring-1 ring-white/20 hover:ring-white/60 sm:w-40"
          aria-label={`See ${c.label} big`}
        >
          <img
            ref={watch.frameRef(c.id, 'thumb')}
            alt=""
            className="absolute inset-0 size-full object-contain"
          />
          <span className="absolute top-1 left-1 rounded bg-black/60 px-1.5 font-mono text-xs text-white">
            {c.label}
          </span>
        </button>
      ))}
    </div>
  );
}

function Controls({ zp }) {
  return (
    <div className="flex items-center justify-center gap-2 px-4 pb-3">
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
  );
}

/**
 * One camera filling the screen: the wheel or a pinch zooms (up to 8×), a drag pans, a
 * double-click or double-tap zooms in there or back out; Pause holds the picture
 * (Space too) while the sort goes on. The other cameras show small below it; clicking
 * one switches to it. Esc or × closes it.
 */
export default function CameraViewer({ camera, cameras = [], watch, onSwitch, onClose }) {
  // paused: the still of the picture showing then (an object URL), or '' while live.
  const [still, setStill] = useState('');
  const paused = still !== '';
  const stillRef = useRef('');
  const camId = camera?.id;
  const watchRef = useRef(watch);
  useEffect(() => {
    watchRef.current = watch;
  });

  const togglePause = useCallback(() => {
    const old = stillRef.current;
    const next = old ? '' : watchRef.current.still(camId) || '';
    if (old) URL.revokeObjectURL(old);
    stillRef.current = next;
    setStill(next);
  }, [camId]);
  // Let the still go when it closes.
  useEffect(
    () => () => {
      if (stillRef.current) URL.revokeObjectURL(stillRef.current);
      stillRef.current = '';
    },
    [camId],
  );

  useEffect(() => {
    if (!camera) return undefined;
    const onKey = (e) => {
      if (e.key === ' ' && !e.target.closest?.('button')) {
        e.preventDefault();
        togglePause();
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [camera, togglePause]);

  const unpause = () => {
    if (stillRef.current) URL.revokeObjectURL(stillRef.current);
    stillRef.current = '';
    setStill('');
  };
  const close = () => {
    unpause();
    onClose();
  };
  const switchTo = (cam) => {
    unpause();
    onSwitch(cam);
  };

  return (
    <Dialog open={Boolean(camera)} onOpenChange={(open) => !open && close()}>
      {camera && (
        <DialogContent
          showCloseButton={false}
          // Over the picture, so it's dark in either theme.
          className="top-0 left-0 flex h-dvh w-screen max-w-none translate-x-0 translate-y-0 flex-col gap-0 rounded-none border-0 bg-black p-0 sm:max-w-none"
        >
          <div className="flex items-center gap-3 px-4 py-3 text-white">
            <DialogTitle className="text-base">
              {camera.label} <span className="font-normal text-white/70">· {camera.name}</span>
            </DialogTitle>
            <DialogDescription className="sr-only">
              Live camera. Scroll or pinch to zoom, drag to move around, Space to pause, Escape to
              close.
            </DialogDescription>
            <span className="ml-auto hidden text-sm text-white/70 md:inline">
              Scroll or pinch to zoom · drag to move · Space to pause
            </span>
            <Button
              variant="secondary"
              onClick={togglePause}
              className="ml-auto md:ml-0"
              aria-pressed={paused}
            >
              {paused ? (
                <>
                  <Play /> Continue
                </>
              ) : (
                <>
                  <Pause /> Pause
                </>
              )}
            </Button>
            <DialogClose asChild>
              <Button variant="secondary" size="icon" aria-label="Close">
                <X />
              </Button>
            </DialogClose>
          </div>
          <Stage key={camera.id} camera={camera} watch={watch} paused={paused} still={still} />
          {onSwitch && (
            <Others cameras={cameras} current={camera} watch={watch} onSwitch={switchTo} />
          )}
        </DialogContent>
      )}
    </Dialog>
  );
}
