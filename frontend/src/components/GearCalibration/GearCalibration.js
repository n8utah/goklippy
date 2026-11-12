import { useState } from 'react';

import NumberField from '@/components/NumberField/NumberField';
import { Button } from '@/components/ui/button';

const whole = (value) => Math.round(value).toLocaleString();

/**
 * Gearing for one motor: the ratio typed in or worked out from tooth counts, what that
 * means in steps, and a check that turns one rotor turn so the ratio can be corrected from
 * a measurement. Used for C1–C4 and the chute.
 */
export default function GearCalibration({
  id,
  config,
  microsteps,
  microstepsAssumed,
  stepsPerRev,
  slot,
  canTurn,
  onChange,
  onTurn,
}) {
  const [turned, setTurned] = useState(false);
  const [actual, setActual] = useState(360);
  const teeth = config.teeth || {};

  const setTeeth = (patch) => {
    const next = { ...teeth, ...patch };
    const gear = next.motor > 0 && next.rotor > 0 ? next.rotor / next.motor : config.gear;
    onChange({ teeth: next, gear });
  };

  const turn = async () => {
    if (await onTurn()) {
      setActual(360);
      setTurned(true);
    }
  };

  const apply = () => {
    onChange({ gear: Math.round(((config.gear * 360) / actual) * 10000) / 10000, teeth: null });
    setTurned(false);
  };

  return (
    <fieldset className="flex flex-col gap-3">
      <legend className="mb-2 font-display font-semibold">Gearing</legend>
      <div className="flex flex-wrap gap-4">
        <NumberField
          id={`${id}-gear`}
          label="Motor turns per rotor turn"
          value={config.gear}
          min={0.01}
          step="any"
          onChange={(gear) => onChange({ gear, teeth: null })}
        />
        <NumberField
          id={`${id}-motor-teeth`}
          label="or motor gear teeth"
          value={teeth.motor ?? ''}
          min={1}
          onChange={(motor) => setTeeth({ motor })}
          inputClassName="w-20"
        />
        <NumberField
          id={`${id}-rotor-teeth`}
          label="rotor gear teeth"
          value={teeth.rotor ?? ''}
          min={1}
          onChange={(rotor) => setTeeth({ rotor })}
          inputClassName="w-20"
        />
      </div>
      <p className="text-sm text-muted-foreground">
        <span className="font-mono text-foreground">{whole(stepsPerRev)}</span> steps per rotor turn
        {slot && (
          <>
            ,{' '}
            <span className="font-mono text-foreground">
              {whole((stepsPerRev * slot.degrees) / 360)}
            </span>{' '}
            per {slot.name}
          </>
        )}
        . {microsteps} microsteps{' '}
        {microstepsAssumed
          ? '(assumed: the motor isn’t in the connected config)'
          : 'from the board config'}
        .
      </p>
      {turned ? (
        <div className="flex flex-wrap items-end gap-3 rounded-lg border border-primary/40 bg-accent p-3">
          <NumberField
            id={`${id}-actual`}
            label="Once it stops, how far did the rotor really turn?"
            value={actual}
            min={1}
            step="any"
            unit="°"
            onChange={setActual}
          />
          <Button disabled={!(actual > 0)} onClick={apply}>
            Correct the ratio
          </Button>
          <Button variant="ghost" onClick={() => setTurned(false)}>
            Cancel
          </Button>
        </div>
      ) : (
        <div className="flex flex-wrap items-center gap-3">
          <Button variant="outline" disabled={!canTurn} onClick={turn}>
            Calibrate: turn one rotor turn
          </Button>
          <span className="text-sm text-muted-foreground">
            Mark the rotor first. If it stops short or overshoots, enter how far it went.
          </span>
        </div>
      )}
    </fieldset>
  );
}
