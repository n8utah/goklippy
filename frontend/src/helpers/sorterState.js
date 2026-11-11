import { createContext, useContext, useEffect, useState } from 'react';

import { openSorterWS } from './api';

const RECONNECT_MS = 2000;

const SorterStateContext = createContext(null);

/**
 * One WebSocket to the sorter for the whole app, reconnecting if it drops. The frame's
 * status pill and every page read the same state through useSorterState.
 */
export function SorterStateProvider({ children }) {
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

  return (
    <SorterStateContext.Provider value={{ state, connected }}>
      {children}
    </SorterStateContext.Provider>
  );
}

/** The sorter's live state: { state, connected }. Needs SorterStateProvider above it. */
export function useSorterState() {
  const ctx = useContext(SorterStateContext);
  if (!ctx) throw new Error('useSorterState must be used within SorterStateProvider');
  return ctx;
}
