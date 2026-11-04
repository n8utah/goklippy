import {
  clampStepperSpeed,
  DEFAULT_STEPPER_SPEED,
  labelForMemberKey,
  oidsForMembers,
  stepperMemberKey,
} from './stepperGroups';

const STORAGE_KEY = 'goklippy_stepper_routines';

/** @typedef {{ id: string, forward: boolean, steps: number, speedHz: number }} RoutineSegment */
/** @typedef {{ id: string, name: string, members: string[], segments: RoutineSegment[] }} StepperRoutine */

export function newRoutineId() {
  return `rtn_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 6)}`;
}

export function newSegmentId() {
  return `seg_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 6)}`;
}

export function defaultSegments() {
  return [
    { id: newSegmentId(), forward: true, steps: 100, speedHz: DEFAULT_STEPPER_SPEED },
    { id: newSegmentId(), forward: false, steps: 10, speedHz: DEFAULT_STEPPER_SPEED },
  ];
}

function normalizeSegment(seg) {
  return {
    id: seg.id || newSegmentId(),
    forward: Boolean(seg.forward),
    steps: Math.max(1, Math.round(Number(seg.steps)) || 1),
    speedHz: clampStepperSpeed(seg.speedHz ?? DEFAULT_STEPPER_SPEED),
  };
}

function normalizeRoutine(routine) {
  const segments = Array.isArray(routine.segments) ? routine.segments.map(normalizeSegment) : defaultSegments();
  return {
    id: routine.id || newRoutineId(),
    name: routine.name || 'Untitled routine',
    members: Array.isArray(routine.members) ? routine.members : [],
    segments: segments.length > 0 ? segments : defaultSegments(),
  };
}

export function loadStepperRoutines() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) {
      return [];
    }
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) {
      return [];
    }
    return parsed.map(normalizeRoutine);
  } catch {
    return [];
  }
}

/** @param {StepperRoutine[]} routines */
export function saveStepperRoutines(routines) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(routines));
}

export function formatSegment(seg) {
  const dir = seg.forward ? 'fwd' : 'back';
  return `${seg.steps} ${dir} @ ${seg.speedHz}`;
}

export function formatRoutinePattern(routine) {
  const parts = routine.segments.map(formatSegment);
  return parts.length > 0 ? `${parts.join(' → ')} → loop` : 'No segments';
}

export function formatRoutineMembers(routine, stepperObjects) {
  if (routine.members.length === 0) {
    return 'No steppers selected';
  }
  return routine.members.map((key) => labelForMemberKey(key, stepperObjects)).join(', ');
}

/** API payload segments (no client ids). */
export function segmentsForAPI(segments) {
  return segments.map(({ forward, steps, speedHz }) => ({
    forward,
    steps: Math.max(1, Math.round(steps) || 1),
    speed_hz: clampStepperSpeed(speedHz),
  }));
}

export function oidsForRoutine(routine, stepperObjects) {
  return oidsForMembers(routine.members, stepperObjects);
}

export { stepperMemberKey, oidsForMembers, labelForMemberKey, clampStepperSpeed, DEFAULT_STEPPER_SPEED };
