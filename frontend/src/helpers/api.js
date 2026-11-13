export const API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? '';

/** MCU connect can take 30–120s (serial boot, identify, baud scan). */
export const MCU_CONNECT_TIMEOUT_MS = 130_000;

const MCU_DEBUG = import.meta.env.DEV;

export function logMCU(phase, detail = {}) {
  if (!MCU_DEBUG) {
    return;
  }
  const elapsed = detail.elapsedMs != null ? `${detail.elapsedMs}ms` : undefined;
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

export function resetMCU(sessionId) {
  return postJSON(`/api/v1/mcu/sessions/${sessionId}/reset`, {});
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

/** A sorter settings document ("machine" or "routines"): { saved, data }. */
export function getSorterSettings(name) {
  return getJSON(`/api/v1/sorter/settings/${name}`);
}

export function putSorterSettings(name, data) {
  return putJSON(`/api/v1/sorter/settings/${name}`, data);
}

/** home | start | pause | resume | stop */
export function sorterCommand(command) {
  return postJSON('/api/v1/sorter/commands', { command });
}

/** Turn a channel ("c1".."c4", "distributor") by degrees; positive is forward. */
export function moveSorterChannel(channel, degrees, rpm) {
  return postJSON(`/api/v1/sorter/channels/${channel}/move`, { degrees, rpm });
}

/** The chute read from the board now, its geometry, and every bin's aim. */
export function getSorterChute() {
  return getJSON('/api/v1/sorter/chute');
}

export function aimSorterChute(angle) {
  return postJSON('/api/v1/sorter/chute/aim', { angle });
}

export function zeroSorterChute() {
  return postJSON('/api/v1/sorter/chute/zero', {});
}

/** Blinks the light every second for a while, then leaves it off (a sorter job). */
export function blinkSorterLights(seconds = 30) {
  return postJSON('/api/v1/sorter/lights/blink', { seconds });
}

/** Switches the machine's light (the RAMPS D10 output). Returns { on, message }. */
export function setSorterLights(on) {
  return postJSON('/api/v1/sorter/lights', { on });
}

export function calibrateSorterChute(firstAngle, lastAngle, binsPerSection) {
  return postJSON('/api/v1/sorter/chute/calibrate', {
    first_angle: firstAngle,
    last_angle: lastAngle,
    bins_per_section: binsPerSection,
  });
}

export function sendToSorterBin(layer, bin) {
  return postJSON('/api/v1/sorter/distribution/send', { layer, bin });
}

export function testSorterLayer(layer, dwellMs) {
  return postJSON('/api/v1/sorter/distribution/test', { layer, dwell_ms: dwellMs });
}

export function cancelSorterJob() {
  return postJSON('/api/v1/sorter/jobs/cancel', {});
}

export function listSortingProfiles() {
  return getJSON('/api/v1/sorter/profiles');
}

/** Uploads a sorting_profile.json, sent as-is (it can be tens of MB). */
export async function importSortingProfile(text) {
  const response = await fetch(`${API_BASE_URL}/api/v1/sorter/profiles`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: text,
  });
  return parseJSON(response);
}

/** mode: keep | reset | preassign */
export function activateSortingProfile(id, mode) {
  return postJSON(`/api/v1/sorter/profiles/${id}/activate`, { mode });
}

export async function deleteSortingProfile(id) {
  const response = await fetch(`${API_BASE_URL}/api/v1/sorter/profiles/${id}`, {
    method: 'DELETE',
  });
  return parseJSON(response);
}

export function getSorterBins() {
  return getJSON('/api/v1/sorter/bins');
}

export function setSorterBin(layer, bin, categories) {
  return putJSON(`/api/v1/sorter/bins/${layer}/${bin}`, { categories });
}

export function resetSorterBins() {
  return postJSON('/api/v1/sorter/bins/reset', {});
}

/** Sends 1–8 photos of one piece to Brickognize and routes the best match (dry run). */
export async function identifySorterPiece(files, pieceMm) {
  const form = new FormData();
  for (const file of files) form.append('image', file);
  if (pieceMm) form.append('piece_mm', String(pieceMm));
  const response = await fetch(`${API_BASE_URL}/api/v1/sorter/identify`, {
    method: 'POST',
    body: form,
  });
  return parseJSON(response);
}

/** category is the part's Brickognize category. */
export function listSorterPieces({ run, status, color, category, q, limit = 50, offset = 0 } = {}) {
  const params = new URLSearchParams({ limit: String(limit), offset: String(offset) });
  if (run) params.set('run', String(run));
  if (status) params.set('status', status);
  if (color) params.set('color', color);
  if (category) params.set('category', category);
  if (q) params.set('q', q);
  return getJSON(`/api/v1/sorter/pieces?${params}`);
}

export function getSorterPiece(id) {
  return getJSON(`/api/v1/sorter/pieces/${id}`);
}

/** The camera journey that became this piece; rejects (404) when none is known. */
export function getSorterPieceJourney(id) {
  return getJSON(`/api/v1/sorter/pieces/${id}/journey`);
}

/** URL of one stored photo of a piece. */
export function sorterPieceImageURL(id, index) {
  return `${API_BASE_URL}/api/v1/sorter/pieces/${id}/images/${index}`;
}

export function listSorterRuns() {
  return getJSON('/api/v1/sorter/runs');
}

export function listSorterIncidents() {
  return getJSON('/api/v1/sorter/incidents');
}

export function getSorterStats() {
  return getJSON('/api/v1/sorter/stats');
}

/** The catalog of every part and colour found. filters: { q, color, category, run, sort, dir } */
export function listSorterCatalog(filters = {}) {
  const params = new URLSearchParams();
  Object.entries(filters).forEach(([k, v]) => v && params.set(k, v));
  return getJSON(`/api/v1/sorter/catalog?${params}`);
}

/** One part and colour with every kept photo of it and its pieces. */
export function getSorterCatalogPart(part, color) {
  return getJSON(`/api/v1/sorter/catalog/part?${new URLSearchParams({ part, color })}`);
}

/** How many piece photos are kept: { files, bytes }. */
export function getSorterPhotos() {
  return getJSON('/api/v1/sorter/photos');
}

/** Removes every piece photo; the records and the catalog stay. */
export async function clearSorterPhotos() {
  const response = await fetch(`${API_BASE_URL}/api/v1/sorter/photos`, { method: 'DELETE' });
  return parseJSON(response);
}

export function getSorterCatalogFacets() {
  return getJSON('/api/v1/sorter/catalog/facets');
}

export async function clearSorterCatalog() {
  const response = await fetch(`${API_BASE_URL}/api/v1/sorter/catalog`, { method: 'DELETE' });
  return parseJSON(response);
}

/** Adds the identified pieces in the records not in the catalog yet. */
export function fillSorterCatalog() {
  return postJSON('/api/v1/sorter/catalog/fill', {});
}

/** The picture kept of a channel when an incident opened. */
export function sorterIncidentCameraURL(id, channel) {
  return `${API_BASE_URL}/api/v1/sorter/incidents/${id}/cameras/${channel}`;
}

/** How many incident pictures are kept: { files, bytes }. */
export function getSorterIncidentPictures() {
  return getJSON('/api/v1/sorter/incidents/pictures');
}

/** Removes every incident's pictures; the incidents stay. */
export async function clearSorterIncidentPictures() {
  const response = await fetch(`${API_BASE_URL}/api/v1/sorter/incidents/pictures`, {
    method: 'DELETE',
  });
  return parseJSON(response);
}

/** Removes every resolved incident with its pictures; an open one stays. */
export async function clearSorterIncidents() {
  const response = await fetch(`${API_BASE_URL}/api/v1/sorter/incidents`, { method: 'DELETE' });
  return parseJSON(response);
}

export function resolveSorterIncident(action) {
  return postJSON('/api/v1/sorter/incident/resolve', { action });
}

export function getSorterCameras() {
  return getJSON('/api/v1/sorter/cameras');
}

export function listSorterCameraDevices() {
  return getJSON('/api/v1/sorter/cameras/devices');
}

export function getSorterCameraObservation(channel) {
  return getJSON(`/api/v1/sorter/cameras/${channel}/observation`);
}

export function captureSorterBackground(channel) {
  return postJSON(`/api/v1/sorter/cameras/${channel}/background`, {});
}

/** MJPEG stream URL for an <img>. */
/** Live MJPEG of any listed camera (by its device id), assigned or not. */
export function sorterCameraPreviewURL(device) {
  return `${API_BASE_URL}/api/v1/sorter/cameras/preview/stream?device=${encodeURIComponent(device)}`;
}

/** The sizes a camera offers: { modes: [{ width, height, fps, format }], error? }. */
export function listSorterCameraModes(device) {
  return getJSON(`/api/v1/sorter/cameras/modes?device=${encodeURIComponent(device)}`);
}

/** Finds the channel's ring in its newest frame: { measure: { ring, rim_fit, out_of_frame, … }, frame_w, frame_h }. */
export function measureSorterRing(channel) {
  return postJSON(`/api/v1/sorter/cameras/${channel}/measure`, {});
}

/** The newest pieces' journeys (still riding included), newest first. */
export function listSorterJourneys(limit = 100) {
  return getJSON(`/api/v1/sorter/journeys?limit=${limit}`);
}

/** One journey: its tracks, every frame's decision, and its moments. */
export function getSorterJourney(id) {
  return getJSON(`/api/v1/sorter/journeys/${encodeURIComponent(id)}`);
}

export function sorterJourneyFileURL(id, name) {
  return `${API_BASE_URL}/api/v1/sorter/journeys/${encodeURIComponent(id)}/files/${encodeURIComponent(name)}`;
}

/** Starts a job turning the channel all the way round, capturing it empty every step°. */
export function captureSorterRound(channel, step) {
  return postJSON(`/api/v1/sorter/cameras/${channel}/background/round`, { step });
}

/** Starts the camera search: each channel turns a little while these cameras watch. */
export function findSorterCameras(devices, degrees) {
  return postJSON('/api/v1/sorter/cameras/find', { devices, degrees });
}

/** The last camera search: { result: { proposed, scores, noise, notes, error, … } | null }. */
export function getSorterCameraFind() {
  return getJSON('/api/v1/sorter/cameras/find');
}

/** The best camera's picture before or after a channel turned, in a search. */
export function sorterCameraFindImageURL(channel, which, stamp) {
  return `${API_BASE_URL}/api/v1/sorter/cameras/find/${channel}/${which}?t=${encodeURIComponent(stamp || '')}`;
}

export function getSorterCameraPreviewStatus(device) {
  return getJSON(`/api/v1/sorter/cameras/preview/status?device=${encodeURIComponent(device)}`);
}

export function sorterCameraStreamURL(channel) {
  return `${API_BASE_URL}/api/v1/sorter/cameras/${channel}/stream`;
}

export function listSorterShakes() {
  return getJSON('/api/v1/sorter/shakes');
}

/** Run C2–C4 for the set time to empty them (into the bucket when the chute is in). */
/** Calls off the run finishing on its own (C2 empty a while): C1 feeds again. */
export function keepSorterFeeding() {
  return postJSON('/api/v1/sorter/keep-feeding', {});
}

export function clearSorterChannels() {
  return postJSON('/api/v1/sorter/channels/clear', {});
}

/** A stuck piece being worked around was taken off: watch its spot again. */
export function forgetSorterIgnored(channel) {
  return postJSON(`/api/v1/sorter/channels/${channel}/forget-ignored`, {});
}

export function shakeSorterChannel(channel, profile) {
  return postJSON(`/api/v1/sorter/channels/${channel}/shake`, { profile });
}

export function emptySorterBin(layer, bin) {
  return postJSON(`/api/v1/sorter/bins/${layer}/${bin}/empty`, {});
}

export function getSorterKits() {
  return getJSON('/api/v1/sorter/kits');
}

export function reviewSorterPiece(id, review) {
  return putJSON(`/api/v1/sorter/pieces/${id}/review`, review);
}

/** Where a piece would go now; nothing is saved. */
export function routeSorterPiece(partId, colorId, pieceMm) {
  return postJSON('/api/v1/sorter/route', {
    part_id: partId,
    color_id: colorId,
    piece_mm: pieceMm || 0,
  });
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

/** Open a WebSocket for the sorter's state; onState gets each snapshot. */
export function openSorterWS({ onState, onOpen, onClose } = {}) {
  const ws = new WebSocket(`${wsBaseUrl()}/api/v1/sorter/ws`);
  ws.onopen = () => onOpen?.();
  ws.onmessage = (event) => {
    try {
      const msg = JSON.parse(event.data);
      if (msg.type === 'state') onState?.(msg.state);
    } catch {
      // ignore malformed frames
    }
  };
  ws.onclose = (event) => onClose?.(event);
  return ws;
}

/** Open a WebSocket for live MCU session telemetry. */
export function openMCUSessionWS(
  sessionId,
  { onOpen, onSession, onReadings, onRoutines, onMotion, onError, onClose } = {},
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
      if (msg.type === 'motion') {
        onMotion?.(msg.motion);
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

export function indexMCUStepper(sessionId, oid, payload) {
  return postJSON(`/api/v1/mcu/sessions/${sessionId}/stepper/index`, { oid, ...payload });
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
