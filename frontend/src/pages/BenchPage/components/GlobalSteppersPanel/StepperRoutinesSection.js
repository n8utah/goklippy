import { ArrowDown, ArrowUp, Plus, Trash2 } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';

import Segmented from '@/components/Segmented/Segmented';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useMCUSession } from '@/context/MCUSessionContext';
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
} from '@/helpers/stepperRoutines';

import { BenchCard, MemberPicker, SavedItem } from '../BenchUi';

const DIRECTIONS = [
  { value: true, label: 'Forward' },
  { value: false, label: 'Back' },
];

/**
 * Repeating move patterns in motor steps: each segment turns forward or back so many steps
 * at a speed, and the routine loops until stopped. (Channels' routines, in rotor degrees,
 * are under Machine › Channels.)
 */
export default function StepperRoutinesSection({ connected, busy, onRun, className }) {
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

  const options = useMemo(
    () =>
      stepperObjects.map((obj) => ({
        key: stepperMemberKey(obj),
        label: labelForMemberKey(stepperMemberKey(obj), stepperObjects),
      })),
    [stepperObjects],
  );

  const patchDraft = (fn) => setDraft((prev) => (prev ? fn(prev) : prev));
  const updateSegment = (id, patch) =>
    patchDraft((d) => ({
      ...d,
      segments: d.segments.map((s) => (s.id === id ? { ...s, ...patch } : s)),
    }));
  const moveSegment = (id, delta) =>
    patchDraft((d) => {
      const i = d.segments.findIndex((s) => s.id === id);
      const j = i + delta;
      if (i < 0 || j < 0 || j >= d.segments.length) return d;
      const segments = [...d.segments];
      const [item] = segments.splice(i, 1);
      segments.splice(j, 0, item);
      return { ...d, segments };
    });

  const save = () => {
    if (!draft?.name.trim() || draft.members.length === 0 || draft.segments.length === 0) return;
    const payload = {
      id: draft.id || newRoutineId(),
      name: draft.name.trim(),
      members: draft.members,
      segments: draft.segments.map((s) => ({
        ...s,
        steps: Math.max(1, Math.round(s.steps) || 1),
        speedHz: clampStepperSpeed(s.speedHz),
      })),
    };
    setRoutines((prev) =>
      prev.some((r) => r.id === payload.id)
        ? prev.map((r) => (r.id === payload.id ? payload : r))
        : [...prev, payload],
    );
    setDraft(null);
  };

  const remove = (id) => {
    setRoutines((prev) => prev.filter((r) => r.id !== id));
    if (draft?.id === id) setDraft(null);
  };

  return (
    <BenchCard
      className={className}
      title="Routines"
      about="Repeating patterns in motor steps: each segment turns forward or back so many steps, and the routine loops until stopped. Kept in this browser."
      action={
        <Button
          variant="outline"
          size="sm"
          disabled={Boolean(draft) || options.length === 0}
          onClick={() =>
            setDraft({
              id: '',
              name: '',
              members: options.length === 1 ? [options[0].key] : [],
              segments: defaultSegments(),
            })
          }
        >
          <Plus /> New routine
        </Button>
      }
    >
      {options.length === 0 && (
        <p className="text-sm text-muted-foreground">Connect the board to list its steppers.</p>
      )}

      {draft && (
        <div className="grid gap-4 rounded-lg border border-primary/40 bg-accent/40 p-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.6fr)]">
          <div className="flex flex-col gap-3">
            <label htmlFor="routine-name" className="flex flex-col gap-1.5 text-sm font-medium">
              Name
              <Input
                id="routine-name"
                value={draft.name}
                placeholder="e.g. Shake back and forth"
                onChange={(e) => setDraft({ ...draft, name: e.target.value })}
              />
            </label>
            <p className="text-sm font-medium">Steppers</p>
            <MemberPicker
              options={options}
              chosen={draft.members}
              onToggle={(key) =>
                patchDraft((d) => ({
                  ...d,
                  members: d.members.includes(key)
                    ? d.members.filter((k) => k !== key)
                    : [...d.members, key],
                }))
              }
            />
          </div>
          <div className="flex flex-col gap-2">
            <p className="text-sm font-medium">Segments, in order</p>
            <ol className="flex flex-col gap-2">
              {draft.segments.map((seg, i) => (
                <li
                  key={seg.id}
                  className="flex flex-wrap items-center gap-2 rounded-md border border-border bg-card px-3 py-2"
                >
                  <span className="flex size-7 items-center justify-center rounded-full bg-muted font-mono text-xs">
                    {i + 1}
                  </span>
                  <Segmented
                    label={`Segment ${i + 1} direction`}
                    value={seg.forward}
                    options={DIRECTIONS}
                    onChange={(forward) => updateSegment(seg.id, { forward })}
                  />
                  <Input
                    type="number"
                    min={1}
                    className="h-9 w-24 text-right font-mono"
                    value={seg.steps}
                    aria-label={`Segment ${i + 1} steps`}
                    onChange={(e) => updateSegment(seg.id, { steps: Number(e.target.value) })}
                  />
                  <span className="text-xs text-muted-foreground">steps at</span>
                  <Input
                    type="number"
                    min={1}
                    max={50000}
                    className="h-9 w-24 text-right font-mono"
                    value={seg.speedHz}
                    aria-label={`Segment ${i + 1} speed`}
                    onChange={(e) => updateSegment(seg.id, { speedHz: Number(e.target.value) })}
                  />
                  <span className="text-xs text-muted-foreground">steps/s</span>
                  <span className="ml-auto flex">
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      aria-label="Move up"
                      disabled={i === 0}
                      onClick={() => moveSegment(seg.id, -1)}
                    >
                      <ArrowUp />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      aria-label="Move down"
                      disabled={i === draft.segments.length - 1}
                      onClick={() => moveSegment(seg.id, 1)}
                    >
                      <ArrowDown />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      aria-label="Remove segment"
                      className="text-destructive"
                      disabled={draft.segments.length <= 1}
                      onClick={() =>
                        patchDraft((d) => ({
                          ...d,
                          segments: d.segments.filter((s) => s.id !== seg.id),
                        }))
                      }
                    >
                      <Trash2 />
                    </Button>
                  </span>
                </li>
              ))}
            </ol>
            <div className="flex flex-wrap gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() =>
                  patchDraft((d) => ({
                    ...d,
                    segments: [
                      ...d.segments,
                      {
                        id: newSegmentId(),
                        forward: true,
                        steps: 50,
                        speedHz: DEFAULT_STEPPER_SPEED,
                      },
                    ],
                  }))
                }
              >
                <Plus /> Add segment
              </Button>
              <span className="flex-1" />
              <Button
                disabled={busy || !draft.name.trim() || draft.members.length === 0}
                onClick={save}
              >
                Save routine
              </Button>
              <Button variant="ghost" onClick={() => setDraft(null)}>
                Cancel
              </Button>
            </div>
          </div>
        </div>
      )}

      {routines.length === 0 && !draft && options.length > 0 && (
        <p className="text-sm text-muted-foreground">No routines yet.</p>
      )}
      <ul className="grid gap-3 lg:grid-cols-2">
        {routines.map((routine) => {
          const oids = oidsForRoutine(routine, stepperObjects);
          const missing = routine.members.length - oids.length;
          const running = oids.some((oid) => stepperRoutineOids[oid]);
          // Members whose routine stopped on its own, as the backend reported.
          const failures = [
            ...new Set(oids.map((oid) => stepperRoutineErrors[oid]).filter(Boolean)),
          ];
          return (
            <SavedItem
              key={routine.id}
              name={routine.name}
              badge={running && <Badge variant="running">Running</Badge>}
              summary={
                <>
                  {formatRoutineMembers(routine, stepperObjects)}
                  <span className="block font-mono text-xs">{formatRoutinePattern(routine)}</span>
                </>
              }
              notes={[
                ...(missing > 0 && connected
                  ? [`${missing} of its steppers aren’t in the connected config.`]
                  : []),
                ...failures.map((m) => `Stopped on its own: ${m}`),
              ]}
              onEdit={() =>
                setDraft({ ...routine, segments: routine.segments.map((s) => ({ ...s })) })
              }
              onDelete={() => remove(routine.id)}
            >
              <div className="flex gap-2">
                <Button
                  size="sm"
                  disabled={busy || !connected || oids.length === 0 || running}
                  onClick={() => onRun(() => startStepperRoutineForMembers(routine))}
                >
                  Run
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  disabled={busy || !connected || !running}
                  onClick={() => onRun(() => stopSteppers(oids))}
                >
                  Stop
                </Button>
              </div>
            </SavedItem>
          );
        })}
      </ul>
    </BenchCard>
  );
}
