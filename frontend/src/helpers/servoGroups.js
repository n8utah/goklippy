const STORAGE_KEY = 'goklippy_servo_groups';

export const DEFAULT_SERVO_ANGLE = 90;
export const MIN_SERVO_ANGLE = 0;
export const MAX_SERVO_ANGLE = 180;
export const SLOW_FACTOR = 0.8;
export const FAST_FACTOR = 1.25;

/** @typedef {{ id: string, name: string, members: string[], angles?: Record<string, number> }} ServoGroup */

export function clampServoAngle(value) {
  return Math.min(MAX_SERVO_ANGLE, Math.max(MIN_SERVO_ANGLE, Math.round(value) || MIN_SERVO_ANGLE));
}

export function scaleServoAngle(value, factor) {
  return clampServoAngle(value * factor);
}

export function flipServoAngle(value) {
  return clampServoAngle(MAX_SERVO_ANGLE - value);
}

export function loadServoGroups() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) {
      return [];
    }
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) {
      return [];
    }
    return parsed.map(normalizeGroup);
  } catch {
    return [];
  }
}

function normalizeGroup(group) {
  return {
    ...group,
    members: Array.isArray(group.members) ? group.members : [],
    angles: group.angles && typeof group.angles === 'object' ? group.angles : {},
  };
}

/** @param {ServoGroup[]} groups */
export function saveServoGroups(groups) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(groups));
}

export function newGroupId() {
  return `svgrp_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 6)}`;
}

/** Stable key for a servo object (prefer board slot id). */
export function servoMemberKey(obj) {
  return obj.slot_id || obj.section || String(obj.oid);
}

/** Resolve group member keys to live servo OIDs. */
export function oidsForMembers(memberKeys, servoObjects) {
  const keys = new Set(memberKeys);
  return servoObjects.filter((obj) => keys.has(servoMemberKey(obj))).map((obj) => obj.oid);
}

export function labelForMemberKey(key, servoObjects) {
  const obj = servoObjects.find((item) => servoMemberKey(item) === key);
  if (!obj) {
    return key;
  }
  return obj.label || obj.slot_id || obj.section || key;
}

/** Angle configured for a group member (falls back to default). */
export function angleForMember(group, memberKey) {
  return clampServoAngle(group.angles?.[memberKey] ?? DEFAULT_SERVO_ANGLE);
}

/** Build apply commands for each live group member with its configured angle. */
export function applyPlanForGroup(group, servoObjects) {
  const keys = new Set(group.members);
  return servoObjects
    .filter((obj) => keys.has(servoMemberKey(obj)))
    .map((obj) => ({
      oid: obj.oid,
      angle: angleForMember(group, servoMemberKey(obj)),
    }));
}

/** Scale all configured angles in a group proportionally. */
export function scaleGroupAngles(group, factor) {
  const angles = { ...(group.angles || {}) };
  for (const key of group.members) {
    angles[key] = scaleServoAngle(angleForMember(group, key), factor);
  }
  return { ...group, angles };
}

/** Flip all configured angles in a group (mirror about 90°). */
export function flipGroupAngles(group) {
  const angles = { ...(group.angles || {}) };
  for (const key of group.members) {
    angles[key] = flipServoAngle(angleForMember(group, key));
  }
  return { ...group, angles };
}
