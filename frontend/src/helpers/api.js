export const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080';

export async function postJSON(path, payload) {
  const response = await fetch(`${API_BASE_URL}${path}`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload),
  });

  const body = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(body.error || 'Request failed');
  }
  return body;
}

export async function getJSON(path, accessToken) {
  const response = await fetch(`${API_BASE_URL}${path}`, {
    method: 'GET',
    headers: {
      Authorization: `Bearer ${accessToken}`,
    },
  });

  const body = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(body.error || 'Request failed');
  }
  return body;
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
