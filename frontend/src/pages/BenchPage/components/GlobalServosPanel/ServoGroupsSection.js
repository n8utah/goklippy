import { useEffect, useState } from 'react';

import {
  angleForMember,
  applyPlanForGroup,
  clampServoAngle,
  DEFAULT_SERVO_ANGLE,
  FAST_FACTOR,
  flipGroupAngles,
  labelForMemberKey,
  loadServoGroups,
  newGroupId,
  oidsForMembers,
  saveServoGroups,
  scaleGroupAngles,
  servoMemberKey,
  SLOW_FACTOR,
} from '../../../../helpers/servoGroups';
import { useMCUSession } from '../../../../context/MCUSessionContext';

import styles from '../GlobalSteppersPanel/GlobalSteppersPanel.module.css';
import appStyles from '../../../../styles/app.module.css';

function emptyDraft() {
  return { id: null, name: '', members: [], angles: {} };
}

const ServoGroupsSection = ({ connected, busy, onRun }) => {
  const { servoObjects, setServosAtAngles } = useMCUSession();

  const [groups, setGroups] = useState(() => loadServoGroups());
  const [draft, setDraft] = useState(null);

  useEffect(() => {
    saveServoGroups(groups);
  }, [groups]);

  const memberOptions = servoObjects.map((obj) => ({
    key: servoMemberKey(obj),
    label: obj.label || obj.slot_id || obj.section || `OID ${obj.oid}`,
  }));

  const beginCreate = () => setDraft(emptyDraft());

  const beginEdit = (group) => {
    setDraft({
      id: group.id,
      name: group.name,
      members: [...group.members],
      angles: { ...(group.angles || {}) },
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
      const angles = { ...prev.angles };
      if (!isRemoving && angles[key] == null) {
        angles[key] = DEFAULT_SERVO_ANGLE;
      }
      return { ...prev, members, angles };
    });
  };

  const setDraftMemberAngle = (key, value) => {
    setDraft((prev) => {
      if (!prev) return prev;
      return {
        ...prev,
        angles: { ...prev.angles, [key]: clampServoAngle(value) },
      };
    });
  };

  const saveDraft = () => {
    if (!draft?.name.trim() || draft.members.length === 0) {
      return;
    }
    const angles = {};
    for (const key of draft.members) {
      angles[key] = clampServoAngle(draft.angles?.[key] ?? DEFAULT_SERVO_ANGLE);
    }
    const payload = {
      id: draft.id || newGroupId(),
      name: draft.name.trim(),
      members: draft.members,
      angles,
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

  const groupOids = (group) => oidsForMembers(group.members, servoObjects);

  const handleGroupApply = (group) => {
    const plan = applyPlanForGroup(group, servoObjects);
    onRun(() => setServosAtAngles(plan));
  };

  const handleGroupScale = (group, factor) => {
    setGroups((prev) =>
      prev.map((item) => (item.id === group.id ? scaleGroupAngles(item, factor) : item)),
    );
    onRun(async () => {
      const plan = applyPlanForGroup(scaleGroupAngles(group, factor), servoObjects);
      await setServosAtAngles(plan);
    });
  };

  const handleGroupFlip = (group) => {
    const flipped = flipGroupAngles(group);
    setGroups((prev) => prev.map((item) => (item.id === group.id ? flipped : item)));
    onRun(async () => {
      const plan = applyPlanForGroup(flipped, servoObjects);
      await setServosAtAngles(plan);
    });
  };

  const formatMemberAngles = (group) =>
    group.members
      .map((key) => `${labelForMemberKey(key, servoObjects)} ${angleForMember(group, key)}°`)
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
        <p className={appStyles.muted}>Configure servos on the board to build groups.</p>
      )}

      {draft && (
        <div className={styles.groupEditor}>
          <label className={styles.label} htmlFor="servo-group-name">
            Group name
          </label>
          <input
            id="servo-group-name"
            className={styles.input}
            value={draft.name}
            onChange={(e) => setDraft({ ...draft, name: e.target.value })}
            placeholder="e.g. Probe servos"
          />
          <p className={styles.label}>Servos and angles</p>
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
                    min="0"
                    max="180"
                    className={styles.memberSpeedInput}
                    disabled={!checked}
                    value={checked ? (draft.angles?.[key] ?? DEFAULT_SERVO_ANGLE) : ''}
                    onChange={(e) => setDraftMemberAngle(key, Number(e.target.value))}
                    aria-label={`${label} angle`}
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
        <p className={appStyles.muted}>No groups yet. Create one to control servos together.</p>
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
              <p className={styles.groupMembers}>{formatMemberAngles(group)}</p>
              {missing > 0 && connected && (
                <p className={styles.groupWarn}>{missing} member(s) not in current session</p>
              )}
              <div className={styles.groupActions}>
                <button
                  type="button"
                  className={appStyles.button}
                  disabled={busy || !connected || available === 0}
                  onClick={() => handleGroupApply(group)}
                >
                  Apply
                </button>
                <button
                  type="button"
                  className={appStyles.buttonSecondary}
                  disabled={busy || !connected || available === 0}
                  onClick={() =>
                    onRun(() =>
                      setServosAtAngles(oids.map((oid) => ({ oid, angle: DEFAULT_SERVO_ANGLE }))),
                    )
                  }
                >
                  Center
                </button>
                <button
                  type="button"
                  className={appStyles.buttonSecondary}
                  disabled={busy || !connected || available === 0}
                  onClick={() => handleGroupFlip(group)}
                >
                  Flip
                </button>
              </div>
              <div className={styles.groupSpeedRow}>
                <button
                  type="button"
                  className={styles.speedBtn}
                  disabled={busy || !connected || group.members.length === 0}
                  onClick={() => handleGroupScale(group, SLOW_FACTOR)}
                >
                  Lower
                  <span className={styles.speedHint}>×{SLOW_FACTOR}</span>
                </button>
                <button
                  type="button"
                  className={styles.speedBtn}
                  disabled={busy || !connected || group.members.length === 0}
                  onClick={() => handleGroupScale(group, FAST_FACTOR)}
                >
                  Raise
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

export default ServoGroupsSection;
