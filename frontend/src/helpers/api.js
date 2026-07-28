export const API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? '';

async function parseJSON(response) {
  const body = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(body.error || 'Request failed');
  }
  return body;
}

export async function postJSON(path, payload, accessToken) {
  const headers = { 'Content-Type': 'application/json' };
  if (accessToken) {
    headers.Authorization = `Bearer ${accessToken}`;
  }
  const response = await fetch(`${API_BASE_URL}${path}`, {
    method: 'POST',
    headers,
    body: JSON.stringify(payload),
  });
  return parseJSON(response);
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
  return postJSON('/api/v1/mcu/connect', payload);
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
export function openMCUSessionWS(sessionId, { onOpen, onSession, onReadings, onError, onClose } = {}) {
  const ws = new WebSocket(`${wsBaseUrl()}/api/v1/mcu/sessions/${sessionId}/ws`);
  ws.onopen = () => onOpen?.();
  ws.onmessage = (event) => {
    try {
      const msg = JSON.parse(event.data);
      if (msg.type === 'session') {
        onSession?.(msg.session);
      }
      if (msg.type === 'readings') {
        onReadings?.(msg.readings);
      }
    } catch {
      // ignore malformed frames
    }
  };
  ws.onerror = () => onError?.();
  ws.onclose = (event) => onClose?.(event);
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

export function jogMCUStepper(sessionId, oid, payload) {
  return postJSON(`/api/v1/mcu/sessions/${sessionId}/stepper/jog`, { oid, ...payload });
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
