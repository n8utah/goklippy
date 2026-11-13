import { createContext, useContext, useEffect, useRef, useState } from 'react';

import { openSorterWS } from './api';
import { useWake } from './wake';

const RECONNECT_MS = 2000;

const SorterStateContext = createContext(null);

/**
 * One WebSocket to the sorter for the whole app, reconnecting if it drops, and at once
 * when the page wakes (a phone's screen back on): the old socket may look open but be
 * dead. The frame's status pill and every page read the same state through
 * useSorterState.
 */
export function SorterStateProvider({ children }) {
  const [state, setState] = useState(null);
  const [connected, setConnected] = useState(false);
  const reconnect = useRef(() => {});

  useEffect(() => {
    let ws;
    let timer;
    let closed = false;
    let generation = 0;
    const connect = () => {
      const mine = ++generation;
      ws = openSorterWS({
        onOpen: () => mine === generation && setConnected(true),
        onState: (s) => mine === generation && setState(s),
        onClose: () => {
          if (mine !== generation) return; // replaced by a newer socket
          setConnected(false);
          if (!closed) timer = setTimeout(connect, RECONNECT_MS);
        },
      });
    };
    reconnect.current = () => {
      clearTimeout(timer);
      const old = ws;
      connect(); // the new socket first, so the old one's close is ignored
      old?.close();
    };
    connect();
    return () => {
      closed = true;
      generation++;
      clearTimeout(timer);
      ws?.close();
    };
  }, []);

  useWake(() => reconnect.current());

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
