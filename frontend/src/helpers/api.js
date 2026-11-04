export const API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? '';

/** MCU connect can take 30–120s (serial boot, identify, baud scan). */
export const MCU_CONNECT_TIMEOUT_MS = 130_000;

const MCU_DEBUG = import.meta.env.DEV;

export function logMCU(phase, detail = {}) {
  if (!MCU_DEBUG) {
    return;
  }
  const elapsed =
    detail.elapsedMs != null ? `${detail.elapsedMs}ms` : undefined;
  // eslint-disable-next-line no-console
  console.info('[goklippy mcu]', phase, elapsed ?? '', detail);
}

async function parseJSON(response) {
  const body = await response.json().catch(() => ({}));
  if (!response.ok) {
    const message =
      body.error ||
      (response.status >= 500
        ? `Server error (${response.status}) — check backend logs`
        : 'Request failed');
    throw new Error(message);
  }
  return body;
}

export async function postJSON(path, payload, accessToken, options = {}) {
  const headers = { 'Content-Type': 'application/json' };
  if (accessToken) {
    headers.Authorization = `Bearer ${accessToken}`;
  }
  const { timeoutMs, label } = options;
  const controller = timeoutMs ? new AbortController() : null;
  const timer =
    controller &&
    setTimeout(() => {
      controller.abort();
    }, timeoutMs);

  const started = performance.now();
  if (label) {
    logMCU(`${label} → request`, { path, payload });
  }

  try {
    const response = await fetch(`${API_BASE_URL}${path}`, {
      method: 'POST',
      headers,
      body: JSON.stringify(payload),
      signal: controller?.signal,
    });
    const body = await parseJSON(response);
    if (label) {
      logMCU(`${label} ← ok`, { elapsedMs: Math.round(performance.now() - started) });
    }
    return body;
  } catch (err) {
    if (label) {
      logMCU(`${label} ← failed`, {
        elapsedMs: Math.round(performance.now() - started),
        error: err.message,
        aborted: err.name === 'AbortError',
      });
    }
    if (err.name === 'AbortError') {
      throw new Error(
        `Connect timed out after ${Math.round(timeoutMs / 1000)}s — check backend terminal for serial progress`,
      );
    }
    throw err;
  } finally {
    if (timer) {
      clearTimeout(timer);
    }
  }
}

export async function putJSON(path, payload, accessToken) {
  const headers = { 'Content-Type': 'application/json' };
  if (accessToken) {
    headers.Authorization = `Bearer ${accessToken}`;
  }
  const response = await fetch(`${API_BASE_URL}${path}`, {
    method: 'PUT',
    headers,
    body: JSON.stringify(payload),
  });
  return parseJSON(response);
}

export async function getJSON(path, accessToken) {
  const headers = {};
  if (accessToken) {
    headers.Authorization = `Bearer ${accessToken}`;
  }
  const response = await fetch(`${API_BASE_URL}${path}`, { headers });
  return parseJSON(response);
}

export function login(payload) {
  return postJSON('/api/v1/auth/login', payload);
}

export function refresh(refreshToken) {
  return postJSON('/api/v1/auth/refresh', { refresh_token: refreshToken });
}

export function logout(refreshToken) {
  return postJSON('/api/v1/auth/logout', { refresh_token: refreshToken });
}

export function getMe(accessToken) {
  return getJSON('/api/v1/auth/me', accessToken);
}

export function scanMCUPorts() {
  return postJSON('/api/v1/mcu/scan', {});
}

export function connectMCU(payload) {
  return postJSON('/api/v1/mcu/connect', payload, undefined, {
    timeoutMs: MCU_CONNECT_TIMEOUT_MS,
    label: 'connect',
  });
}

export function disconnectMCU(sessionId) {
  return postJSON('/api/v1/mcu/disconnect', { session_id: sessionId });
}

export function getMCUSession(sessionId) {
  return getJSON(`/api/v1/mcu/sessions/${sessionId}`);
}

export function listMCUSessions() {
  return getJSON('/api/v1/mcu/sessions');
}

export function getMCUReadings(sessionId) {
  return getJSON(`/api/v1/mcu/sessions/${sessionId}/readings`);
}

function wsBaseUrl() {
  if (API_BASE_URL) {
    const url = new URL(API_BASE_URL, window.location.origin);
    const proto = url.protocol === 'https:' ? 'wss:' : 'ws:';
    return `${proto}//${url.host}`;
  }
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  return `${proto}//${window.location.host}`;
}

/** Open a WebSocket for live MCU session telemetry. */
export function openMCUSessionWS(
  sessionId,
  { onOpen, onSession, onReadings, onRoutines, onError, onClose } = {},
) {
  const url = `${wsBaseUrl()}/api/v1/mcu/sessions/${sessionId}/ws`;
  logMCU('websocket opening', { sessionId, url });
  const ws = new WebSocket(url);
  ws.onopen = () => {
    logMCU('websocket open', { sessionId });
    onOpen?.();
  };
  ws.onmessage = (event) => {
    try {
      const msg = JSON.parse(event.data);
      if (msg.type === 'session') {
        onSession?.(msg.session);
      }
      if (msg.type === 'readings') {
        onReadings?.(msg.readings);
      }
      if (msg.type === 'routines') {
        onRoutines?.(msg.routines || []);
      }
    } catch {
      // ignore malformed frames
    }
  };
  ws.onerror = () => {
    logMCU('websocket error', { sessionId });
    onError?.();
  };
  ws.onclose = (event) => {
    logMCU('websocket closed', { sessionId, code: event.code, reason: event.reason });
    onClose?.(event);
  };
  return ws;
}

export function stopMCUStepper(sessionId, oid) {
  return postJSON(`/api/v1/mcu/sessions/${sessionId}/stepper/stop`, { oid });
}

export function setMCUDigitalOut(sessionId, oid, value) {
  return postJSON(`/api/v1/mcu/sessions/${sessionId}/digital-out`, { oid, value });
}

export function setMCUServo(sessionId, oid, angle) {
  return postJSON(`/api/v1/mcu/sessions/${sessionId}/servo`, { oid, angle });
}

export function checkMCUPCA9685(sessionId) {
  return postJSON(`/api/v1/mcu/sessions/${sessionId}/pca9685/check`, {});
}

export function jogMCUStepper(sessionId, oid, payload) {
  return postJSON(`/api/v1/mcu/sessions/${sessionId}/stepper/jog`, { oid, ...payload });
}

export function startMCUStepperRoutine(sessionId, oid, segments) {
  return postJSON(`/api/v1/mcu/sessions/${sessionId}/stepper/routine`, { oid, segments });
}

export function getMCUPorts() {
  return getJSON('/api/v1/mcu/ports');
}

export function listKlipperConfigFiles() {
  return getJSON('/api/v1/klipper-config/files');
}

export function loadKlipperConfig(path) {
  return postJSON('/api/v1/klipper-config/load', { path });
}

export function getKlipperWorkspace() {
  return getJSON('/api/v1/klipper-config/workspace');
}

export function saveKlipperConfig() {
  return postJSON('/api/v1/klipper-config/save', {});
}

export function updateKlipperSection(name, options) {
  return putJSON(`/api/v1/klipper-config/sections/${encodeURIComponent(name)}`, { options });
}

export function deleteKlipperSection(name) {
  return fetch(`${API_BASE_URL}/api/v1/klipper-config/sections/${encodeURIComponent(name)}`, {
    method: 'DELETE',
    headers: { 'Content-Type': 'application/json' },
  }).then(async (res) => {
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      throw new Error(data.error || res.statusText);
    }
    return data;
  });
}

export function updateKlipperRaw(content) {
  return putJSON('/api/v1/klipper-config/raw', { content });
}

export function getBoardProfile(id) {
  return getJSON(`/api/v1/board-profiles/${id}`);
}

export function listBoardProfiles() {
  return getJSON('/api/v1/board-profiles');
}

export function getBoardProfileEditor(id) {
  return getJSON(`/api/v1/board-profiles/${id}/editor`);
}

export function updateBoardProfileHotspots(id, hotspots) {
  return putJSON(`/api/v1/board-profiles/${id}/hotspots`, { hotspots });
}

export function getBoardDiagram(id) {
  return getJSON(`/api/v1/board-profiles/${id}/diagram`);
}

/** Resolve a board diagram image URL from API metadata (supports local paths and remote URLs). */
export function resolveBoardImageUrl(diagram) {
  if (!diagram?.image_url) {
    return null;
  }
  if (/^https?:\/\//i.test(diagram.image_url)) {
    return diagram.image_url;
  }
  return `${API_BASE_URL}${diagram.image_url}`;
}

export function boardProfileImageURL(id) {
  return `${API_BASE_URL}/api/v1/board-profiles/${id}/image`;
}
