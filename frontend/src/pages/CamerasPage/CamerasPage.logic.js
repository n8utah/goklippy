import { useCallback, useEffect, useState } from 'react';

import {
  cancelSorterJob,
  captureSorterBackground,
  captureSorterRound,
  measureSorterRing,
  findSorterCameras,
  getSorterCameraFind,
  getSorterCameras,
  listSorterCameraDevices,
} from '../../helpers/api';
import { useMachineSettings } from '../../helpers/sorterSettings';
import { useSorterState } from '../../helpers/sorterState';

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
/**
 * Larger sizes, for one channel's own camera (a 4K camera over C4). Above
 * TRACK_WIDTH it's tracked on a copy scaled to that width, and the full-size pictures
 * are what's sent to Brickognize.
 */
export const HIGH_RES_SIZES = [
  [3840, 2160],
  [2688, 1944],
  [2592, 1944],
  [2560, 1440],
  [1920, 1080],
];
export const TRACK_WIDTH = 1280;

/** The size a channel's camera opens at: its own, or every camera's. */
export function cameraSizeFor(cams, channel) {
  const own = channel && cams.sizes?.[channel];
  return own?.width > 0 && own?.height > 0
    ? { width: own.width, height: own.height, own: true }
    : { width: cams.width, height: cams.height, own: false };
}
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

/**
 * Machine › Cameras. `view` is the tab from the URL: 'setup' picks which camera is where;
 * a channel ('c2'…'c4') draws that channel's zones. `setView` goes to another tab.
 */
export function useCamerasPageLogic(view, setView) {
  const [settings, setSettings, syncError] = useMachineSettings();
  const channel = view === 'setup' ? 'c2' : view;
  const [devices, setDevices] = useState({ ...NO_DEVICES, loading: true });
  const [previewing, setPreviewing] = useState({}); // device id -> shown
  const [roundStep, setRoundStep] = useState(1);
  const sorter = useSorterState();
  const [status, setStatus] = useState(null);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');

  const cams = settings.cameras;
  // How far the channel's rotor turns before it looks the same again: one section.
  const sections = view === 'c4' ? settings.platter?.sectors : settings.channels?.[view]?.sections;
  const roundPeriod = sections > 0 ? 360 / sections : 360;

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
  const assign = (device, ch) => assignMany([[device, ch]]);

  /** assign for several cameras at once: [[device, channel], …]. */
  const assignMany = (pairs) => {
    const next = { ...cams.devices };
    const names = { ...cams.deviceNames };
    for (const [device, ch] of pairs) {
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

  /** Gives this channel's camera its own size; null goes back to every camera's. */
  const setChannelSize = (size) => {
    const sizes = { ...cams.sizes };
    if (size) sizes[channel] = size;
    else delete sizes[channel];
    setCameras({ sizes });
  };

  const startZones = () => {
    const w = chStatus?.frame_w || cams.width || 640;
    const h = chStatus?.frame_h || cams.height || 480;
    setZones(defaultZones(w, h));
  };

  /**
   * Finds the ring in the channel's picture and puts it in the zones, keeping the drop
   * and exit arcs (or the defaults on a first measure).
   */
  const [measuring, setMeasuring] = useState(false);
  const measureRing = async () => {
    setError('');
    setNotice('');
    setMeasuring(true);
    try {
      const { measure: m, frame_w: w, frame_h: h } = await measureSorterRing(channel);
      const base = zones ?? defaultZones(w, h);
      setZones({ ...base, frame_w: w, frame_h: h, ring: m.ring });
      const out = Math.round(m.out_of_frame * 100);
      const fit = Math.round(m.rim_fit * 100);
      const parts = [
        `Ring measured: center (${m.ring.center.x}, ${m.ring.center.y}), ${m.ring.inner_radius} to ${m.ring.outer_radius} px.`,
        m.hole_radius > 0
          ? ''
          : 'The hole in the middle wasn’t found, so the inner edge is a guess.',
        fit < 70
          ? `Only ${fit}% of the rim’s edge points fit the circle; check it on the picture.`
          : '',
        out >= 10
          ? `${out}% of the rim is outside the picture: pieces riding there can’t be seen. Move the camera so the whole bowl is in view.`
          : '',
        'Check the blue drop and orange exit sit where pieces land and leave.',
      ];
      setNotice(parts.filter(Boolean).join(' '));
    } catch (err) {
      setError(err.message);
    } finally {
      setMeasuring(false);
    }
  };

  // The channel's setup, step by step, from its live status.
  const setupSteps = [
    { label: 'Camera picked and running', done: Boolean(chStatus?.camera?.running) },
    { label: 'Ring measured or drawn', done: Boolean(chStatus?.zones_set) },
    { label: 'Drop (blue) and exit (orange) where pieces land and leave', done: null },
    {
      label: 'Captured empty across a section',
      done: (chStatus?.views ?? 0) > 0,
      note: chStatus?.has_background && !chStatus?.views ? 'one picture only' : '',
    },
  ];

  /** Update one field by path, e.g. ['ring', 'center', 'x']. */
  const setZoneField = (path, value) => {
    const next = structuredClone(zones);
    let node = next;
    for (const key of path.slice(0, -1)) node = node[key];
    node[path[path.length - 1]] = value;
    setZones(next);
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

  /** Starts the job turning the channel round, capturing it empty every roundStep°. */
  const captureRound = async () => {
    setError('');
    setNotice('');
    try {
      await captureSorterRound(channel, roundStep);
      setNotice(
        `Turning ${channel.toUpperCase()} all the way round, a picture every ${roundStep}°. Keep it clear and don't feed it until it's done.`,
      );
    } catch (err) {
      setError(err.message);
    }
  };
  const roundJob = sorter.state?.job?.name?.startsWith('Capture ') ? sorter.state.job : null;

  return {
    simulated: status?.simulated,
    roundStep,
    setRoundStep,
    roundPeriod,
    captureRound,
    roundJob,
    view,
    setView,
    channel,
    assign,
    assignMany,
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
    measureRing,
    measuring,
    setupSteps,
    setZoneField,
    setZones,
    setChannelSize,
    captureBackground,
    error: error || syncError,
    notice,
  };
}

export const FIND_JOB = 'Find which camera is where';
export const ALL_CHANNELS = ['c1', 'c2', 'c3', 'c4'];
// The laptop's own camera isn't part of the machine; leave it unticked to start.
const BUILT_IN = /laptop|integrated|built-?in|facetime|front|rear/i;

/**
 * The camera search: which cameras to watch, starting it, following the job on the
 * sorter's live state, and applying what it proposes.
 */
export function useCameraFinder(logic) {
  const { state, connected } = useSorterState();
  const [unpicked, setUnpicked] = useState(null); // device ids left out; null = default
  const [degrees, setDegrees] = useState(15);
  const [result, setResult] = useState(null);
  const [error, setError] = useState('');

  const list = logic.devices.list;
  const skipped = unpicked ?? new Set(list.filter((d) => BUILT_IN.test(d.name)).map((d) => d.id));
  const picked = list.filter((d) => !skipped.has(d.id));
  const toggle = (id) => {
    const next = new Set(skipped);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    setUnpicked(next);
  };

  const running = state?.job?.name === FIND_JOB;
  const finishedAt = state?.last_job?.name === FIND_JOB ? state.last_job.at : '';

  // Fetch the result on opening and whenever a search finishes.
  useEffect(() => {
    let cancelled = false;
    getSorterCameraFind().then(
      (data) => {
        if (!cancelled) setResult(data.result);
      },
      () => {},
    );
    return () => {
      cancelled = true;
    };
  }, [finishedAt]);

  const start = async () => {
    setError('');
    try {
      await findSorterCameras(
        picked.map((d) => d.id),
        degrees,
      );
    } catch (err) {
      setError(err.message);
    }
  };

  const cancel = () => cancelSorterJob().catch((err) => setError(err.message));

  /** Puts each proposed camera over its channel. */
  const apply = () => {
    const pairs = Object.entries(result?.proposed ?? {})
      .map(([ch, id]) => [list.find((d) => d.id === id), ch])
      .filter(([d]) => d);
    logic.assignMany(pairs);
  };

  const s = state?.state;
  const blocked = !connected
    ? 'Connecting to the backend…'
    : logic.simulated
      ? 'The machine is simulated: there are no cameras to find.'
      : !state?.hardware?.connected
        ? 'Connect the board under Machine › Board first: the search turns the channels.'
        : s === 'running' || s === 'homing'
          ? 'Pause or stop the sorter first.'
          : state?.job && !running
            ? 'Wait for the sorter’s current task to finish.'
            : picked.length === 0
              ? 'Tick at least one camera.'
              : '';

  return {
    picked,
    isPicked: (id) => !skipped.has(id),
    toggle,
    degrees,
    setDegrees,
    running,
    job: running ? state.job : null,
    result,
    stamp: result?.finished ?? '',
    start,
    cancel,
    apply,
    blocked,
    error,
  };
}
