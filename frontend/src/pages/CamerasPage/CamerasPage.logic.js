import { useCallback, useEffect, useState } from 'react';

import { captureSorterBackground, getSorterCameras, listSorterCameraDevices } from '../../helpers/api';
import { useMachineSettings } from '../../helpers/sorterSettings';

export const CAMERA_CHANNELS = ['c2', 'c3', 'c4'];
export const CHANNEL_LABELS = { c2: 'C-channel 2', c3: 'C-channel 3', c4: 'Classification (C4)' };
export const TEST_DEVICE = 'lavfi:testsrc';
const STATUS_POLL_MS = 2000;

/** Zones to start from on a frame: the ring centred, Sorter V2-ish arcs. */
export function defaultZones(w, h) {
  const r = Math.min(w, h) / 2;
  return {
    frame_w: w,
    frame_h: h,
    ring: { center: { x: Math.round(w / 2), y: Math.round(h / 2) }, inner_radius: Math.round(r * 0.45), outer_radius: Math.round(r * 0.9), exit_radius: Math.round(r * 0.85) },
    drop: { start: 200, end: 260 },
    exit: { start: 330, end: 350 },
    precise_width: 15,
  };
}

export function useCamerasPageLogic() {
  const [settings, setSettings, syncError] = useMachineSettings();
  const [channel, setChannel] = useState('c4');
  const [devices, setDevices] = useState({ list: [], error: '', loading: true });
  const [status, setStatus] = useState(null);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');

  const cams = settings.cameras;

  useEffect(() => {
    let cancelled = false;
    const poll = () =>
      getSorterCameras().then(
        (data) => {
          if (!cancelled) setStatus(data);
        },
        () => {},
      );
    poll();
    const timer = setInterval(poll, STATUS_POLL_MS);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, []);

  const findDevices = useCallback(async () => {
    setDevices((d) => ({ ...d, loading: true }));
    try {
      const data = await listSorterCameraDevices();
      setDevices({ list: data.devices, error: data.error || '', loading: false });
    } catch (err) {
      setDevices({ list: [], error: err.message, loading: false });
    }
  }, []);

  // The first look happens on opening the page; the button looks again.
  useEffect(() => {
    let cancelled = false;
    listSorterCameraDevices().then(
      (data) => {
        if (!cancelled) setDevices({ list: data.devices, error: data.error || '', loading: false });
      },
      (err) => {
        if (!cancelled) setDevices({ list: [], error: err.message, loading: false });
      },
    );
    return () => {
      cancelled = true;
    };
  }, []);

  const setCameras = (patch) => setSettings((prev) => ({ ...prev, cameras: { ...prev.cameras, ...patch } }));

  const setDevice = (device) => setCameras({ devices: { ...cams.devices, [channel]: device } });

  const chStatus = status?.channels?.[channel];
  const zones = cams.zones?.[channel] ?? null;

  const setZones = (next) => setCameras({ zones: { ...cams.zones, [channel]: next } });

  const startZones = () => {
    const w = chStatus?.frame_w || cams.width || 640;
    const h = chStatus?.frame_h || cams.height || 480;
    setZones(defaultZones(w, h));
  };

  /** Update one field by path, e.g. ['ring', 'center', 'x']. */
  const setZoneField = (path, value) => {
    const next = structuredClone(zones);
    let node = next;
    for (const key of path.slice(0, -1)) node = node[key];
    node[path[path.length - 1]] = value;
    setZones(next);
  };

  const pickCenter = ({ x, y }) => {
    if (!zones) return;
    setZoneField(['ring', 'center'], { x, y });
  };

  const captureBackground = async () => {
    setError('');
    setNotice('');
    try {
      await captureSorterBackground(channel);
      setNotice('Captured. Pieces are now whatever differs from this picture, so keep the channel clear when you capture.');
    } catch (err) {
      setError(err.message);
    }
  };

  return {
    simulated: status?.simulated,
    channel,
    setChannel,
    cams,
    setCameras,
    devices,
    findDevices,
    setDevice,
    chStatus,
    zones,
    startZones,
    setZoneField,
    pickCenter,
    captureBackground,
    error: error || syncError,
    notice,
  };
}
