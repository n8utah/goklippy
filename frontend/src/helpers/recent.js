import { useSyncExternalStore } from 'react';

// The pages and settings opened last, newest first, for search's empty state. Kept in
// this browser. Entries are { type: 'page' | 'setting', id }, where a page's id is its
// path and a setting's its registry key.

const KEY = 'goklippy-recent';
const LIMIT = 5;
const listeners = new Set();

function read() {
  try {
    const list = JSON.parse(window.localStorage.getItem(KEY));
    return Array.isArray(list) ? list : [];
  } catch {
    return [];
  }
}

let recent = typeof window !== 'undefined' ? read() : [];

/**
 * Puts an entry at the front of the recent list. `same` groups entries that replace each
 * other, such as a page's parts (going through Settings' sections keeps only the last).
 */
export function pushRecent(type, id, same = id) {
  if (recent[0]?.type === type && recent[0]?.id === id) return;
  const kept = recent.filter((r) => !(r.type === type && (r.same ?? r.id) === same));
  recent = [{ type, id, same }, ...kept].slice(0, LIMIT);
  try {
    window.localStorage.setItem(KEY, JSON.stringify(recent));
  } catch {
    // still recent for this visit
  }
  listeners.forEach((fn) => fn());
}

function subscribe(fn) {
  listeners.add(fn);
  return () => listeners.delete(fn);
}

/** The recent list, newest first. */
export function useRecent() {
  return useSyncExternalStore(subscribe, () => recent);
}
