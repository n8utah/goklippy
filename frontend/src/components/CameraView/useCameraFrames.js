import { useEffect } from 'react';

import { sorterCameraFrameURL } from '@/helpers/api';

/** At most this often a new frame is asked for (about 12 a second). */
const FRAME_MS = 80;

/** Resolves once the page is shown (at once if it is). */
function whenVisible(signal) {
  if (!document.hidden) return Promise.resolve();
  return new Promise((resolve) => {
    const done = () => {
      if (document.hidden && !signal.aborted) return;
      document.removeEventListener('visibilitychange', done);
      signal.removeEventListener('abort', done);
      resolve();
    };
    document.addEventListener('visibilitychange', done);
    signal.addEventListener('abort', done);
  });
}

const sleep = (ms, signal) =>
  new Promise((resolve) => {
    const timer = setTimeout(resolve, ms);
    signal.addEventListener('abort', () => {
      clearTimeout(timer);
      resolve();
    });
  });

/**
 * Shows a channel's live camera in imgRef's <img>, one frame at a time: each is
 * fetched, shown, and decoded before the next is asked for, and the one before it is
 * let go. An MJPEG stream in an <img> instead holds a connection open for good, and a
 * tab left watching three of them for hours grew until it stopped responding (and a
 * stream reopened after a wake could leave the old one holding its connection). Nothing
 * is fetched while the page is hidden. onFail is called when a frame can't be had.
 */
export function useCameraFrames(imgRef, channel, { enabled = true, attempt = 0, onFail }) {
  useEffect(() => {
    if (!enabled) return undefined;
    const abort = new AbortController();
    const { signal } = abort;
    let shown = null; // the object URL on screen

    const run = async () => {
      while (!signal.aborted) {
        await whenVisible(signal);
        if (signal.aborted) return;
        const started = performance.now();
        let blob;
        try {
          const res = await fetch(sorterCameraFrameURL(channel), { cache: 'no-store', signal });
          if (!res.ok) throw new Error(`frame: ${res.status}`);
          blob = await res.blob();
        } catch (err) {
          if (!signal.aborted) onFail?.(err);
          return;
        }
        const img = imgRef.current;
        if (signal.aborted || !img) return;
        const url = URL.createObjectURL(blob);
        img.src = url;
        try {
          await img.decode();
        } catch {
          // A frame that won't decode is skipped; the next replaces it.
        }
        if (shown) URL.revokeObjectURL(shown);
        shown = url;
        await sleep(Math.max(0, FRAME_MS - (performance.now() - started)), signal);
      }
    };
    run();

    return () => {
      abort.abort();
      if (shown) URL.revokeObjectURL(shown);
    };
    // onFail is read when called; a new one each render needn't restart the frames.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [imgRef, channel, enabled, attempt]);
}
