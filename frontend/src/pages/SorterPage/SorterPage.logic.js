import { useCallback, useEffect, useMemo, useRef, useState } from 'react';

import { INDEXING_LABEL, useMCUSession } from '../../context/MCUSessionContext';
import { MAX_STEPPER_SPEED } from '../../helpers/stepperGroups';
import { loadStepperRoutines } from '../../helpers/stepperRoutines';

const STORAGE_KEY = 'goklippy_sorter_settings';
const ROUTINES_KEY = 'goklippy_sorter_routines';
const FULL_STEPS_PER_REV = 200;
const DEFAULT_MICROSTEPS = 16;
const RETUNE_DEBOUNCE_MS = 150;
// Steppers-page routines were tuned on bare motors, so they are read as a 1:1 motor at 16 microsteps.
const BARE_MOTOR_STEPS_PER_REV = FULL_STEPS_PER_REV * DEFAULT_MICROSTEPS;

export const FEEDER_IDS = ['c1', 'c2', 'c3'];
export const CHANNEL_IDS = [...FEEDER_IDS, 'c4'];

const DEFAULT_SETTINGS = {
  feederRpm: { c1: 5, c2: 7.5, c3: 11.25 },
  indexDegrees: 72,
  indexRpm: 10,
  dwellMs: 1000,
  showSettings: true,
  distributor: {
    stepper: 'stepper_e1',
    gear: 5,
    rpm: 5,
    layers: [
      { servo: '', bins: 18, openAngle: 90, closedAngle: 0 },
      { servo: '', bins: 18, openAngle: 90, closedAngle: 0 },
      { servo: '', bins: 18, openAngle: 90, closedAngle: 0 },
      { servo: '', bins: 12, openAngle: 90, closedAngle: 0 },
    ],
  },
  channels: {
    c1: { stepper: 'stepper_x', forward: true, gear: 1 },
    c2: { stepper: 'stepper_y', forward: true, gear: 1 },
    c3: { stepper: 'stepper_z', forward: true, gear: 1 },
    c4: { stepper: 'stepper_e0', forward: true, gear: 10.8 },
  },
  routines: {},
};

/** Older saves stored C2 and C3 as multiples of C1. */
function feederRpmFrom(saved) {
  if (saved.feederRpm) return saved.feederRpm;
  if (saved.baseRpm == null) return DEFAULT_SETTINGS.feederRpm;
  const c2 = saved.baseRpm * (saved.c2Factor ?? 1.5);
  return { c1: saved.baseRpm, c2, c3: c2 * (saved.c3Factor ?? 1.5) };
}

function loadSettings() {
  try {
    const saved = JSON.parse(localStorage.getItem(STORAGE_KEY));
    if (saved?.channels) {
      // eslint-disable-next-line no-unused-vars
      const { baseRpm, c2Factor, c3Factor, ...rest } = saved;
      return {
        ...DEFAULT_SETTINGS,
        ...rest,
        feederRpm: feederRpmFrom(saved),
        distributor: { ...DEFAULT_SETTINGS.distributor, ...saved.distributor },
        channels: { ...DEFAULT_SETTINGS.channels, ...saved.channels },
      };
    }
  } catch {
    // fall through to defaults
  }
  return DEFAULT_SETTINGS;
}

function saveSettings(settings) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(settings));
  } catch {
    // settings just won't persist
  }
}

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

function loadSorterRoutines() {
  try {
    const saved = JSON.parse(localStorage.getItem(ROUTINES_KEY));
    if (Array.isArray(saved)) return saved;
  } catch {
    // fall through to a fresh copy
  }
  return loadStepperRoutines().map(fromStepperRoutine);
}

function saveSorterRoutines(routines) {
  try {
    localStorage.setItem(ROUTINES_KEY, JSON.stringify(routines));
  } catch {
    // routines just won't persist
  }
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
      throw new Error(`${where}: ${move.rpm} rpm is faster than ${label} can go (max ${maxRpm.toFixed(1)} rpm).`);
    }
    return { forward: move.forward === config.forward, steps, speedHz: rpmToStepsPerSec(move.rpm, stepsPerRev) };
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

/** Signed shortest rotation from one angle to another, in -180..180 degrees. */
export function shortestRotation(from, to) {
  const delta = (((to - from) % 360) + 360) % 360;
  return delta > 180 ? delta - 360 : delta;
}

export function useSorterPageLogic() {
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

  const [settings, setSettings] = useState(loadSettings);
  const [zeroes, setZeroes] = useState({});
  const [routines, setRoutines] = useState(loadSorterRoutines);
  const [activeLayerChoice, setActiveLayerChoice] = useState(0);
  const [targetBin, setTargetBin] = useState(null);
  const [actionError, setActionError] = useState('');
  const [resetting, setResetting] = useState(false);

  useEffect(() => saveSettings(settings), [settings]);
  useEffect(() => saveSorterRoutines(routines), [routines]);

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
        routineRunning: oid != null && Boolean(stepperRoutineOids[oid]) && stepperRoutineOids[oid] !== INDEXING_LABEL,
        error: oid == null ? '' : stepperRoutineErrors[oid] || '',
      };
    }
    return out;
  }, [settings, targets, stepperObjects, motionByOid, zeroes, isStepperRunning, stepperRoutineOids, stepperRoutineErrors]);

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
    const bins = config.layers[activeLayer].bins;
    const angle = (((live?.position ?? 0) - (zeroes[oid] ?? 0)) / stepsPerRev) * 360;
    return {
      config,
      oid,
      microsteps,
      microstepsAssumed: !object?.meta?.microsteps,
      stepsPerRev,
      speedHz: rpmToStepsPerSec(config.rpm, stepsPerRev),
      measuredRpm: live ? Math.abs(stepsToRpm(live.steps_per_sec, stepsPerRev)) : 0,
      moving: (live?.direction ?? 0) !== 0,
      angle,
      activeLayer,
      bins,
      targetBin: targetBin != null && targetBin < bins ? targetBin : null,
      alignment: indexAlignment(angle, 360 / bins),
      layers: config.layers.map((layer) => ({
        ...layer,
        servoObject: servoObjects.find((obj) => obj.section === layer.servo),
      })),
    };
  }, [settings.distributor, stepperObjects, servoObjects, motionByOid, zeroes, activeLayerChoice, targetBin]);

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

  const pointToBin = (bin) =>
    guard(async () => {
      setTargetBin(bin);
      if (distributor.oid == null) return;
      if (distributor.moving) {
        throw new Error('The distributor is still moving. Wait for it to stop.');
      }
      const rotation = shortestRotation(distributor.angle, (bin * 360) / distributor.bins);
      const steps = Math.round((Math.abs(rotation) / 360) * distributor.stepsPerRev);
      if (steps > 0) {
        await jogStepper(distributor.oid, { forward: rotation > 0, speedHz: distributor.speedHz, steps });
      }
    });

  const setDistributor = (patch) =>
    setSettings((prev) => ({ ...prev, distributor: { ...prev.distributor, ...patch } }));

  const setLayer = (index, patch) =>
    setDistributor({
      layers: settings.distributor.layers.map((layer, i) => (i === index ? { ...layer, ...patch } : layer)),
    });

  const addLayer = () =>
    setDistributor({
      layers: [...settings.distributor.layers, { servo: '', bins: 18, openAngle: 90, closedAngle: 0 }],
    });

  const removeLayer = (index) =>
    setDistributor({ layers: settings.distributor.layers.filter((_, i) => i !== index) });

  const zeroDistributor = () => {
    const position = motionByOid[distributor.oid]?.position;
    if (position != null) {
      setZeroes((prev) => ({ ...prev, [distributor.oid]: position }));
    }
  };

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
    pointToBin,
    setDistributor,
    setLayer,
    addLayer,
    removeLayer,
    zeroDistributor,
    routines,
    routineIdFor,
    chooseRoutine,
    runRoutine,
    handleReset,
    zeroChannel,
  };
}
