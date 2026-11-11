import { useSyncExternalStore } from 'react';

// The color theme: 'light', 'dark' or 'system' (follow the computer). The choice is kept in
// this browser; the resolved theme is a `dark` class on <html>, which switches every color
// token (styles/globals.css).

const KEY = 'goklippy-theme';
const media =
  typeof window !== 'undefined' ? window.matchMedia('(prefers-color-scheme: dark)') : null;
const listeners = new Set();

function readChoice() {
  try {
    const v = window.localStorage.getItem(KEY);
    return v === 'light' || v === 'dark' ? v : 'system';
  } catch {
    return 'system';
  }
}

let choice = typeof window !== 'undefined' ? readChoice() : 'system';

const resolve = (c) => (c === 'system' ? (media?.matches ? 'dark' : 'light') : c);

function apply() {
  document.documentElement.classList.toggle('dark', resolve(choice) === 'dark');
  listeners.forEach((fn) => fn());
}

/** Applies the saved theme; call once before the app renders, so it doesn't flash. */
export function startTheme() {
  apply();
  media?.addEventListener('change', () => {
    if (choice === 'system') apply();
  });
}

/** Sets the theme choice: 'light', 'dark' or 'system'. */
export function setTheme(next) {
  choice = next === 'light' || next === 'dark' ? next : 'system';
  try {
    if (choice === 'system') window.localStorage.removeItem(KEY);
    else window.localStorage.setItem(KEY, choice);
  } catch {
    // A browser that won't store it still switches for this visit.
  }
  apply();
}

function subscribe(fn) {
  listeners.add(fn);
  return () => listeners.delete(fn);
}

const snapshot = () => `${choice}:${resolve(choice)}`;

/** The theme choice and what it resolves to now: { theme, resolved, setTheme }. */
export function useTheme() {
  const [theme, resolved] = useSyncExternalStore(subscribe, snapshot).split(':');
  return { theme, resolved, setTheme };
}
