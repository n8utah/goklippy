// The sort viewer's API, on the same origin (the session is an HttpOnly cookie).

async function request(method, path, body) {
  const response = await fetch(path, {
    method,
    headers: body === undefined ? {} : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
    credentials: 'same-origin',
  });
  const data = await response.json().catch(() => ({}));
  if (!response.ok) {
    const err = new Error(data.error || `Request failed (${response.status})`);
    err.status = response.status;
    throw err;
  }
  return data;
}

const get = (path) => request('GET', path);

export const getMe = () => get('/api/auth/me');
export const login = (email, password) => request('POST', '/api/auth/login', { email, password });
export const logout = () => request('POST', '/api/auth/logout', {});

export const listCollections = () => get('/api/collections');
export const getCollection = (id) => get(`/api/collections/${id}`);
export const getPlan = (id) => get(`/api/collections/${id}/plan`);
export const listParts = (id) => get(`/api/collections/${id}/parts`);
export const listRuns = (id) => get(`/api/collections/${id}/runs`);

/** { unknown, q, limit, offset } */
export function listPieces(id, { unknown, q, limit = 60, offset = 0 } = {}) {
  const params = new URLSearchParams({ limit: String(limit), offset: String(offset) });
  if (unknown) params.set('unknown', '1');
  if (q) params.set('q', q);
  return get(`/api/collections/${id}/pieces?${params}`);
}

export const photoURL = (pieceId) => `/api/photos/${pieceId}`;

/** Server-sent events for a collection; onEvent gets each { type, piece }. */
export function watchCollection(id, onEvent) {
  const source = new EventSource(`/api/collections/${id}/events`);
  source.onmessage = (e) => {
    try {
      onEvent(JSON.parse(e.data));
    } catch {
      // a malformed event is skipped
    }
  };
  return () => source.close();
}

/** Sets a collection could build, best covered first. { q, min, any, limit } */
export function listSets(id, { q, min, any, limit } = {}) {
  const params = new URLSearchParams();
  if (q) params.set('q', q);
  if (min) params.set('min', String(min));
  if (any) params.set('any', '1');
  if (limit) params.set('limit', String(limit));
  return get(`/api/collections/${id}/sets?${params}`);
}

export const getSet = (id, setNum) =>
  get(`/api/collections/${id}/sets/${encodeURIComponent(setNum)}`);

// Admin.
export const listViewers = () => get('/api/admin/viewers');
export const createViewer = (viewer) => request('POST', '/api/admin/viewers', viewer);
export const updateViewer = (id, viewer) => request('PUT', `/api/admin/viewers/${id}`, viewer);
export const deleteViewer = (id) => request('DELETE', `/api/admin/viewers/${id}`);
export const listKeys = () => get('/api/admin/keys');
export const createKey = (name) => request('POST', '/api/admin/keys', { name });
export const revokeKey = (id) => request('DELETE', `/api/admin/keys/${id}`);
export const clearCollectionPhotos = (id) =>
  request('POST', `/api/admin/collections/${id}/clear-photos`, {});
export const deleteCollection = (id) => request('DELETE', `/api/admin/collections/${id}`);
export const clearAllPhotos = () => request('POST', '/api/admin/photos/clear', {});
export const getSetsStatus = () => get('/api/admin/sets');
export const refreshSets = () => request('POST', '/api/admin/sets/refresh', {});
