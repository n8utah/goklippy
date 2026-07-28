import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react';

import {
  connectMCU,
  disconnectMCU,
  getMCUSession,
  jogMCUStepper,
  listMCUSessions,
  openMCUSessionWS,
  setMCUDigitalOut,
  setMCUServo,
  stopMCUStepper,
} from '../helpers/api';

const SESSION_STORAGE_KEY = 'goklippy_mcu_session_id';
const RECONNECT_BASE_MS = 1000;
const RECONNECT_MAX_MS = 30000;

const MCUSessionContext = createContext(null);

export function MCUSessionProvider({ children }) {
  const [session, setSession] = useState(null);
  const [readings, setReadings] = useState({});
  const [connecting, setConnecting] = useState(false);
  const [error, setError] = useState('');
  const [wsConnected, setWsConnected] = useState(false);
  const [runningSteppers, setRunningSteppers] = useState({});

  const wsRef = useRef(null);
  const sessionIdRef = useRef(null);
  const reconnectEnabledRef = useRef(false);
  const reconnectAttemptRef = useRef(0);
  const reconnectTimerRef = useRef(null);

  const clearReconnectTimer = useCallback(() => {
    if (reconnectTimerRef.current) {
      clearTimeout(reconnectTimerRef.current);
      reconnectTimerRef.current = null;
    }
  }, []);

  const closeWebSocket = useCallback(() => {
    clearReconnectTimer();
    if (wsRef.current) {
      wsRef.current._intentionalClose = true;
      wsRef.current.close();
      wsRef.current = null;
    }
    setWsConnected(false);
  }, [clearReconnectTimer]);

  const attachWebSocket = useCallback(
    (sessionId) => {
      closeWebSocket();
      sessionIdRef.current = sessionId;
      reconnectEnabledRef.current = true;

      const ws = openMCUSessionWS(sessionId, {
        onOpen: () => {
          reconnectAttemptRef.current = 0;
          setWsConnected(true);
          setError('');
        },
        onSession: (payload) => {
          setSession((prev) => ({ ...prev, ...payload }));
        },
        onReadings: (payload) => {
          setReadings(payload || {});
        },
        onError: () => {
          setWsConnected(false);
        },
        onClose: () => {
          wsRef.current = null;
          setWsConnected(false);
          if (ws._intentionalClose || !reconnectEnabledRef.current) {
            return;
          }
          scheduleReconnectRef.current?.();
        },
      });

      ws._intentionalClose = false;
      wsRef.current = ws;
    },
    [closeWebSocket],
  );

  const scheduleReconnectRef = useRef(null);

  const scheduleReconnect = useCallback(async () => {
    if (!reconnectEnabledRef.current || !sessionIdRef.current) {
      return;
    }

    clearReconnectTimer();
    const attempt = reconnectAttemptRef.current;
    const delay = Math.min(RECONNECT_MAX_MS, RECONNECT_BASE_MS * 2 ** attempt);
    reconnectAttemptRef.current = attempt + 1;
    setError(`Live connection lost — reconnecting in ${Math.round(delay / 1000)}s…`);

    reconnectTimerRef.current = setTimeout(async () => {
      if (!reconnectEnabledRef.current) {
        return;
      }

      const sessionId = sessionIdRef.current;
      setError('Live connection lost — reconnecting…');

      try {
        const data = await getMCUSession(sessionId);
        setSession((prev) => ({ ...prev, ...data }));
        sessionStorage.setItem(SESSION_STORAGE_KEY, data.session_id);
        attachWebSocket(data.session_id);
        return;
      } catch {
        // session id stale — fall through
      }

      try {
        const { sessions } = await listMCUSessions();
        if (sessions?.length > 0 && reconnectEnabledRef.current) {
          const data = sessions[0];
          setSession(data);
          sessionStorage.setItem(SESSION_STORAGE_KEY, data.session_id);
          attachWebSocket(data.session_id);
          return;
        }
      } catch {
        // backend unavailable
      }

      if (reconnectEnabledRef.current) {
        scheduleReconnectRef.current?.();
      } else {
        setError('Live connection lost');
      }
    }, delay);
  }, [attachWebSocket, clearReconnectTimer]);

  scheduleReconnectRef.current = scheduleReconnect;

  const adoptSession = useCallback(
    (data) => {
      setSession(data);
      setReadings({});
      setRunningSteppers({});
      setError('');
      sessionStorage.setItem(SESSION_STORAGE_KEY, data.session_id);
      attachWebSocket(data.session_id);
    },
    [attachWebSocket],
  );

  const resumeSession = useCallback(async () => {
    const storedId = sessionStorage.getItem(SESSION_STORAGE_KEY);
    if (storedId) {
      try {
        const data = await getMCUSession(storedId);
        adoptSession(data);
        return true;
      } catch {
        sessionStorage.removeItem(SESSION_STORAGE_KEY);
      }
    }
    try {
      const { sessions } = await listMCUSessions();
      if (sessions?.length > 0) {
        adoptSession(sessions[0]);
        return true;
      }
    } catch {
      // no active backend sessions
    }
    return false;
  }, [adoptSession]);

  useEffect(() => {
    resumeSession();
    return () => {
      reconnectEnabledRef.current = false;
      clearReconnectTimer();
      closeWebSocket();
    };
  }, [resumeSession, closeWebSocket, clearReconnectTimer]);

  const connect = useCallback(
    async (port, options = {}) => {
      setConnecting(true);
      setError('');
      try {
        const data = await connectMCU({
          port,
          baud: options.baud,
          use_workspace: options.useWorkspace ?? true,
          config_path: options.configPath,
          profile_id: options.profileId ?? 'ramps14',
        });
        adoptSession(data);
        return data;
      } catch (err) {
        setError(err.message);
        throw err;
      } finally {
        setConnecting(false);
      }
    },
    [adoptSession],
  );

  const disconnect = useCallback(async () => {
    reconnectEnabledRef.current = false;
    clearReconnectTimer();
    closeWebSocket();

    if (!session?.session_id) {
      setSession(null);
      setReadings({});
      setRunningSteppers({});
      sessionStorage.removeItem(SESSION_STORAGE_KEY);
      sessionIdRef.current = null;
      return;
    }

    try {
      await disconnectMCU(session.session_id);
    } catch {
      // still clear local state
    }

    setSession(null);
    setReadings({});
    setRunningSteppers({});
    sessionStorage.removeItem(SESSION_STORAGE_KEY);
    sessionIdRef.current = null;
  }, [session, closeWebSocket, clearReconnectTimer]);

  const objectsBySlot = useMemo(() => {
    const map = {};
    for (const obj of session?.objects || []) {
      if (obj.slot_id) {
        map[obj.slot_id] = obj;
      }
    }
    return map;
  }, [session]);

  const readingForSlot = useCallback(
    (slotId) => {
      const obj = objectsBySlot[slotId];
      if (!obj) {
        return null;
      }
      return readings[String(obj.oid)] || null;
    },
    [objectsBySlot, readings],
  );

  const setDigitalOut = useCallback(
    async (oid, value) => {
      if (!session?.session_id) return;
      await setMCUDigitalOut(session.session_id, oid, value);
    },
    [session],
  );

  const setServo = useCallback(
    async (oid, angle) => {
      if (!session?.session_id) return;
      await setMCUServo(session.session_id, oid, angle);
    },
    [session],
  );

  const jogStepper = useCallback(
    async (oid, { forward, speedHz, steps }) => {
      if (!session?.session_id) return;
      return jogMCUStepper(session.session_id, oid, { forward, speed_hz: speedHz, steps });
    },
    [session],
  );

  const stopStepper = useCallback(
    async (oid) => {
      if (!session?.session_id) return;
      const result = await stopMCUStepper(session.session_id, oid);
      setRunningSteppers((prev) => {
        if (!prev[oid]) return prev;
        const next = { ...prev };
        delete next[oid];
        return next;
      });
      return result;
    },
    [session],
  );

  const isStepperRunning = useCallback(
    (oid) => Boolean(runningSteppers[oid]),
    [runningSteppers],
  );

  const setStepperRunning = useCallback((oid, running) => {
    setRunningSteppers((prev) => {
      if (running) {
        if (prev[oid]) return prev;
        return { ...prev, [oid]: true };
      }
      if (!prev[oid]) return prev;
      const next = { ...prev };
      delete next[oid];
      return next;
    });
  }, []);

  const value = useMemo(
    () => ({
      session,
      readings,
      connecting,
      error,
      wsConnected,
      connected: Boolean(session?.session_id),
      objectsBySlot,
      readingForSlot,
      connect,
      disconnect,
      resumeSession,
      setDigitalOut,
      setServo,
      jogStepper,
      stopStepper,
      isStepperRunning,
      setStepperRunning,
    }),
    [
      session,
      readings,
      connecting,
      error,
      wsConnected,
      objectsBySlot,
      readingForSlot,
      connect,
      disconnect,
      resumeSession,
      setDigitalOut,
      setServo,
      jogStepper,
      stopStepper,
      isStepperRunning,
      setStepperRunning,
    ],
  );

  return <MCUSessionContext.Provider value={value}>{children}</MCUSessionContext.Provider>;
}

export function useMCUSession() {
  const ctx = useContext(MCUSessionContext);
  if (!ctx) {
    throw new Error('useMCUSession must be used within MCUSessionProvider');
  }
  return ctx;
}
