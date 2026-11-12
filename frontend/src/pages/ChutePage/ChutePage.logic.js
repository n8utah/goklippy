import { useCallback, useEffect, useMemo, useState } from 'react';

import { useMCUSession } from '../../context/MCUSessionContext';
import {
  aimSorterChute,
  calibrateSorterChute,
  cancelSorterJob,
  getSorterChute,
  moveSorterChannel,
  sendToSorterBin,
  sorterCommand,
  testSorterLayer,
  zeroSorterChute,
} from '../../helpers/api';
import { useMachineSettings } from '../../helpers/sorterSettings';
import { useSorterState } from '../../helpers/sorterState';

// After a jog, read the angle back once the move should be over.
const JOG_READBACK_MS = 600;
export const JOG_STEPS = [-2, -0.25, 0.25, 2];

export function useChutePageLogic() {
  const { session } = useMCUSession();
  const { state, connected: liveConnected } = useSorterState();
  const [settings, setSettings, syncError, savedCount] = useMachineSettings();
  const [chute, setChute] = useState(null); // GET /sorter/chute: status, geometry, layers
  const [layerIndex, setLayerIndex] = useState(0);
  const [captures, setCaptures] = useState({ first: null, last: null });
  const [calibration, setCalibration] = useState(null);
  const [testDwellMs, setTestDwellMs] = useState(1000);
  const [error, setError] = useState('');

  const distributor = settings.distributor;

  const refreshChute = useCallback(async () => {
    const data = await getSorterChute();
    setChute(data);
    return data;
  }, []);

  // Bin angles come from the saved settings, so read them again after each save.
  useEffect(() => {
    let cancelled = false;
    getSorterChute().then(
      (data) => {
        if (!cancelled) setChute(data);
      },
      (err) => {
        if (!cancelled) setError(err.message);
      },
    );
    return () => {
      cancelled = true;
    };
  }, [savedCount]);

  const guard = useCallback(async (fn) => {
    setError('');
    try {
      await fn();
    } catch (err) {
      setError(err.message);
    }
  }, []);

  // The live snapshot's chute (once a second) is newer than the last GET unless a
  // capture or jog just read it.
  const status = state?.chute ?? chute?.status ?? null;
  const layers = chute?.layers ?? [];
  const layer = layers[Math.min(layerIndex, Math.max(0, layers.length - 1))] ?? [];
  const busy = Boolean(state?.job);
  const sorting = state?.state === 'running' || state?.state === 'homing';

  const endstopObjects = useMemo(
    () => (session?.objects || []).filter((obj) => obj.kind === 'endstop'),
    [session],
  );

  // Layers the calibration can use: at least two bins per section.
  const calibrationLayers = useMemo(
    () =>
      distributor.layers
        .map((l, i) => ({
          index: i,
          perSection: Math.floor(l.bins / Math.max(1, distributor.sections)),
        }))
        .filter((l) => l.perSection >= 2),
    [distributor],
  );
  const [calibrationLayer, setCalibrationLayer] = useState(0);
  const calLayer =
    calibrationLayers.find((l) => l.index === calibrationLayer) ?? calibrationLayers[0];

  const home = () => guard(() => sorterCommand('home'));
  const zero = () => guard(() => zeroSorterChute().then(refreshChute));
  const cancel = () => guard(cancelSorterJob);
  const aim = (angle) => guard(() => aimSorterChute(angle));
  const sendToBin = (bin) => guard(() => sendToSorterBin(layerIndex + 1, bin));
  const testLayer = () => guard(() => testSorterLayer(layerIndex + 1, testDwellMs));

  const jog = (degrees) =>
    guard(async () => {
      await moveSorterChannel('distributor', degrees, distributor.rpm);
      setTimeout(() => refreshChute().catch(() => {}), JOG_READBACK_MS);
    });

  /** Reads the chute's angle from the board now and keeps it as a capture. */
  const capture = (which) =>
    guard(async () => {
      const data = await refreshChute();
      if (data.status?.angle == null) {
        throw new Error('The chute has no angle to capture until it is homed (or zeroed).');
      }
      setCaptures((prev) => ({ ...prev, [which]: data.status.angle }));
      setCalibration(null);
    });

  const workItOut = () =>
    guard(async () => {
      if (!calLayer) throw new Error('No layer has at least two bins per section to calibrate on.');
      setCalibration(
        await calibrateSorterChute(captures.first, captures.last, calLayer.perSection),
      );
    });

  const applyCalibration = () => {
    if (!calibration) return;
    setDistributor({
      sectionWidth: calibration.section_width,
      firstSectionOffset: calibration.first_section_offset,
    });
    setCaptures({ first: null, last: null });
    setCalibration(null);
  };

  const setDistributor = (patch) =>
    setSettings((prev) => ({ ...prev, distributor: { ...prev.distributor, ...patch } }));

  return {
    state,
    liveConnected,
    status,
    geometry: chute?.geometry,
    layers,
    layer,
    layerIndex,
    setLayerIndex,
    distributor,
    setDistributor,
    endstopObjects,
    busy,
    sorting,
    error: error || syncError,
    home,
    zero,
    cancel,
    aim,
    jog,
    sendToBin,
    testLayer,
    testDwellMs,
    setTestDwellMs,
    calibrationLayers,
    calLayer,
    setCalibrationLayer,
    captures,
    capture,
    calibration,
    workItOut,
    applyCalibration,
  };
}
