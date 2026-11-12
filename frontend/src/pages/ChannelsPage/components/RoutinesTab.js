import { Plus, Trash2 } from 'lucide-react';
import { useState } from 'react';

import NumberField from '@/components/NumberField/NumberField';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { newMove, newSorterRoutine } from '@/helpers/useMachineMotors';

/**
 * Channels › Routines: moves in rotor degrees and rpm, repeated in order until stopped, so
 * a routine does the same motion on any channel. Each channel's tab picks and runs one.
 */
export default function RoutinesTab({ routines, onChange }) {
  const [selectedId, setSelectedId] = useState(routines[0]?.id ?? '');
  const routine = routines.find((r) => r.id === selectedId) || routines[0];

  const update = (patch) =>
    onChange(routines.map((r) => (r.id === routine.id ? { ...r, ...patch } : r)));
  const updateMove = (moveId, patch) =>
    update({ moves: routine.moves.map((m) => (m.id === moveId ? { ...m, ...patch } : m)) });
  const add = () => {
    const created = newSorterRoutine();
    onChange([...routines, created]);
    setSelectedId(created.id);
  };
  const remove = () => {
    onChange(routines.filter((r) => r.id !== routine.id));
    setSelectedId('');
  };

  return (
    <section className="flex flex-col gap-4 rounded-xl border border-border bg-card p-5">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
        <div className="max-w-[70ch]">
          <h2 className="font-display text-xl font-semibold">Routines</h2>
          <p className="text-sm text-muted-foreground">
            Moves in rotor degrees and rpm, repeated in order until stopped. Forward follows each
            channel’s direction. Pick and run one from a channel’s tab.
          </p>
        </div>
        <Button variant="outline" onClick={add} className="shrink-0">
          <Plus /> New routine
        </Button>
      </div>

      {!routine ? (
        <p className="text-sm text-muted-foreground">No routines yet.</p>
      ) : (
        <>
          <div className="flex flex-wrap items-end gap-3">
            <label className="flex flex-col gap-1.5 text-sm">
              <span className="font-medium">Routine</span>
              <Select value={routine.id} onValueChange={setSelectedId}>
                <SelectTrigger className="w-56">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {routines.map((r) => (
                    <SelectItem key={r.id} value={r.id}>
                      {r.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </label>
            <label htmlFor="routine-name" className="flex flex-col gap-1.5 text-sm">
              <span className="font-medium">Name</span>
              <Input
                id="routine-name"
                className="w-56"
                value={routine.name}
                onChange={(e) => update({ name: e.target.value })}
              />
            </label>
            <Button variant="ghost" onClick={remove} className="text-destructive">
              <Trash2 /> Delete routine
            </Button>
          </div>

          <ol className="flex flex-col gap-2">
            {routine.moves.map((move, i) => (
              <li
                key={move.id}
                className="flex flex-wrap items-end gap-3 rounded-lg border border-border px-3 py-3"
              >
                <span className="flex size-8 items-center justify-center rounded-full bg-muted font-mono text-sm">
                  {i + 1}
                </span>
                <label className="flex flex-col gap-1.5 text-sm">
                  <span className="font-medium">Direction</span>
                  <Select
                    value={move.forward ? 'forward' : 'reverse'}
                    onValueChange={(v) => updateMove(move.id, { forward: v === 'forward' })}
                  >
                    <SelectTrigger className="w-32">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="forward">Forward</SelectItem>
                      <SelectItem value="reverse">Reverse</SelectItem>
                    </SelectContent>
                  </Select>
                </label>
                <NumberField
                  id={`${move.id}-deg`}
                  label="Turn"
                  value={move.degrees}
                  min={0.01}
                  step="any"
                  unit="°"
                  onChange={(degrees) => updateMove(move.id, { degrees })}
                />
                <NumberField
                  id={`${move.id}-rpm`}
                  label="Speed"
                  value={move.rpm}
                  min={0.01}
                  step="any"
                  unit="rpm"
                  onChange={(rpm) => updateMove(move.id, { rpm })}
                />
                <Button
                  variant="ghost"
                  size="icon"
                  className="ml-auto"
                  disabled={routine.moves.length === 1}
                  aria-label={`Remove move ${i + 1}`}
                  onClick={() => update({ moves: routine.moves.filter((m) => m.id !== move.id) })}
                >
                  <Trash2 />
                </Button>
              </li>
            ))}
          </ol>
          <div>
            <Button
              variant="outline"
              onClick={() => update({ moves: [...routine.moves, newMove()] })}
            >
              <Plus /> Add move
            </Button>
          </div>
        </>
      )}
    </section>
  );
}
