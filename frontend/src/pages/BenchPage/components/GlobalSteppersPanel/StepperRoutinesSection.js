import { useEffect, useMemo, useState } from 'react';

import { useMCUSession } from '../../../../context/MCUSessionContext';
import {
  clampStepperSpeed,
  DEFAULT_STEPPER_SPEED,
  defaultSegments,
  formatRoutineMembers,
  formatRoutinePattern,
  labelForMemberKey,
  loadStepperRoutines,
  newRoutineId,
  newSegmentId,
  oidsForRoutine,
  saveStepperRoutines,
  stepperMemberKey,
} from '../../../../helpers/stepperRoutines';

import styles from './GlobalSteppersPanel.module.css';
import appStyles from '../../../../styles/app.module.css';

const StepperRoutinesSection = ({ connected, busy, onRun }) => {
  const {
    stepperObjects,
    stepperRoutineOids,
    stepperRoutineErrors,
    startStepperRoutineForMembers,
    stopSteppers,
  } = useMCUSession();

  const [routines, setRoutines] = useState(() => loadStepperRoutines());
  const [draft, setDraft] = useState(null);

  useEffect(() => {
    saveStepperRoutines(routines);
  }, [routines]);

  const memberOptions = useMemo(
    () =>
      stepperObjects.map((obj) => ({
        key: stepperMemberKey(obj),
        label: labelForMemberKey(stepperMemberKey(obj), stepperObjects),
      })),
    [stepperObjects],
  );

  const beginCreate = () => {
    setDraft({
      id: '',
      name: '',
      members: memberOptions.length === 1 ? [memberOptions[0].key] : [],
      segments: defaultSegments(),
    });
  };

  const beginEdit = (routine) => {
    setDraft({
      ...routine,
      segments: routine.segments.map((seg) => ({ ...seg })),
    });
  };

  const cancelDraft = () => setDraft(null);

  const toggleMember = (key) => {
    setDraft((prev) => {
      if (!prev) return prev;
      const members = prev.members.includes(key)
        ? prev.members.filter((item) => item !== key)
        : [...prev.members, key];
      return { ...prev, members };
    });
  };

  const updateSegment = (segId, patch) => {
    setDraft((prev) => {
      if (!prev) return prev;
      return {
        ...prev,
        segments: prev.segments.map((seg) => (seg.id === segId ? { ...seg, ...patch } : seg)),
      };
    });
  };

  const addSegment = () => {
    setDraft((prev) => {
      if (!prev) return prev;
      return {
        ...prev,
        segments: [
          ...prev.segments,
          { id: newSegmentId(), forward: true, steps: 50, speedHz: DEFAULT_STEPPER_SPEED },
        ],
      };
    });
  };

  const removeSegment = (segId) => {
    setDraft((prev) => {
      if (!prev || prev.segments.length <= 1) return prev;
      return { ...prev, segments: prev.segments.filter((seg) => seg.id !== segId) };
    });
  };

  const moveSegment = (segId, delta) => {
    setDraft((prev) => {
      if (!prev) return prev;
      const idx = prev.segments.findIndex((seg) => seg.id === segId);
      if (idx < 0) return prev;
      const nextIdx = idx + delta;
      if (nextIdx < 0 || nextIdx >= prev.segments.length) return prev;
      const segments = [...prev.segments];
      const [item] = segments.splice(idx, 1);
      segments.splice(nextIdx, 0, item);
      return { ...prev, segments };
    });
  };

  const saveDraft = () => {
    if (!draft?.name.trim() || draft.members.length === 0 || draft.segments.length === 0) {
      return;
    }
    const payload = {
      id: draft.id || newRoutineId(),
      name: draft.name.trim(),
      members: draft.members,
      segments: draft.segments.map((seg) => ({
        ...seg,
        steps: Math.max(1, Math.round(seg.steps) || 1),
        speedHz: clampStepperSpeed(seg.speedHz),
      })),
    };
    setRoutines((prev) => {
      const idx = prev.findIndex((item) => item.id === payload.id);
      if (idx >= 0) {
        const next = [...prev];
        next[idx] = payload;
        return next;
      }
      return [...prev, payload];
    });
    setDraft(null);
  };

  const deleteRoutine = (id) => {
    setRoutines((prev) => prev.filter((item) => item.id !== id));
    if (draft?.id === id) {
      setDraft(null);
    }
  };

  const routineHasRunning = (routine) => {
    const oids = oidsForRoutine(routine, stepperObjects);
    return oids.some((oid) => stepperRoutineOids[oid]);
  };

  const handleStart = (routine) => {
    onRun(() => startStepperRoutineForMembers(routine));
  };

  const handleStop = (routine) => {
    const oids = oidsForRoutine(routine, stepperObjects);
    onRun(() => stopSteppers(oids));
  };

  return (
    <div className={styles.routinesBlock}>
      <div className={styles.groupsHeader}>
        <p className={styles.listLabel}>Routines</p>
        <button
          type="button"
          className={styles.linkBtn}
          disabled={Boolean(draft) || memberOptions.length === 0}
          onClick={beginCreate}
        >
          + New routine
        </button>
      </div>

      <p className={styles.routinesHint}>
        Build repeating move patterns — each segment runs forward or backward for a set number of
        steps, then loops forever until stopped.
      </p>

      {memberOptions.length === 0 && (
        <p className={appStyles.muted}>Configure steppers on the board to build routines.</p>
      )}

      {draft && (
        <div className={styles.groupEditor}>
          <label className={styles.label} htmlFor="routine-name">
            Routine name
          </label>
          <input
            id="routine-name"
            className={styles.input}
            value={draft.name}
            onChange={(e) => setDraft({ ...draft, name: e.target.value })}
            placeholder="e.g. Scan pattern"
          />

          <p className={styles.label}>Steppers</p>
          <div className={styles.memberGrid}>
            {memberOptions.map(({ key, label }) => (
              <label key={key} className={styles.memberCheck}>
                <input
                  type="checkbox"
                  checked={draft.members.includes(key)}
                  onChange={() => toggleMember(key)}
                />
                {label}
              </label>
            ))}
          </div>

          <p className={styles.label}>Segments</p>
          <ul className={styles.segmentList}>
            {draft.segments.map((seg, index) => (
              <li key={seg.id} className={styles.segmentRow}>
                <span className={styles.segmentIndex}>{index + 1}</span>
                <select
                  className={styles.segmentSelect}
                  value={seg.forward ? 'fwd' : 'back'}
                  onChange={(e) => updateSegment(seg.id, { forward: e.target.value === 'fwd' })}
                  aria-label={`Segment ${index + 1} direction`}
                >
                  <option value="fwd">Forward</option>
                  <option value="back">Backward</option>
                </select>
                <input
                  type="number"
                  min="1"
                  className={styles.segmentStepsInput}
                  value={seg.steps}
                  onChange={(e) => updateSegment(seg.id, { steps: Number(e.target.value) })}
                  aria-label={`Segment ${index + 1} steps`}
                />
                <span className={styles.segmentUnit}>steps</span>
                <input
                  type="number"
                  min="1"
                  max="50000"
                  className={styles.segmentSpeedInput}
                  value={seg.speedHz}
                  onChange={(e) => updateSegment(seg.id, { speedHz: Number(e.target.value) })}
                  aria-label={`Segment ${index + 1} speed`}
                />
                <span className={styles.segmentUnit}>steps/s</span>
                <div className={styles.segmentTools}>
                  <button
                    type="button"
                    className={styles.iconBtn}
                    disabled={index === 0}
                    onClick={() => moveSegment(seg.id, -1)}
                    aria-label="Move up"
                  >
                    ↑
                  </button>
                  <button
                    type="button"
                    className={styles.iconBtn}
                    disabled={index === draft.segments.length - 1}
                    onClick={() => moveSegment(seg.id, 1)}
                    aria-label="Move down"
                  >
                    ↓
                  </button>
                  <button
                    type="button"
                    className={styles.iconBtnDanger}
                    disabled={draft.segments.length <= 1}
                    onClick={() => removeSegment(seg.id)}
                    aria-label="Remove segment"
                  >
                    ×
                  </button>
                </div>
              </li>
            ))}
          </ul>
          <button type="button" className={styles.linkBtn} onClick={addSegment}>
            + Add segment
          </button>

          <div className={styles.editorActions}>
            <button type="button" className={appStyles.button} disabled={busy} onClick={saveDraft}>
              Save routine
            </button>
            <button type="button" className={appStyles.buttonSecondary} onClick={cancelDraft}>
              Cancel
            </button>
          </div>
        </div>
      )}

      {routines.length === 0 && !draft && memberOptions.length > 0 && (
        <p className={appStyles.muted}>
          No routines yet. Create one to run repeating step patterns.
        </p>
      )}

      <ul className={styles.groupList}>
        {routines.map((routine) => {
          const oids = oidsForRoutine(routine, stepperObjects);
          const available = oids.length;
          const missing = routine.members.length - available;
          const running = routineHasRunning(routine);
          // Messages from members whose routine stopped on its own (backend-reported).
          const failures = [
            ...new Set(oids.map((oid) => stepperRoutineErrors[oid]).filter(Boolean)),
          ];
          return (
            <li key={routine.id} className={styles.groupCard}>
              <div className={styles.groupCardHead}>
                <span className={styles.groupName}>
                  {routine.name}
                  {running && <span className={styles.runningBadge}>Running</span>}
                </span>
                <div className={styles.groupCardTools}>
                  <button
                    type="button"
                    className={styles.linkBtn}
                    onClick={() => beginEdit(routine)}
                  >
                    Edit
                  </button>
                  <button
                    type="button"
                    className={styles.linkBtnDanger}
                    onClick={() => deleteRoutine(routine.id)}
                  >
                    Delete
                  </button>
                </div>
              </div>
              <p className={styles.groupMembers}>{formatRoutineMembers(routine, stepperObjects)}</p>
              <p className={styles.routinePattern}>{formatRoutinePattern(routine)}</p>
              {missing > 0 && connected && (
                <p className={styles.groupWarn}>{missing} member(s) not in current session</p>
              )}
              {failures.map((msg) => (
                <p key={msg} className={styles.groupWarn}>
                  Stopped on its own: {msg}
                </p>
              ))}
              <div className={styles.groupActions}>
                <button
                  type="button"
                  className={appStyles.button}
                  disabled={busy || !connected || available === 0 || running}
                  onClick={() => handleStart(routine)}
                >
                  Run
                </button>
                <button
                  type="button"
                  className={appStyles.buttonSecondary}
                  disabled={busy || !connected || available === 0 || !running}
                  onClick={() => handleStop(routine)}
                >
                  Stop
                </button>
              </div>
            </li>
          );
        })}
      </ul>
    </div>
  );
};

export default StepperRoutinesSection;
