import { useState } from 'react';

import NumberField from '@/components/NumberField/NumberField';
import { Button } from '@/components/ui/button';
import { useMCUSession } from '@/context/MCUSessionContext';
import { DEFAULT_STEPPER_SPEED, FAST_FACTOR, SLOW_FACTOR } from '@/helpers/stepperGroups';

import { BenchCard, ScaleButtons } from '../BenchUi';
import StepperGroupsSection from './StepperGroupsSection';
import StepperRoutinesSection from './StepperRoutinesSection';

const stepperLabel = (obj) => obj.label || obj.slot_id || obj.section || `object ${obj.oid}`;

/** Bench › Steppers: every stepper at once, groups of them, and repeating routines. */
export default function GlobalSteppersPanel() {
  const {
    connected,
    stepperObjects,
    stepperJogState,
    startAllSteppers,
    stopAllSteppers,
    reverseAllSteppers,
    scaleAllStepperSpeeds,
  } = useMCUSession();
  const [busy, setBusy] = useState(false);
  const [startSpeed, setStartSpeed] = useState(DEFAULT_STEPPER_SPEED);
  const [actionError, setActionError] = useState('');
  const running = Object.entries(stepperJogState);
  const none = !connected || stepperObjects.length === 0;

  const run = async (action) => {
    setBusy(true);
    setActionError('');
    try {
      await action();
    } catch (err) {
      setActionError(err.message || 'The stepper command failed');
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="grid items-start gap-4 xl:grid-cols-2">
      <BenchCard
        title="All steppers"
        about={
          connected
            ? `${stepperObjects.length} stepper${stepperObjects.length === 1 ? '' : 's'} in the connected config, jogging together.`
            : 'Connect the board to run them. Groups and routines can be edited without it.'
        }
      >
        {actionError && <p className="text-sm text-destructive">{actionError}</p>}
        <div className="flex flex-wrap items-end gap-2">
          <NumberField
            id="all-start-speed"
            label="Start at"
            value={startSpeed}
            min={1}
            max={50000}
            unit="steps/s"
            disabled={busy || !connected}
            onChange={setStartSpeed}
          />
          <Button
            disabled={busy || none}
            onClick={() => run(() => startAllSteppers({ forward: true, speedHz: startSpeed }))}
          >
            Start all
          </Button>
          <Button variant="outline" disabled={busy || none} onClick={() => run(stopAllSteppers)}>
            Stop all
          </Button>
          <Button
            variant="outline"
            disabled={busy || !connected || running.length === 0}
            onClick={() => run(reverseAllSteppers)}
          >
            Reverse all
          </Button>
        </div>
        <ScaleButtons
          lower="Slower"
          higher="Faster"
          lowFactor={SLOW_FACTOR}
          highFactor={FAST_FACTOR}
          disabled={busy || !connected || running.length === 0}
          onScale={(f) => run(() => scaleAllStepperSpeeds(f))}
        />
        {connected && (
          <div>
            <p className="mb-2 text-sm font-medium">Running now</p>
            {running.length === 0 ? (
              <p className="text-sm text-muted-foreground">Nothing jogging.</p>
            ) : (
              <ul className="divide-y divide-border rounded-md border border-border text-sm">
                {running.map(([oidStr, state]) => {
                  const obj = stepperObjects.find((o) => o.oid === Number(oidStr));
                  return (
                    <li key={oidStr} className="flex justify-between gap-2 px-3 py-2">
                      <span>{obj ? stepperLabel(obj) : `object ${oidStr}`}</span>
                      <span className="font-mono text-muted-foreground">
                        {state.forward ? 'fwd' : 'rev'} {state.speedHz} steps/s
                      </span>
                    </li>
                  );
                })}
              </ul>
            )}
          </div>
        )}
      </BenchCard>
      <StepperGroupsSection connected={connected} busy={busy} onRun={run} />
      <StepperRoutinesSection
        connected={connected}
        busy={busy}
        onRun={run}
        className="xl:col-span-2"
      />
    </div>
  );
}
