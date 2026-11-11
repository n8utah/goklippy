const STORAGE_KEY = 'goklippy_stepper_groups';

export const DEFAULT_STEPPER_SPEED = 500;
export const SLOW_FACTOR = 0.8;
export const FAST_FACTOR = 1.25;
export const MIN_STEPPER_SPEED = 1;
export const MAX_STEPPER_SPEED = 50000;

/** @typedef {{ id: string, name: string, members: string[], speeds?: Record<string, number> }} StepperGroup */

export function clampStepperSpeed(value) {
  return Math.min(
    MAX_STEPPER_SPEED,
    Math.max(MIN_STEPPER_SPEED, Math.round(value) || MIN_STEPPER_SPEED),
  );
}

export function scaleStepperSpeed(value, factor) {
  return clampStepperSpeed(value * factor);
}

export function loadStepperGroups() {
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
    speeds: group.speeds && typeof group.speeds === 'object' ? group.speeds : {},
  };
}

/** @param {StepperGroup[]} groups */
export function saveStepperGroups(groups) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(groups));
}

export function newGroupId() {
  return `grp_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 6)}`;
}

/** Stable key for a stepper object (prefer board slot id). */
export function stepperMemberKey(obj) {
  return obj.slot_id || obj.section || String(obj.oid);
}

/** Resolve group member keys to live stepper OIDs. */
export function oidsForMembers(memberKeys, stepperObjects) {
  const keys = new Set(memberKeys);
  return stepperObjects.filter((obj) => keys.has(stepperMemberKey(obj))).map((obj) => obj.oid);
}

export function labelForMemberKey(key, stepperObjects) {
  const obj = stepperObjects.find((item) => stepperMemberKey(item) === key);
  if (!obj) {
    return key;
  }
  return obj.label || obj.slot_id || obj.section || key;
}

/** Speed configured for a group member (falls back to default). */
export function speedForMember(group, memberKey) {
  return clampStepperSpeed(group.speeds?.[memberKey] ?? DEFAULT_STEPPER_SPEED);
}

/** Build start commands for each live group member with its configured speed. */
export function startPlanForGroup(group, stepperObjects, { forward = true } = {}) {
  const keys = new Set(group.members);
  return stepperObjects
    .filter((obj) => keys.has(stepperMemberKey(obj)))
    .map((obj) => ({
      oid: obj.oid,
      forward,
      speedHz: speedForMember(group, stepperMemberKey(obj)),
    }));
}

/** Scale all configured speeds in a group proportionally. */
export function scaleGroupSpeeds(group, factor) {
  const speeds = { ...(group.speeds || {}) };
  for (const key of group.members) {
    speeds[key] = scaleStepperSpeed(speedForMember(group, key), factor);
  }
  return { ...group, speeds };
}
