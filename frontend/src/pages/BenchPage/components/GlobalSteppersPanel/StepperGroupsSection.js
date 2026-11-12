import { Plus } from 'lucide-react';
import { useEffect, useState } from 'react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useMCUSession } from '@/context/MCUSessionContext';
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
} from '@/helpers/stepperGroups';

import { BenchCard, MemberPicker, SavedItem, ScaleButtons } from '../BenchUi';

const emptyDraft = () => ({ id: null, name: '', members: [], speeds: {} });

/** Steppers that start, stop, reverse and change speed together, each at its own speed. */
export default function StepperGroupsSection({ connected, busy, onRun }) {
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

  const options = stepperObjects.map((obj) => ({
    key: stepperMemberKey(obj),
    label: obj.label || obj.slot_id || obj.section || `object ${obj.oid}`,
  }));

  const toggle = (key) =>
    setDraft((prev) => {
      const removing = prev.members.includes(key);
      const speeds = { ...prev.speeds };
      if (!removing && speeds[key] == null) speeds[key] = DEFAULT_STEPPER_SPEED;
      return {
        ...prev,
        members: removing ? prev.members.filter((k) => k !== key) : [...prev.members, key],
        speeds,
      };
    });

  const save = () => {
    if (!draft?.name.trim() || draft.members.length === 0) return;
    const speeds = Object.fromEntries(
      draft.members.map((k) => [k, clampStepperSpeed(draft.speeds?.[k] ?? DEFAULT_STEPPER_SPEED)]),
    );
    const payload = {
      id: draft.id || newGroupId(),
      name: draft.name.trim(),
      members: draft.members,
      speeds,
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

  const scale = (group, factor) => {
    const oids = oidsForMembers(group.members, stepperObjects);
    setGroups((prev) => prev.map((g) => (g.id === group.id ? scaleGroupSpeeds(g, factor) : g)));
    onRun(async () => {
      if (oids.some((oid) => stepperJogState[oid])) await scaleStepperSpeeds(oids, factor);
    });
  };

  return (
    <BenchCard
      title="Groups"
      about="Steppers that start, stop, reverse and change speed together, each at its own speed. Kept in this browser."
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
        <p className="text-sm text-muted-foreground">Connect the board to list its steppers.</p>
      )}

      {draft && (
        <div className="flex flex-col gap-3 rounded-lg border border-primary/40 bg-accent/40 p-4">
          <label htmlFor="group-name" className="flex flex-col gap-1.5 text-sm font-medium">
            Name
            <Input
              id="group-name"
              value={draft.name}
              placeholder="e.g. Feeders"
              onChange={(e) => setDraft({ ...draft, name: e.target.value })}
            />
          </label>
          <p className="text-sm font-medium">Steppers and their speeds</p>
          <MemberPicker
            options={options}
            chosen={draft.members}
            onToggle={toggle}
            values={Object.fromEntries(
              options.map(({ key }) => [key, draft.speeds?.[key] ?? DEFAULT_STEPPER_SPEED]),
            )}
            onValue={(key, v) =>
              setDraft((prev) => ({
                ...prev,
                speeds: { ...prev.speeds, [key]: clampStepperSpeed(v) },
              }))
            }
            unit="steps/s"
            min={1}
            max={50000}
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
          const oids = oidsForMembers(group.members, stepperObjects);
          const missing = group.members.length - oids.length;
          const anyRunning = oids.some((oid) => stepperJogState[oid]);
          return (
            <SavedItem
              key={group.id}
              name={group.name}
              summary={group.members
                .map((k) => `${labelForMemberKey(k, stepperObjects)} ${speedForMember(group, k)}`)
                .join(' · ')}
              notes={
                missing > 0 && connected
                  ? [`${missing} of its steppers aren’t in the connected config.`]
                  : []
              }
              onEdit={() =>
                setDraft({
                  id: group.id,
                  name: group.name,
                  members: [...group.members],
                  speeds: { ...(group.speeds || {}) },
                })
              }
              onDelete={() => remove(group.id)}
            >
              <div className="flex flex-wrap gap-2">
                <Button
                  size="sm"
                  disabled={busy || !connected || oids.length === 0}
                  onClick={() =>
                    onRun(() => startSteppersAtSpeeds(startPlanForGroup(group, stepperObjects)))
                  }
                >
                  Start
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  disabled={busy || !connected || oids.length === 0}
                  onClick={() => onRun(() => stopSteppers(oids))}
                >
                  Stop
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  disabled={busy || !connected || !anyRunning}
                  onClick={() => onRun(() => reverseSteppers(oids))}
                >
                  Reverse
                </Button>
              </div>
              <ScaleButtons
                lower="Slower"
                higher="Faster"
                lowFactor={SLOW_FACTOR}
                highFactor={FAST_FACTOR}
                disabled={busy || !connected || group.members.length === 0}
                onScale={(f) => scale(group, f)}
              />
            </SavedItem>
          );
        })}
      </ul>
    </BenchCard>
  );
}
