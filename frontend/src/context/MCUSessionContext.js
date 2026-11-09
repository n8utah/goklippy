import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react';

import {
  checkMCUPCA9685,
  connectMCU,
  disconnectMCU,
  getMCUSession,
  indexMCUStepper,
  jogMCUStepper,
  listMCUSessions,
  logMCU,
  openMCUSessionWS,
  resetMCU,
  setMCUDigitalOut,
  setMCUServo,
  stopMCUStepper,
  startMCUStepperRoutine,
} from '../helpers/api';
import {
  clampServoAngle,
  DEFAULT_SERVO_ANGLE,
  scaleServoAngle,
  flipServoAngle,
} from '../helpers/servoGroups';
import { segmentsForAPI } from '../helpers/stepperRoutines';

const SESSION_STORAGE_KEY = 'goklippy_mcu_session_id';
const RECONNECT_BASE_MS = 1000;
const RECONNECT_MAX_MS = 30000;
const MOTION_HISTORY_LENGTH = 60; // 12 s at 5 readings per second
export const INDEXING_LABEL = 'indexing';

const MCUSessionContext = createContext(null);

export function MCUSessionProvider({ children }) {
  const [session, setSession] = useState(null);
  const [readings, setReadings] = useState({});
  const [connecting, setConnecting] = useState(false);
  const [connectStatus, setConnectStatus] = useState('');
  const [error, setError] = useState('');
  const [wsConnected, setWsConnected] = useState(false);
  const [runningSteppers, setRunningSteppers] = useState({});
  const [stepperJogState, setStepperJogState] = useState({});
  const [stepperRoutineOids, setStepperRoutineOids] = useState({});
  // Routines that stopped on their own, by oid: { [oid]: error message }.
  const [stepperRoutineErrors, setStepperRoutineErrors] = useState({});
  const [servoAngles, setServoAngles] = useState({});
  // Latest measured motion: { at, steppers: [...], shutdown_reason }.
  const [motion, setMotion] = useState(null);
  // Rolling step rate per stepper oid, oldest first.
  const [motionHistory, setMotionHistory] = useState({});
  // PCA9685 board checks by board name: { state, message, ... } or { state: 'checking' }.
  const [pcaStatus, setPcaStatus] = useState({});

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
        onMotion: (snapshot) => {
          setMotion(snapshot);
          setMotionHistory((prev) => {
            const next = { ...prev };
            for (const m of snapshot.steppers || []) {
              next[m.oid] = [...(prev[m.oid] || []), Math.abs(m.steps_per_sec)].slice(-MOTION_HISTORY_LENGTH);
            }
            return next;
          });
        },
        onRoutines: (routines) => {
          // The backend is the source of truth for which routines are running: one
          // can stop on its own when the MCU refuses its steps.
          setStepperRoutineOids((prev) => {
            const next = {};
            for (const r of routines) {
              if (r.running) next[r.oid] = prev[r.oid] || true;
            }
            return next;
          });
          setStepperRoutineErrors(
            Object.fromEntries(routines.filter((r) => r.error).map((r) => [r.oid, r.error])),
          );
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
      setStepperJogState({});
      setStepperRoutineOids({});
      setStepperRoutineErrors({});
      setServoAngles({});
      setPcaStatus({});
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

  const connectInFlightRef = useRef(false);
  const connectTickRef = useRef(null);

  const connect = useCallback(
    async (port, options = {}) => {
      if (connectInFlightRef.current) {
        setError('Connect already in progress — please wait');
        return;
      }
      connectInFlightRef.current = true;
      setConnecting(true);
      setError('');
      const started = performance.now();
      setConnectStatus('Sending connect request to backend…');
      logMCU('connect start', { port, options });

      connectTickRef.current = setInterval(() => {
        const secs = Math.round((performance.now() - started) / 1000);
        setConnectStatus(
          `Connecting to ${port}… (${secs}s — check backend terminal for serial progress)`,
        );
      }, 1000);

      try {
        setConnectStatus(`Connecting to ${port}… (0s — check backend terminal for serial progress)`);
        const data = await connectMCU({
          port,
          baud: options.baud ?? 0,
          use_workspace: options.useWorkspace ?? true,
          config_path: options.configPath,
          profile_id: options.profileId ?? 'ramps14',
        });
        logMCU('connect session received', {
          port,
          sessionId: data.session_id,
          objects: data.objects?.length,
          elapsedMs: Math.round(performance.now() - started),
        });
        setConnectStatus('Connected — opening live telemetry…');
        adoptSession(data);
        return data;
      } catch (err) {
        logMCU('connect error', {
          port,
          elapsedMs: Math.round(performance.now() - started),
          error: err.message,
        });
        setError(err.message);
        throw err;
      } finally {
        if (connectTickRef.current) {
          clearInterval(connectTickRef.current);
          connectTickRef.current = null;
        }
        connectInFlightRef.current = false;
        setConnecting(false);
        setConnectStatus('');
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
      setStepperJogState({});
      setStepperRoutineOids({});
      setStepperRoutineErrors({});
      setServoAngles({});
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
    setStepperJogState({});
    setStepperRoutineOids({});
    setStepperRoutineErrors({});
    setServoAngles({});
    setPcaStatus({});
    sessionStorage.removeItem(SESSION_STORAGE_KEY);
    sessionIdRef.current = null;
  }, [session, closeWebSocket, clearReconnectTimer]);

  /** Restart the board; the session stays connected but all motion has ended. */
  const resetBoard = useCallback(async () => {
    if (!session?.session_id) return;
    await resetMCU(session.session_id);
    setRunningSteppers({});
    setStepperJogState({});
    setStepperRoutineOids({});
    setStepperRoutineErrors({});
    setServoAngles({});
    setPcaStatus({});
    setError('');
  }, [session]);

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
      const clamped = clampServoAngle(angle);
      await setMCUServo(session.session_id, oid, clamped);
      setServoAngles((prev) => ({ ...prev, [oid]: clamped }));
      return clamped;
    },
    [session],
  );

  const pcaBoardNames = useMemo(
    () =>
      (session?.objects || [])
        .filter((obj) => obj.kind === 'i2c' && obj.meta?.device === 'pca9685')
        .map((obj) => obj.meta.board),
    [session],
  );

  const checkPCA9685Boards = useCallback(async () => {
    const sessionId = session?.session_id;
    if (!sessionId || pcaBoardNames.length === 0) return;
    setPcaStatus(Object.fromEntries(pcaBoardNames.map((name) => [name, { state: 'checking' }])));
    try {
      const { boards } = await checkMCUPCA9685(sessionId);
      setPcaStatus(Object.fromEntries((boards || []).map((b) => [b.board, b])));
    } catch (err) {
      setPcaStatus(
        Object.fromEntries(
          pcaBoardNames.map((name) => [name, { state: 'error', message: err.message }]),
        ),
      );
    }
  }, [session?.session_id, pcaBoardNames]);

  // Check boards once per session so a missing one shows up right after connecting.
  const checkedSessionRef = useRef(null);
  useEffect(() => {
    if (!session?.session_id || checkedSessionRef.current === session.session_id) return;
    if (pcaBoardNames.length === 0) return;
    checkedSessionRef.current = session.session_id;
    checkPCA9685Boards();
  }, [session?.session_id, pcaBoardNames, checkPCA9685Boards]);

  const servoObjects = useMemo(
    () =>
      (session?.objects || []).filter(
        (obj) => obj.kind === 'pwm_out' && obj.meta?.device === 'servo',
      ),
    [session],
  );

  const setServosAtAngles = useCallback(
    async (items) => {
      if (!session?.session_id || items.length === 0) return;
      await Promise.all(items.map(({ oid, angle }) => setServo(oid, angle)));
    },
    [session, setServo],
  );

  const setAllServosAngle = useCallback(
    async (angle) => {
      if (servoObjects.length === 0) return;
      await setServosAtAngles(servoObjects.map((obj) => ({ oid: obj.oid, angle })));
    },
    [servoObjects, setServosAtAngles],
  );

  const flipServos = useCallback(
    async (oids) => {
      if (!session?.session_id || oids.length === 0) return;
      await setServosAtAngles(
        oids.map((oid) => ({
          oid,
          angle: flipServoAngle(servoAngles[oid] ?? DEFAULT_SERVO_ANGLE),
        })),
      );
    },
    [session, servoAngles, setServosAtAngles],
  );

  const flipAllServos = useCallback(async () => {
    await flipServos(servoObjects.map((obj) => obj.oid));
  }, [flipServos, servoObjects]);

  const scaleServoAnglesFor = useCallback(
    async (oids, factor) => {
      if (!session?.session_id || oids.length === 0) return;
      await setServosAtAngles(
        oids.map((oid) => ({
          oid,
          angle: scaleServoAngle(servoAngles[oid] ?? DEFAULT_SERVO_ANGLE, factor),
        })),
      );
    },
    [session, servoAngles, setServosAtAngles],
  );

  const scaleAllServoAngles = useCallback(
    async (factor) => {
      await scaleServoAnglesFor(
        servoObjects.map((obj) => obj.oid),
        factor,
      );
    },
    [scaleServoAnglesFor, servoObjects],
  );

  const stepperObjects = useMemo(
    () => (session?.objects || []).filter((obj) => obj.kind === 'stepper'),
    [session],
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

  // A stop, jog or new run on a stepper dismisses its routine's last failure.
  const clearRoutineErrors = useCallback((oids) => {
    setStepperRoutineErrors((prev) => {
      if (!oids.some((oid) => prev[oid])) return prev;
      const next = { ...prev };
      for (const oid of oids) {
        delete next[oid];
      }
      return next;
    });
  }, []);

  const jogStepper = useCallback(
    async (oid, { forward, speedHz, steps }) => {
      if (!session?.session_id) return;
      const stepCount = steps ?? 0;
      const result = await jogMCUStepper(session.session_id, oid, {
        forward,
        speed_hz: speedHz,
        steps: stepCount,
      });
      clearRoutineErrors([oid]);
      if (stepCount <= 0) {
        setStepperJogState((prev) => ({ ...prev, [oid]: { forward, speedHz } }));
        setStepperRunning(oid, true);
        setStepperRoutineOids((prev) => {
          if (!prev[oid]) return prev;
          const next = { ...prev };
          delete next[oid];
          return next;
        });
      } else {
        setStepperJogState((prev) => {
          if (!prev[oid]) return prev;
          const next = { ...prev };
          delete next[oid];
          return next;
        });
        setStepperRunning(oid, false);
      }
      return result;
    },
    [session, setStepperRunning, clearRoutineErrors],
  );

  const stopStepper = useCallback(
    async (oid) => {
      if (!session?.session_id) return;
      const result = await stopMCUStepper(session.session_id, oid);
      clearRoutineErrors([oid]);
      setRunningSteppers((prev) => {
        if (!prev[oid]) return prev;
        const next = { ...prev };
        delete next[oid];
        return next;
      });
      setStepperJogState((prev) => {
        if (!prev[oid]) return prev;
        const next = { ...prev };
        delete next[oid];
        return next;
      });
      setStepperRoutineOids((prev) => {
        if (!prev[oid]) return prev;
        const next = { ...prev };
        delete next[oid];
        return next;
      });
      return result;
    },
    [session, clearRoutineErrors],
  );

  const stopAllSteppers = useCallback(async () => {
    if (!session?.session_id || stepperObjects.length === 0) return;
    const oids = stepperObjects.map((obj) => obj.oid);
    await Promise.all(oids.map((oid) => stopMCUStepper(session.session_id, oid)));
    setRunningSteppers({});
    setStepperJogState({});
    setStepperRoutineOids({});
    setStepperRoutineErrors({});
  }, [session, stepperObjects]);

  const stopSteppers = useCallback(
    async (oids) => {
      if (!session?.session_id || oids.length === 0) return;
      await Promise.all(oids.map((oid) => stopMCUStepper(session.session_id, oid)));
      clearRoutineErrors(oids);
      setRunningSteppers((prev) => {
        const next = { ...prev };
        for (const oid of oids) {
          delete next[oid];
        }
        return next;
      });
      setStepperJogState((prev) => {
        const next = { ...prev };
        for (const oid of oids) {
          delete next[oid];
        }
        return next;
      });
      setStepperRoutineOids((prev) => {
        const next = { ...prev };
        for (const oid of oids) {
          delete next[oid];
        }
        return next;
      });
    },
    [session, clearRoutineErrors],
  );

  const startStepperRoutine = useCallback(
    async (oid, segments, routineName = '') => {
      if (!session?.session_id) return;
      await startMCUStepperRoutine(session.session_id, oid, segmentsForAPI(segments));
      clearRoutineErrors([oid]);
      setStepperRoutineOids((prev) => ({ ...prev, [oid]: routineName || true }));
      setRunningSteppers((prev) => {
        if (!prev[oid]) return prev;
        const next = { ...prev };
        delete next[oid];
        return next;
      });
      setStepperJogState((prev) => {
        if (!prev[oid]) return prev;
        const next = { ...prev };
        delete next[oid];
        return next;
      });
    },
    [session, clearRoutineErrors],
  );

  /** Rotate in fixed increments; count 0 repeats until stopped. */
  const startStepperIndexing = useCallback(
    async (oid, { forward, speedHz, stepsPerIndex, dwellMs, count }) => {
      if (!session?.session_id) return;
      await indexMCUStepper(session.session_id, oid, {
        forward,
        speed_hz: speedHz,
        steps_per_index: stepsPerIndex,
        dwell_ms: dwellMs,
        count,
      });
      clearRoutineErrors([oid]);
      setRunningSteppers((prev) => {
        if (!prev[oid]) return prev;
        const next = { ...prev };
        delete next[oid];
        return next;
      });
      setStepperRoutineOids((prev) => ({ ...prev, [oid]: INDEXING_LABEL }));
    },
    [session, clearRoutineErrors],
  );

  const startStepperRoutineForMembers = useCallback(
    async (routine) => {
      if (!session?.session_id || !routine?.segments?.length) return;
      const keys = new Set(routine.members);
      const targets = stepperObjects.filter((obj) => keys.has(obj.slot_id || obj.section || String(obj.oid)));
      for (const obj of targets) {
        await startStepperRoutine(obj.oid, routine.segments, routine.name);
      }
    },
    [session, stepperObjects, startStepperRoutine],
  );

  const startSteppers = useCallback(
    async (oids, { forward = true, speedHz = 500 } = {}) => {
      if (!session?.session_id || oids.length === 0) return;
      await Promise.all(oids.map((oid) => jogStepper(oid, { forward, speedHz, steps: 0 })));
    },
    [session, jogStepper],
  );

  const startSteppersAtSpeeds = useCallback(
    async (items) => {
      if (!session?.session_id || items.length === 0) return;
      await Promise.all(
        items.map(({ oid, forward = true, speedHz = 500 }) =>
          jogStepper(oid, { forward, speedHz, steps: 0 }),
        ),
      );
    },
    [session, jogStepper],
  );

  const startAllSteppers = useCallback(
    async (options = {}) => {
      if (stepperObjects.length === 0) return;
      await startSteppers(
        stepperObjects.map((obj) => obj.oid),
        options,
      );
    },
    [stepperObjects, startSteppers],
  );

  const reverseSteppers = useCallback(
    async (oids) => {
      if (!session?.session_id || oids.length === 0) return;
      const updates = oids
        .filter((oid) => stepperJogState[oid])
        .map((oid) => ({
          oid,
          forward: !stepperJogState[oid].forward,
          speedHz: stepperJogState[oid].speedHz,
        }));
      if (updates.length === 0) return;

      await Promise.all(
        updates.map((item) =>
          jogMCUStepper(session.session_id, item.oid, {
            forward: item.forward,
            speed_hz: item.speedHz,
            steps: 0,
          }),
        ),
      );

      setStepperJogState((prev) => {
        const next = { ...prev };
        for (const item of updates) {
          next[item.oid] = { forward: item.forward, speedHz: item.speedHz };
        }
        return next;
      });
    },
    [session, stepperJogState],
  );

  const reverseAllSteppers = useCallback(async () => {
    await reverseSteppers(Object.keys(stepperJogState).map(Number));
  }, [reverseSteppers, stepperJogState]);

  const scaleStepperSpeeds = useCallback(
    async (oids, factor) => {
      if (!session?.session_id || oids.length === 0) return;

      const updates = oids
        .filter((oid) => stepperJogState[oid])
        .map((oid) => ({
          oid,
          forward: stepperJogState[oid].forward,
          speedHz: Math.min(50000, Math.max(1, Math.round(stepperJogState[oid].speedHz * factor))),
        }));
      if (updates.length === 0) return;

      await Promise.all(
        updates.map((item) =>
          jogMCUStepper(session.session_id, item.oid, {
            forward: item.forward,
            speed_hz: item.speedHz,
            steps: 0,
          }),
        ),
      );

      setStepperJogState((prev) => {
        const next = { ...prev };
        for (const item of updates) {
          next[item.oid] = { forward: item.forward, speedHz: item.speedHz };
        }
        return next;
      });
    },
    [session, stepperJogState],
  );

  const scaleAllStepperSpeeds = useCallback(
    async (factor) => {
      await scaleStepperSpeeds(Object.keys(stepperJogState).map(Number), factor);
    },
    [scaleStepperSpeeds, stepperJogState],
  );

  const isStepperRunning = useCallback(
    (oid) => Boolean(runningSteppers[oid]),
    [runningSteppers],
  );

  const value = useMemo(
    () => ({
      session,
      readings,
      connecting,
      connectStatus,
      error,
      wsConnected,
      connected: Boolean(session?.session_id),
      objectsBySlot,
      readingForSlot,
      connect,
      disconnect,
      resetBoard,
      resumeSession,
      setDigitalOut,
      setServo,
      servoObjects,
      servoAngles,
      setServosAtAngles,
      setAllServosAngle,
      flipServos,
      flipAllServos,
      scaleServoAnglesFor,
      scaleAllServoAngles,
      pcaStatus,
      checkPCA9685Boards,
      jogStepper,
      stopStepper,
      stopAllSteppers,
      startAllSteppers,
      startSteppers,
      startSteppersAtSpeeds,
      stopSteppers,
      reverseSteppers,
      reverseAllSteppers,
      scaleAllStepperSpeeds,
      scaleStepperSpeeds,
      stepperObjects,
      stepperJogState,
      stepperRoutineOids,
      stepperRoutineErrors,
      isStepperRunning,
      setStepperRunning,
      startStepperRoutine,
      startStepperIndexing,
      motion,
      motionHistory,
      startStepperRoutineForMembers,
    }),
    [
      session,
      readings,
      connecting,
      connectStatus,
      error,
      wsConnected,
      objectsBySlot,
      readingForSlot,
      connect,
      disconnect,
      resetBoard,
      resumeSession,
      setDigitalOut,
      setServo,
      servoObjects,
      servoAngles,
      setServosAtAngles,
      setAllServosAngle,
      flipServos,
      flipAllServos,
      scaleServoAnglesFor,
      scaleAllServoAngles,
      pcaStatus,
      checkPCA9685Boards,
      jogStepper,
      stopStepper,
      stopAllSteppers,
      startAllSteppers,
      startSteppers,
      startSteppersAtSpeeds,
      stopSteppers,
      reverseSteppers,
      reverseAllSteppers,
      scaleAllStepperSpeeds,
      scaleStepperSpeeds,
      stepperObjects,
      stepperJogState,
      stepperRoutineOids,
      stepperRoutineErrors,
      isStepperRunning,
      setStepperRunning,
      startStepperRoutine,
      startStepperIndexing,
      motion,
      motionHistory,
      startStepperRoutineForMembers,
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
