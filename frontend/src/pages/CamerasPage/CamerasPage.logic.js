import { useCallback, useEffect, useState } from 'react';

import {
  captureSorterBackground,
  getSorterCameras,
  listSorterCameraDevices,
} from '../../helpers/api';
import { useMachineSettings } from '../../helpers/sorterSettings';

export const CAMERA_CHANNELS = ['c2', 'c3', 'c4'];
export const CHANNEL_LABELS = { c2: 'C-channel 2', c3: 'C-channel 3', c4: 'Classification (C4)' };
export const TEST_DEVICE = 'lavfi:testsrc';
/**
 * Sizes every camera can be opened at, largest first. Each camera must offer the
 * chosen one (Check sizes on Setup). Smaller is less work for the laptop.
 */
export const CAMERA_SIZES = [
  [1280, 720],
  [960, 540],
  [848, 480],
  [800, 600],
  [640, 480],
  [352, 288],
  [320, 240],
];
const STATUS_POLL_MS = 2000;

/** Zones to start from on a frame: the ring centred, Sorter V2-ish arcs. */
export function defaultZones(w, h) {
  const r = Math.min(w, h) / 2;
  return {
    frame_w: w,
    frame_h: h,
    ring: {
      center: { x: Math.round(w / 2), y: Math.round(h / 2) },
      inner_radius: Math.round(r * 0.45),
      outer_radius: Math.round(r * 0.9),
      exit_radius: Math.round(r * 0.85),
    },
    drop: { start: 200, end: 260 },
    exit: { start: 330, end: 350 },
    precise_width: 15,
  };
}

const NO_DEVICES = { list: [], ffmpeg: '', version: '', warning: '', error: '', loading: false };

function devicesFrom(data) {
  return {
    list: data.devices ?? [],
    ffmpeg: data.ffmpeg ?? '',
    version: data.version ?? '',
    warning: data.warning ?? '',
    error: data.error ?? '',
    loading: false,
  };
}

/** How a device reads in the page: its name, and what tells it apart from a twin. */
export function deviceLabel(device) {
  return device.hint ? `${device.name} (${device.hint})` : device.name;
}

export function useCamerasPageLogic() {
  const [settings, setSettings, syncError] = useMachineSettings();
  // 'setup' picks which camera is where; a channel tab draws that channel's zones.
  const [view, setView] = useState('setup');
  const channel = view === 'setup' ? 'c2' : view;
  const [devices, setDevices] = useState({ ...NO_DEVICES, loading: true });
  const [previewing, setPreviewing] = useState({}); // device id -> shown
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
      setDevices(devicesFrom(await listSorterCameraDevices()));
    } catch (err) {
      setDevices({ ...NO_DEVICES, error: err.message });
    }
  }, []);

  // The first look happens on opening the page; the button looks again.
  useEffect(() => {
    let cancelled = false;
    listSorterCameraDevices().then(
      (data) => {
        if (!cancelled) setDevices(devicesFrom(data));
      },
      (err) => {
        if (!cancelled) setDevices({ ...NO_DEVICES, error: err.message });
      },
    );
    return () => {
      cancelled = true;
    };
  }, []);

  const setCameras = (patch) =>
    setSettings((prev) => ({ ...prev, cameras: { ...prev.cameras, ...patch } }));

  const setDevice = (id) => {
    const found = devices.list.find((d) => d.id === id);
    if (found) {
      assign(found, channel);
      return;
    }
    setCameras({
      devices: { ...cams.devices, [channel]: id },
      deviceNames: {
        ...cams.deviceNames,
        [channel]: id === TEST_DEVICE ? 'ffmpeg test picture' : id,
      },
    });
  };

  /**
   * Puts a camera over a channel ('' for not used). A camera serves one channel, and a
   * channel has one camera, so it comes off any other channel. Its label is kept so a
   * camera that's unplugged can still be named.
   */
  const assign = (device, ch) => {
    const next = { ...cams.devices };
    const names = { ...cams.deviceNames };
    for (const c of CAMERA_CHANNELS) {
      if (next[c] === device.id || next[c] === device.name) {
        next[c] = '';
        delete names[c];
      }
    }
    if (ch) {
      next[ch] = device.id;
      names[ch] = deviceLabel(device);
    }
    setCameras({ devices: next, deviceNames: names });
  };

  /** Which channel a device is on, matching saves made by name before paths. */
  const channelOf = (device) =>
    CAMERA_CHANNELS.find(
      (c) =>
        cams.devices?.[c] && (cams.devices[c] === device.id || cams.devices[c] === device.name),
    ) ?? '';

  // Assigned cameras the last listing didn't find: unplugged, or moved to another USB
  // port (the id follows the port).
  const listed = !devices.loading && !devices.error;
  const missing = CAMERA_CHANNELS.filter((c) => {
    const id = cams.devices?.[c];
    return (
      listed && id && id !== TEST_DEVICE && !devices.list.some((d) => d.id === id || d.name === id)
    );
  }).map((c) => ({ channel: c, label: cams.deviceNames?.[c] || cams.devices[c] }));

  const togglePreview = (id) => setPreviewing((p) => ({ ...p, [id]: !p[id] }));

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
      setNotice(
        'Captured. Pieces are now whatever differs from this picture, so keep the channel clear when you capture.',
      );
    } catch (err) {
      setError(err.message);
    }
  };

  return {
    simulated: status?.simulated,
    view,
    setView,
    channel,
    assign,
    channelOf,
    missing,
    previewing,
    togglePreview,
    statusOf: (c) => status?.channels?.[c],
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
