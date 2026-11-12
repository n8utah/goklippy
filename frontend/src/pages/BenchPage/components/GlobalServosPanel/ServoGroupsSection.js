import { Plus } from 'lucide-react';
import { useEffect, useState } from 'react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useMCUSession } from '@/context/MCUSessionContext';
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
} from '@/helpers/servoGroups';

import { BenchCard, MemberPicker, SavedItem, ScaleButtons } from '../BenchUi';

const emptyDraft = () => ({ id: null, name: '', members: [], angles: {} });

/** Servos that move together, each to its own angle. */
export default function ServoGroupsSection({ connected, busy, onRun }) {
  const { servoObjects, setServosAtAngles } = useMCUSession();
  const [groups, setGroups] = useState(() => loadServoGroups());
  const [draft, setDraft] = useState(null);

  useEffect(() => {
    saveServoGroups(groups);
  }, [groups]);

  const options = servoObjects.map((obj) => ({
    key: servoMemberKey(obj),
    label: obj.label || obj.slot_id || obj.section || `object ${obj.oid}`,
  }));

  const toggle = (key) =>
    setDraft((prev) => {
      const removing = prev.members.includes(key);
      const angles = { ...prev.angles };
      if (!removing && angles[key] == null) angles[key] = DEFAULT_SERVO_ANGLE;
      return {
        ...prev,
        members: removing ? prev.members.filter((k) => k !== key) : [...prev.members, key],
        angles,
      };
    });

  const save = () => {
    if (!draft?.name.trim() || draft.members.length === 0) return;
    const angles = Object.fromEntries(
      draft.members.map((k) => [k, clampServoAngle(draft.angles?.[k] ?? DEFAULT_SERVO_ANGLE)]),
    );
    const payload = {
      id: draft.id || newGroupId(),
      name: draft.name.trim(),
      members: draft.members,
      angles,
    };
    setGroups((prev) =>
      prev.some((g) => g.id === payload.id)
        ? prev.map((g) => (g.id === payload.id ? payload : g))
        : [...prev, payload],
    );
    setDraft(null);
  };

  const remove = (id) => {
    setGroups((prev) => prev.filter((g) => g.id !== id));
    if (draft?.id === id) setDraft(null);
  };

  /** Saves the group's new angles and moves its servos there. */
  const moveTo = (next) => {
    setGroups((prev) => prev.map((g) => (g.id === next.id ? next : g)));
    onRun(() => setServosAtAngles(applyPlanForGroup(next, servoObjects)));
  };

  return (
    <BenchCard
      title="Groups"
      about="Servos that move together, each to its own angle. Kept in this browser."
      action={
        <Button
          variant="outline"
          size="sm"
          disabled={Boolean(draft) || options.length === 0}
          onClick={() => setDraft(emptyDraft())}
        >
          <Plus /> New group
        </Button>
      }
    >
      {options.length === 0 && (
        <p className="text-sm text-muted-foreground">Connect the board to list its servos.</p>
      )}

      {draft && (
        <div className="flex flex-col gap-3 rounded-lg border border-primary/40 bg-accent/40 p-4">
          <label htmlFor="servo-group-name" className="flex flex-col gap-1.5 text-sm font-medium">
            Name
            <Input
              id="servo-group-name"
              value={draft.name}
              placeholder="e.g. Layer doors"
              onChange={(e) => setDraft({ ...draft, name: e.target.value })}
            />
          </label>
          <p className="text-sm font-medium">Servos and their angles</p>
          <MemberPicker
            options={options}
            chosen={draft.members}
            onToggle={toggle}
            values={Object.fromEntries(
              options.map(({ key }) => [key, draft.angles?.[key] ?? DEFAULT_SERVO_ANGLE]),
            )}
            onValue={(key, v) =>
              setDraft((prev) => ({
                ...prev,
                angles: { ...prev.angles, [key]: clampServoAngle(v) },
              }))
            }
            unit="°"
            min={0}
            max={180}
          />
          <div className="flex gap-2">
            <Button
              disabled={busy || !draft.name.trim() || draft.members.length === 0}
              onClick={save}
            >
              Save group
            </Button>
            <Button variant="ghost" onClick={() => setDraft(null)}>
              Cancel
            </Button>
          </div>
        </div>
      )}

      {groups.length === 0 && !draft && options.length > 0 && (
        <p className="text-sm text-muted-foreground">No groups yet.</p>
      )}
      <ul className="flex flex-col gap-3">
        {groups.map((group) => {
          const oids = oidsForMembers(group.members, servoObjects);
          const missing = group.members.length - oids.length;
          const off = busy || !connected || oids.length === 0;
          return (
            <SavedItem
              key={group.id}
              name={group.name}
              summary={group.members
                .map((k) => `${labelForMemberKey(k, servoObjects)} ${angleForMember(group, k)}°`)
                .join(' · ')}
              notes={
                missing > 0 && connected
                  ? [`${missing} of its servos aren’t in the connected config.`]
                  : []
              }
              onEdit={() =>
                setDraft({
                  id: group.id,
                  name: group.name,
                  members: [...group.members],
                  angles: { ...(group.angles || {}) },
                })
              }
              onDelete={() => remove(group.id)}
            >
              <div className="flex flex-wrap gap-2">
                <Button
                  size="sm"
                  disabled={off}
                  onClick={() =>
                    onRun(() => setServosAtAngles(applyPlanForGroup(group, servoObjects)))
                  }
                >
                  Move
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  disabled={off}
                  onClick={() =>
                    onRun(() =>
                      setServosAtAngles(oids.map((oid) => ({ oid, angle: DEFAULT_SERVO_ANGLE }))),
                    )
                  }
                >
                  Center
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  disabled={off}
                  onClick={() => moveTo(flipGroupAngles(group))}
                >
                  Flip
                </Button>
              </div>
              <ScaleButtons
                lower="Lower"
                higher="Raise"
                lowFactor={SLOW_FACTOR}
                highFactor={FAST_FACTOR}
                disabled={busy || !connected || group.members.length === 0}
                onScale={(f) => moveTo(scaleGroupAngles(group, f))}
              />
            </SavedItem>
          );
        })}
      </ul>
    </BenchCard>
  );
}
