import { useCallback, useEffect, useMemo, useRef, useState } from 'react';

import { INDEXING_LABEL, useMCUSession } from '../context/MCUSessionContext';
import { readLocal, useMachineSettings, useServerDocument } from './sorterSettings';
import { MAX_STEPPER_SPEED } from './stepperGroups';
import { loadStepperRoutines } from './stepperRoutines';

const ROUTINES_KEY = 'goklippy_sorter_routines';
const FULL_STEPS_PER_REV = 200;
const DEFAULT_MICROSTEPS = 16;
const RETUNE_DEBOUNCE_MS = 150;
// Steppers-page routines were tuned on bare motors, so they are read as a 1:1 motor at 16 microsteps.
const BARE_MOTOR_STEPS_PER_REV = FULL_STEPS_PER_REV * DEFAULT_MICROSTEPS;

export const FEEDER_IDS = ['c1', 'c2', 'c3'];
export const CHANNEL_IDS = [...FEEDER_IDS, 'c4'];

const normalizeRoutines = (saved) => (Array.isArray(saved) ? saved : []);

export function stepsPerRotorRev(microsteps, gear) {
  return FULL_STEPS_PER_REV * microsteps * gear;
}

export function rpmToStepsPerSec(rpm, stepsPerRev) {
  return (rpm / 60) * stepsPerRev;
}

export function stepsToRpm(stepsPerSec, stepsPerRev) {
  return (stepsPerSec / stepsPerRev) * 60;
}

function roundTo(value, places) {
  const factor = 10 ** places;
  return Math.round(value * factor) / factor;
}

function newId(prefix) {
  return `${prefix}_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 6)}`;
}

export function newMove() {
  return { id: newId('mv'), forward: true, degrees: 10, rpm: 5 };
}

export function newSorterRoutine() {
  return { id: newId('rtn'), name: 'New routine', moves: [newMove()] };
}

/** Converts a Steppers-page routine (motor steps) to rotor degrees and rpm. */
export function fromStepperRoutine(routine) {
  return {
    id: routine.id,
    name: routine.name,
    moves: routine.segments.map((seg) => ({
      id: seg.id,
      forward: seg.forward,
      degrees: roundTo((seg.steps / BARE_MOTOR_STEPS_PER_REV) * 360, 2),
      rpm: roundTo(stepsToRpm(seg.speedHz, BARE_MOTOR_STEPS_PER_REV), 2),
    })),
  };
}

function localRoutines() {
  const saved = readLocal(ROUTINES_KEY);
  return Array.isArray(saved) ? saved : loadStepperRoutines().map(fromStepperRoutine);
}

/**
 * Turns a routine's rotor moves into motor step segments for one channel.
 * Forward means the channel's forward direction, so a reversed channel runs it mirrored.
 */
export function routineSegmentsFor(routine, { config, stepsPerRev }, label) {
  const maxRpm = stepsToRpm(MAX_STEPPER_SPEED, stepsPerRev);
  return routine.moves.map((move, i) => {
    const steps = Math.round((move.degrees / 360) * stepsPerRev);
    const where = `${routine.name}, move ${i + 1}`;
    if (steps < 1) {
      throw new Error(`${where}: ${move.degrees}° is too small a turn for ${label}.`);
    }
    if (move.rpm > maxRpm) {
      throw new Error(
        `${where}: ${move.rpm} rpm is faster than ${label} can go (max ${maxRpm.toFixed(1)} rpm).`,
      );
    }
    return {
      forward: move.forward === config.forward,
      steps,
      speedHz: rpmToStepsPerSec(move.rpm, stepsPerRev),
    };
  });
}

/** Which of the 72° carousel slots the rotor is nearest, and how far off it is. */
export function indexAlignment(angle, indexDegrees) {
  const slots = Math.round(360 / indexDegrees);
  const nearest = Math.round(angle / indexDegrees);
  return {
    slot: ((nearest % slots) + slots) % slots,
    slots,
    offset: angle - nearest * indexDegrees,
  };
}

/**
 * The machine's motors by hand: C1–C4 (speed, direction, gearing, routines, C4 indexing)
 * and the chute's stepper and layer servos, from the machine settings and the board's live
 * motion. Machine › Channels and Machine › Chute's Layers tab use it.
 */
export function useMachineMotors() {
  const {
    connected,
    wsConnected,
    session,
    error: sessionError,
    motion,
    motionHistory,
    stepperObjects,
    stepperRoutineOids,
    stepperRoutineErrors,
    isStepperRunning,
    jogStepper,
    stopStepper,
    stopAllSteppers,
    startStepperIndexing,
    startStepperRoutine,
    resetBoard,
    servoObjects,
    setServosAtAngles,
  } = useMCUSession();

  const [settings, setSettings, settingsSyncError] = useMachineSettings();
  const [zeroes, setZeroes] = useState({});
  const [routines, setRoutines, routinesSyncError] = useServerDocument(
    'routines',
    ROUTINES_KEY,
    localRoutines,
    normalizeRoutines,
  );
  const [activeLayerChoice, setActiveLayerChoice] = useState(0);
  const [actionError, setActionError] = useState('');
  const [resetting, setResetting] = useState(false);

  const motionByOid = useMemo(
    () => Object.fromEntries((motion?.steppers || []).map((m) => [m.oid, m])),
    [motion],
  );

  const targets = settings.feederRpm;

  const channels = useMemo(() => {
    const out = {};
    for (const id of CHANNEL_IDS) {
      const config = settings.channels[id];
      const object = stepperObjects.find((obj) => obj.section === config.stepper);
      const microsteps = Number(object?.meta?.microsteps) || DEFAULT_MICROSTEPS;
      const stepsPerRev = stepsPerRotorRev(microsteps, config.gear);
      const oid = object?.oid;
      const live = oid == null ? null : motionByOid[oid];
      const position = live?.position ?? 0;
      const targetRpm = id === 'c4' ? settings.indexRpm : targets[id];
      out[id] = {
        id,
        config,
        object,
        oid,
        microsteps,
        microstepsAssumed: !object?.meta?.microsteps,
        stepsPerRev,
        targetRpm,
        targetHz: rpmToStepsPerSec(targetRpm, stepsPerRev),
        measuredRpm: live ? Math.abs(stepsToRpm(live.steps_per_sec, stepsPerRev)) : 0,
        direction: live?.direction ?? 0,
        angle: ((position - (zeroes[oid] ?? 0)) / stepsPerRev) * 360,
        running: oid != null && isStepperRunning(oid),
        indexing: oid != null && stepperRoutineOids[oid] === INDEXING_LABEL,
        routineRunning:
          oid != null &&
          Boolean(stepperRoutineOids[oid]) &&
          stepperRoutineOids[oid] !== INDEXING_LABEL,
        error: oid == null ? '' : stepperRoutineErrors[oid] || '',
      };
    }
    return out;
  }, [
    settings,
    targets,
    stepperObjects,
    motionByOid,
    zeroes,
    isStepperRunning,
    stepperRoutineOids,
    stepperRoutineErrors,
  ]);

  const guard = useCallback(async (fn) => {
    setActionError('');
    try {
      await fn();
      return true;
    } catch (err) {
      setActionError(err.message);
      return false;
    }
  }, []);

  const distributor = useMemo(() => {
    const config = settings.distributor;
    const object = stepperObjects.find((obj) => obj.section === config.stepper);
    const microsteps = Number(object?.meta?.microsteps) || DEFAULT_MICROSTEPS;
    const stepsPerRev = stepsPerRotorRev(microsteps, config.gear);
    const oid = object?.oid;
    const live = oid == null ? null : motionByOid[oid];
    const activeLayer = Math.min(activeLayerChoice, config.layers.length - 1);
    return {
      config,
      oid,
      microsteps,
      microstepsAssumed: !object?.meta?.microsteps,
      stepsPerRev,
      speedHz: rpmToStepsPerSec(config.rpm, stepsPerRev),
      measuredRpm: live ? Math.abs(stepsToRpm(live.steps_per_sec, stepsPerRev)) : 0,
      moving: (live?.direction ?? 0) !== 0,
      activeLayer,
      sectionPitch: 360 / Math.max(1, config.sections),
      layers: config.layers.map((layer) => ({
        ...layer,
        servoObject: servoObjects.find((obj) => obj.section === layer.servo),
      })),
    };
  }, [settings.distributor, stepperObjects, servoObjects, motionByOid, activeLayerChoice]);

  // What each running channel was last told, so only real changes re-command it.
  const lastSentRef = useRef({});
  const commandKey = (channel) => `${channel.oid}:${channel.config.forward}:${channel.targetHz}`;

  const startChannel = useCallback(
    (channel) =>
      guard(async () => {
        lastSentRef.current[channel.id] = commandKey(channel);
        await jogStepper(channel.oid, {
          forward: channel.config.forward,
          speedHz: channel.targetHz,
          steps: 0,
        });
      }),
    [guard, jogStepper],
  );

  // Retune channels that are already running when their speed or direction changes.
  useEffect(() => {
    const timer = setTimeout(() => {
      for (const id of CHANNEL_IDS) {
        const channel = channels[id];
        if (channel.running && lastSentRef.current[id] !== commandKey(channel)) {
          startChannel(channel);
        }
      }
    }, RETUNE_DEBOUNCE_MS);
    return () => clearTimeout(timer);
  }, [channels, startChannel]);

  const stopChannel = (channel) => guard(() => stopStepper(channel.oid));

  const startFeeders = () =>
    guard(async () => {
      await Promise.all(
        FEEDER_IDS.filter((id) => channels[id].oid != null).map((id) => {
          const channel = channels[id];
          lastSentRef.current[id] = commandKey(channel);
          return jogStepper(channel.oid, {
            forward: channel.config.forward,
            speedHz: channel.targetHz,
            steps: 0,
          });
        }),
      );
    });

  const stopAll = () => guard(stopAllSteppers);

  const stopFeeders = () =>
    guard(async () => {
      await Promise.all(
        FEEDER_IDS.filter((id) => channels[id].running).map((id) => stopStepper(channels[id].oid)),
      );
    });

  /** Open one layer's servo and close the rest, so pieces can only reach that layer. */
  const selectLayer = (index) =>
    guard(async () => {
      setActiveLayerChoice(index);
      await setServosAtAngles(
        distributor.layers
          .filter((layer) => layer.servoObject)
          .map((layer) => ({
            oid: layer.servoObject.oid,
            angle: layer === distributor.layers[index] ? layer.openAngle : layer.closedAngle,
          })),
      );
    });

  const setDistributor = (patch) =>
    setSettings((prev) => ({ ...prev, distributor: { ...prev.distributor, ...patch } }));

  const setLayer = (index, patch) =>
    setDistributor({
      layers: settings.distributor.layers.map((layer, i) =>
        i === index ? { ...layer, ...patch } : layer,
      ),
    });

  const addLayer = () =>
    setDistributor({
      layers: [
        ...settings.distributor.layers,
        { servo: '', bins: 18, openAngle: 90, closedAngle: 0 },
      ],
    });

  const removeLayer = (index) =>
    setDistributor({ layers: settings.distributor.layers.filter((_, i) => i !== index) });

  /** The channel's chosen routine, or '' for none (including one since deleted). */
  const routineIdFor = (channelId) => {
    const id = settings.routines[channelId];
    return routines.some((r) => r.id === id) ? id : '';
  };

  const chooseRoutine = (channelId, routineId) =>
    setSettings((prev) => ({ ...prev, routines: { ...prev.routines, [channelId]: routineId } }));

  const runRoutine = (channel) =>
    guard(async () => {
      const routine = routines.find((r) => r.id === routineIdFor(channel.id));
      if (routine) {
        const segments = routineSegmentsFor(routine, channel, channel.id.toUpperCase());
        await startStepperRoutine(channel.oid, segments, routine.name);
      }
    });

  /** Turns one rotor revolution by the current gearing, for checking the ratio against a mark. */
  const turnOneRev = (oid, forward, speedHz, stepsPerRev) =>
    guard(() => jogStepper(oid, { forward, speedHz, steps: Math.round(stepsPerRev) }));

  const turnChannelRev = (channel) =>
    turnOneRev(channel.oid, channel.config.forward, channel.targetHz, channel.stepsPerRev);

  const turnDistributorRev = () =>
    turnOneRev(distributor.oid, true, distributor.speedHz, distributor.stepsPerRev);

  const runIndex = (count, forward = channels.c4.config.forward) =>
    guard(() => {
      const c4 = channels.c4;
      return startStepperIndexing(c4.oid, {
        forward,
        speedHz: c4.targetHz,
        stepsPerIndex: (settings.indexDegrees / 360) * c4.stepsPerRev,
        accelHz2: rpmToStepsPerSec(settings.indexAccel, c4.stepsPerRev),
        dwellMs: settings.dwellMs,
        count,
      });
    });

  const handleReset = async () => {
    setResetting(true);
    await guard(resetBoard);
    setResetting(false);
    setZeroes({});
  };

  const setSetting = (key, value) => setSettings((prev) => ({ ...prev, [key]: value }));

  const setFeederRpm = (id, rpm) =>
    setSettings((prev) => ({ ...prev, feederRpm: { ...prev.feederRpm, [id]: rpm } }));

  const setChannelConfig = (id, patch) =>
    setSettings((prev) => ({
      ...prev,
      channels: { ...prev.channels, [id]: { ...prev.channels[id], ...patch } },
    }));

  const zeroChannel = (channel) => {
    const position = motionByOid[channel.oid]?.position;
    if (position != null) {
      setZeroes((prev) => ({ ...prev, [channel.oid]: position }));
    }
  };

  const speedsOrdered = targets.c1 < targets.c2 && targets.c2 < targets.c3;

  return {
    connected,
    wsConnected,
    port: session?.port,
    channels,
    history: motionHistory,
    settings,
    speedsOrdered,
    stepperObjects,
    shutdownReason: motion?.shutdown_reason || '',
    error: actionError || sessionError,
    syncError: settingsSyncError || routinesSyncError,
    resetting,
    setSetting,
    setFeederRpm,
    setChannelConfig,
    turnChannelRev,
    turnDistributorRev,
    setRoutines,
    startChannel,
    stopChannel,
    startFeeders,
    stopAll,
    runIndex,
    stopFeeders,
    distributor,
    servoObjects,
    selectLayer,
    setDistributor,
    setLayer,
    addLayer,
    removeLayer,
    routines,
    routineIdFor,
    chooseRoutine,
    runRoutine,
    handleReset,
    zeroChannel,
  };
}
