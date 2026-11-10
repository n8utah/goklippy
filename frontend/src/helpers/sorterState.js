import { useEffect, useState } from 'react';

import { openSorterWS } from './api';

const RECONNECT_MS = 2000;

/** The sorter's live state over its WebSocket, reconnecting if it drops. */
export function useSorterState() {
  const [state, setState] = useState(null);
  const [connected, setConnected] = useState(false);

  useEffect(() => {
    let ws;
    let timer;
    let closed = false;
    const connect = () => {
      ws = openSorterWS({
        onOpen: () => setConnected(true),
        onState: setState,
        onClose: () => {
          setConnected(false);
          if (!closed) timer = setTimeout(connect, RECONNECT_MS);
        },
      });
    };
    connect();
    return () => {
      closed = true;
      clearTimeout(timer);
      ws?.close();
    };
  }, []);

  return { state, connected };
}
