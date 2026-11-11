import { useEffect, useState } from 'react';

import {
  clampStepperSpeed,
  DEFAULT_STEPPER_SPEED,
  FAST_FACTOR,
  labelForMemberKey,
  loadStepperGroups,
  newGroupId,
  oidsForMembers,
  saveStepperGroups,
  scaleGroupSpeeds,
  speedForMember,
  SLOW_FACTOR,
  startPlanForGroup,
  stepperMemberKey,
} from '../../../../helpers/stepperGroups';
import { useMCUSession } from '../../../../context/MCUSessionContext';

import styles from './GlobalSteppersPanel.module.css';
import appStyles from '../../../../styles/app.module.css';

function emptyDraft() {
  return { id: null, name: '', members: [], speeds: {} };
}

const StepperGroupsSection = ({ connected, busy, onRun }) => {
  const {
    stepperObjects,
    stepperJogState,
    startSteppersAtSpeeds,
    stopSteppers,
    reverseSteppers,
    scaleStepperSpeeds,
  } = useMCUSession();

  const [groups, setGroups] = useState(() => loadStepperGroups());
  const [draft, setDraft] = useState(null);

  useEffect(() => {
    saveStepperGroups(groups);
  }, [groups]);

  const memberOptions = stepperObjects.map((obj) => ({
    key: stepperMemberKey(obj),
    label: obj.label || obj.slot_id || obj.section || `OID ${obj.oid}`,
  }));

  const beginCreate = () => {
    setDraft(emptyDraft());
  };

  const beginEdit = (group) => {
    setDraft({
      id: group.id,
      name: group.name,
      members: [...group.members],
      speeds: { ...(group.speeds || {}) },
    });
  };

  const cancelDraft = () => setDraft(null);

  const toggleMember = (key) => {
    setDraft((prev) => {
      if (!prev) return prev;
      const isRemoving = prev.members.includes(key);
      const members = isRemoving
        ? prev.members.filter((item) => item !== key)
        : [...prev.members, key];
      const speeds = { ...prev.speeds };
      if (!isRemoving && speeds[key] == null) {
        speeds[key] = DEFAULT_STEPPER_SPEED;
      }
      return { ...prev, members, speeds };
    });
  };

  const setDraftMemberSpeed = (key, value) => {
    setDraft((prev) => {
      if (!prev) return prev;
      return {
        ...prev,
        speeds: { ...prev.speeds, [key]: clampStepperSpeed(value) },
      };
    });
  };

  const saveDraft = () => {
    if (!draft?.name.trim() || draft.members.length === 0) {
      return;
    }
    const speeds = {};
    for (const key of draft.members) {
      speeds[key] = clampStepperSpeed(draft.speeds?.[key] ?? DEFAULT_STEPPER_SPEED);
    }
    const payload = {
      id: draft.id || newGroupId(),
      name: draft.name.trim(),
      members: draft.members,
      speeds,
    };
    setGroups((prev) => {
      const idx = prev.findIndex((g) => g.id === payload.id);
      if (idx >= 0) {
        const next = [...prev];
        next[idx] = payload;
        return next;
      }
      return [...prev, payload];
    });
    setDraft(null);
  };

  const deleteGroup = (id) => {
    setGroups((prev) => prev.filter((g) => g.id !== id));
    if (draft?.id === id) {
      setDraft(null);
    }
  };

  const groupOids = (group) => oidsForMembers(group.members, stepperObjects);

  const groupHasRunning = (group) => {
    const oids = groupOids(group);
    return oids.some((oid) => stepperJogState[oid]);
  };

  const handleGroupStart = (group) => {
    const plan = startPlanForGroup(group, stepperObjects);
    onRun(() => startSteppersAtSpeeds(plan));
  };

  const handleGroupScale = (group, factor) => {
    const oids = groupOids(group);
    setGroups((prev) =>
      prev.map((item) => (item.id === group.id ? scaleGroupSpeeds(item, factor) : item)),
    );
    onRun(async () => {
      if (oids.some((oid) => stepperJogState[oid])) {
        await scaleStepperSpeeds(oids, factor);
      }
    });
  };

  const formatMemberSpeeds = (group) =>
    group.members
      .map((key) => `${labelForMemberKey(key, stepperObjects)} ${speedForMember(group, key)}`)
      .join(' · ');

  return (
    <div className={styles.groupsBlock}>
      <div className={styles.groupsHeader}>
        <p className={styles.listLabel}>Groups</p>
        <button
          type="button"
          className={styles.linkBtn}
          disabled={Boolean(draft) || memberOptions.length === 0}
          onClick={beginCreate}
        >
          + New group
        </button>
      </div>

      {memberOptions.length === 0 && (
        <p className={appStyles.muted}>Configure steppers on the board to build groups.</p>
      )}

      {draft && (
        <div className={styles.groupEditor}>
          <label className={styles.label} htmlFor="group-name">
            Group name
          </label>
          <input
            id="group-name"
            className={styles.input}
            value={draft.name}
            onChange={(e) => setDraft({ ...draft, name: e.target.value })}
            placeholder="e.g. XY axes"
          />
          <p className={styles.label}>Steppers and speeds</p>
          <div className={styles.memberGrid}>
            {memberOptions.map(({ key, label }) => {
              const checked = draft.members.includes(key);
              return (
                <div key={key} className={styles.memberRow}>
                  <label className={styles.memberCheck}>
                    <input type="checkbox" checked={checked} onChange={() => toggleMember(key)} />
                    {label}
                  </label>
                  <input
                    type="number"
                    min="1"
                    max="50000"
                    className={styles.memberSpeedInput}
                    disabled={!checked}
                    value={checked ? (draft.speeds?.[key] ?? DEFAULT_STEPPER_SPEED) : ''}
                    onChange={(e) => setDraftMemberSpeed(key, Number(e.target.value))}
                    aria-label={`${label} speed`}
                  />
                </div>
              );
            })}
          </div>
          <div className={styles.editorActions}>
            <button type="button" className={appStyles.button} disabled={busy} onClick={saveDraft}>
              Save group
            </button>
            <button type="button" className={appStyles.buttonSecondary} onClick={cancelDraft}>
              Cancel
            </button>
          </div>
        </div>
      )}

      {groups.length === 0 && !draft && memberOptions.length > 0 && (
        <p className={appStyles.muted}>No groups yet. Create one to control steppers together.</p>
      )}

      <ul className={styles.groupList}>
        {groups.map((group) => {
          const oids = groupOids(group);
          const available = oids.length;
          const missing = group.members.length - available;
          return (
            <li key={group.id} className={styles.groupCard}>
              <div className={styles.groupCardHead}>
                <span className={styles.groupName}>{group.name}</span>
                <div className={styles.groupCardTools}>
                  <button type="button" className={styles.linkBtn} onClick={() => beginEdit(group)}>
                    Edit
                  </button>
                  <button
                    type="button"
                    className={styles.linkBtnDanger}
                    onClick={() => deleteGroup(group.id)}
                  >
                    Delete
                  </button>
                </div>
              </div>
              <p className={styles.groupMembers}>{formatMemberSpeeds(group)}</p>
              {missing > 0 && connected && (
                <p className={styles.groupWarn}>{missing} member(s) not in current session</p>
              )}
              <div className={styles.groupActions}>
                <button
                  type="button"
                  className={appStyles.button}
                  disabled={busy || !connected || available === 0}
                  onClick={() => handleGroupStart(group)}
                >
                  Start
                </button>
                <button
                  type="button"
                  className={appStyles.buttonSecondary}
                  disabled={busy || !connected || available === 0}
                  onClick={() => onRun(() => stopSteppers(oids))}
                >
                  Stop
                </button>
                <button
                  type="button"
                  className={appStyles.buttonSecondary}
                  disabled={busy || !connected || !groupHasRunning(group)}
                  onClick={() => onRun(() => reverseSteppers(oids))}
                >
                  Reverse
                </button>
              </div>
              <div className={styles.groupSpeedRow}>
                <button
                  type="button"
                  className={styles.speedBtn}
                  disabled={busy || !connected || group.members.length === 0}
                  onClick={() => handleGroupScale(group, SLOW_FACTOR)}
                >
                  Slower
                  <span className={styles.speedHint}>×{SLOW_FACTOR}</span>
                </button>
                <button
                  type="button"
                  className={styles.speedBtn}
                  disabled={busy || !connected || group.members.length === 0}
                  onClick={() => handleGroupScale(group, FAST_FACTOR)}
                >
                  Faster
                  <span className={styles.speedHint}>×{FAST_FACTOR}</span>
                </button>
              </div>
            </li>
          );
        })}
      </ul>
    </div>
  );
};

export default StepperGroupsSection;
