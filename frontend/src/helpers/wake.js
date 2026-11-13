import { useEffect, useRef } from 'react';

/** Hidden for at least this long counts as having slept (a phone's screen off). */
const SLEPT_MS = 1500;

/**
 * Calls onWake when the page comes back after sleeping: shown again after being hidden
 * (a phone's screen turned back on, the tab brought back), the network back online, or
 * the page restored from the back/forward cache. A phone often freezes a page in the
 * background and wakes it with connections that look open but are dead, so live
 * connections (the sorter's WebSocket, camera streams) reopen on this.
 */
export function useWake(onWake) {
  const cb = useRef(onWake);
  useEffect(() => {
    cb.current = onWake;
  });
  useEffect(() => {
    let hiddenAt = document.hidden ? Date.now() : 0;
    const wake = () => cb.current?.();
    const onVisibility = () => {
      if (document.hidden) {
        hiddenAt = Date.now();
        return;
      }
      if (hiddenAt && Date.now() - hiddenAt >= SLEPT_MS) wake();
      hiddenAt = 0;
    };
    const onPageShow = (e) => e.persisted && wake();
    document.addEventListener('visibilitychange', onVisibility);
    window.addEventListener('online', wake);
    window.addEventListener('pageshow', onPageShow);
    return () => {
      document.removeEventListener('visibilitychange', onVisibility);
      window.removeEventListener('online', wake);
      window.removeEventListener('pageshow', onPageShow);
    };
  }, []);
}
