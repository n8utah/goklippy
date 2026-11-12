import { ArrowRight, Minus, Plus } from 'lucide-react';
import { useCallback, useEffect, useRef, useState } from 'react';

import NumberField from '@/components/NumberField/NumberField';
import Segmented from '@/components/Segmented/Segmented';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Slider } from '@/components/ui/slider';
import { useMCUSession } from '@/context/MCUSessionContext';
import {
  DEFAULT_OUTPUT_TIMEOUT_S,
  MAX_OUTPUT_TIMEOUT_S,
  isOnOffSlot,
} from '@/helpers/boardSlotConfig';

const DIRECTIONS = [
  { value: true, label: 'Forward' },
  { value: false, label: 'Reverse' },
];
const MODES = [
  { value: 'continuous', label: 'Continuous' },
  { value: 'distance', label: 'Distance' },
];

function Block({ title, children }) {
  return (
    <div className="flex flex-col gap-3 rounded-lg border border-border bg-muted/40 p-3">
      {title && <p className="text-sm font-medium">{title}</p>}
      {children}
    </div>
  );
}

function Stepper({ label, value, onChange, step, min, max, unit, id }) {
  const clamp = (v) => Math.min(max, Math.max(min, v));
  return (
    <div className="flex flex-col gap-1.5 text-sm">
      <label htmlFor={id} className="font-medium">
        {label}
      </label>
      <div className="flex items-center gap-1.5">
        <Button
          variant="outline"
          size="icon-sm"
          aria-label={`${label} down ${step}`}
          onClick={() => onChange(clamp(value - step))}
        >
          <Minus />
        </Button>
        <Input
          id={id}
          type="number"
          min={min}
          max={max}
          className="h-9 w-24 text-right font-mono"
          value={value}
          onChange={(e) => onChange(clamp(Number(e.target.value) || min))}
        />
        <Button
          variant="outline"
          size="icon-sm"
          aria-label={`${label} up ${step}`}
          onClick={() => onChange(clamp(value + step))}
        >
          <Plus />
        </Button>
        {unit && <span className="text-muted-foreground">{unit}</span>}
      </div>
    </div>
  );
}

/**
 * Board › Wiring's side panel: what's wired to the picked pin, and with a board connected,
 * trying it live (an output on and off, a servo's angle, a stepper's jog, a reading).
 */
export default function SlotPanel({
  hotspot,
  slots,
  slotStates,
  onUpdate,
  onClear,
  onOpenConfig,
  connected,
  liveObject,
  liveReading,
  onSetDigitalOut,
  onSetServo,
  onJogStepper,
  onStopStepper,
}) {
  if (!hotspot) {
    return (
      <aside className="ds flex flex-col gap-2 rounded-xl border border-border bg-card p-5">
        <h2 className="font-display text-lg font-semibold">Pin</h2>
        <p className="text-sm text-muted-foreground">
          Pick a pin or connector on the board to say what’s wired to it.
        </p>
        {connected && (
          <p className="text-sm text-muted-foreground">
            Wired pins show their live readings on the board.
          </p>
        )}
      </aside>
    );
  }

  const slotId = hotspot.slot_ids?.[0];
  const slot = slots[slotId] || {};
  const state = slotStates[slotId] || { connected: false, label: slot.label || hotspot.label };
  const timeout = state.maxMcuDuration ?? DEFAULT_OUTPUT_TIMEOUT_S;
  const onOff = isOnOffSlot(slot);

  const markWired = () =>
    onUpdate(slotId, {
      connected: true,
      label: state.label || slot.label || hotspot.label,
      type: slot.type,
      maxMcuDuration: timeout,
    });

  return (
    <aside className="ds flex flex-col gap-4 rounded-xl border border-border bg-card p-5">
      <div>
        <div className="flex flex-wrap items-center gap-2">
          <h2 className="font-display text-lg font-semibold">{hotspot.label}</h2>
          {state.connected ? (
            <Badge variant="running">Wired</Badge>
          ) : (
            <Badge variant="idle">Not set up</Badge>
          )}
        </div>
        <dl className="mt-1 grid grid-cols-[auto_1fr] gap-x-3 text-sm">
          <dt className="text-muted-foreground">Slot</dt>
          <dd className="font-mono">{slotId}</dd>
          <dt className="text-muted-foreground">Type</dt>
          <dd>{onOff ? 'on/off output' : slot.type || 'unknown'}</dd>
          <dt className="text-muted-foreground">Takes</dt>
          <dd>{(hotspot.accepts || []).join(', ') || '—'}</dd>
        </dl>
      </div>

      {connected && liveObject && (
        <LiveControls
          slot={slot}
          liveObject={liveObject}
          liveReading={liveReading}
          onSetDigitalOut={onSetDigitalOut}
          onSetServo={onSetServo}
          onJogStepper={onJogStepper}
          onStopStepper={onStopStepper}
        />
      )}

      {!connected && (
        <div className="flex flex-col gap-4">
          <label htmlFor="slot-label" className="flex flex-col gap-1.5 text-sm font-medium">
            Label
            <Input
              id="slot-label"
              value={state.label || ''}
              onChange={(e) =>
                onUpdate(slotId, { ...state, label: e.target.value, localOnly: true })
              }
            />
          </label>

          {onOff && (
            <div className="flex flex-col gap-1.5">
              <NumberField
                id="slot-timeout"
                label="Safety timeout (0: off)"
                value={timeout}
                min={0}
                max={MAX_OUTPUT_TIMEOUT_S}
                step={0.5}
                unit="s"
                onChange={(v) => onUpdate(slotId, { ...state, maxMcuDuration: v, localOnly: true })}
              />
              <p className="text-xs text-muted-foreground">
                If the backend stops refreshing this output for this long (it crashed, or the USB
                cable came out), the board shuts down: the output and every motor stop. Up to{' '}
                {MAX_OUTPUT_TIMEOUT_S} s.
              </p>
            </div>
          )}

          {slot.defaults?.sensor_type && !onOff && (
            <p className="text-sm">
              <span className="text-muted-foreground">Sensor type</span>{' '}
              <span className="font-mono">{slot.defaults.sensor_type}</span>
            </p>
          )}

          {slot.pins && (
            <div className="text-sm">
              <p className="mb-1 font-medium">Pins</p>
              <ul className="grid grid-cols-2 gap-x-3 font-mono text-xs">
                {Object.entries(slot.pins).map(([key, value]) => (
                  <li key={key}>
                    <span className="text-muted-foreground">{key}</span> {value}
                  </li>
                ))}
              </ul>
            </div>
          )}

          <div className="flex flex-wrap gap-2">
            <Button onClick={markWired}>{state.connected ? 'Update' : 'Mark as wired'}</Button>
            {state.connected && (
              <Button variant="outline" onClick={() => onClear(slotId)}>
                Clear
              </Button>
            )}
          </div>
        </div>
      )}

      <button
        type="button"
        onClick={onOpenConfig}
        className="flex items-center gap-1 self-start text-sm font-medium text-primary-text hover:underline"
      >
        Edit in the config file <ArrowRight className="size-4" aria-hidden />
      </button>
    </aside>
  );
}

function LiveControls({
  slot,
  liveObject,
  liveReading,
  onSetDigitalOut,
  onSetServo,
  onJogStepper,
  onStopStepper,
}) {
  const { kind, oid } = liveObject;
  const isServo = kind === 'pwm_out' && liveObject.meta?.device === 'servo';

  if (kind === 'analog_in' || slot.type === 'thermistor') {
    const val = liveReading?.value;
    return (
      <Block title="Live reading">
        <p className="font-mono text-2xl">
          {val != null ? Number(val).toFixed(3) : '—'}{' '}
          <span className="text-sm text-muted-foreground">{liveReading?.unit || 'adc'}</span>
        </p>
        <p className="text-xs text-muted-foreground">Section {liveObject.section}</p>
      </Block>
    );
  }
  if (kind === 'endstop') {
    return (
      <Block title="Switch">
        <Badge variant={liveReading?.value ? 'warning' : 'idle'} className="self-start">
          {liveReading?.value ? 'Pressed' : 'Open'}
        </Badge>
      </Block>
    );
  }
  if (kind === 'digital_out' || (kind === 'pwm_out' && !isServo)) {
    return (
      <DigitalOutControl
        oid={oid}
        onSetDigitalOut={onSetDigitalOut}
        label={slot.type === 'fan' ? 'Fan' : 'Output'}
      />
    );
  }
  if (isServo) return <ServoControl oid={oid} onSetServo={onSetServo} />;
  if (kind === 'stepper') {
    return (
      <StepperControl
        oid={oid}
        liveReading={liveReading}
        onJogStepper={onJogStepper}
        onStopStepper={onStopStepper}
      />
    );
  }
  return (
    <Block>
      <p className="text-sm text-muted-foreground">
        Set up ({kind}), object <span className="font-mono">{oid}</span>.
      </p>
    </Block>
  );
}

function DigitalOutControl({ oid, onSetDigitalOut, label }) {
  const [on, setOn] = useState(false);
  const [busy, setBusy] = useState(false);
  const toggle = async () => {
    setBusy(true);
    try {
      await onSetDigitalOut(oid, !on);
      setOn(!on);
    } finally {
      setBusy(false);
    }
  };
  return (
    <Block title={label}>
      <div className="flex items-center gap-3">
        <Button disabled={busy} onClick={toggle}>
          {on ? 'Turn off' : 'Turn on'}
        </Button>
        <span className="text-sm text-muted-foreground">{on ? 'On' : 'Off'}</span>
      </div>
    </Block>
  );
}

function ServoControl({ oid, onSetServo }) {
  const { servoAngles } = useMCUSession();
  const [angle, setAngle] = useState(servoAngles[oid] ?? 90);
  const [busy, setBusy] = useState(false);
  // Follow the angle when it's changed elsewhere (the bench, a group).
  const [known, setKnown] = useState(servoAngles[oid]);
  if (servoAngles[oid] != null && servoAngles[oid] !== known) {
    setKnown(servoAngles[oid]);
    setAngle(servoAngles[oid]);
  }

  const apply = async (next = angle) => {
    setBusy(true);
    try {
      const applied = await onSetServo(oid, next);
      if (applied != null) setAngle(applied);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Block title="Servo angle">
      <Stepper
        id={`servo-${oid}`}
        label="Angle"
        value={angle}
        onChange={setAngle}
        step={5}
        min={0}
        max={180}
        unit="°"
      />
      <Slider
        min={0}
        max={180}
        step={1}
        value={[angle]}
        disabled={busy}
        aria-label="Servo angle"
        onValueChange={([v]) => setAngle(v)}
        onValueCommit={([v]) => apply(v)}
      />
      <Button disabled={busy} onClick={() => apply()} className="self-start">
        Move to {angle}°
      </Button>
    </Block>
  );
}

function StepperControl({ oid, liveReading, onJogStepper, onStopStepper }) {
  const { isStepperRunning, setStepperRunning } = useMCUSession();
  const [forward, setForward] = useState(true);
  const [speedHz, setSpeedHz] = useState(500);
  const [mode, setMode] = useState('continuous');
  const [steps, setSteps] = useState(200);
  const [busy, setBusy] = useState(false);
  const running = isStepperRunning(oid);
  const paramsRef = useRef({ forward: true, speedHz: 500 });

  const applyContinuousJog = useCallback(
    (params) => onJogStepper(oid, { forward: params.forward, speedHz: params.speedHz, steps: 0 }),
    [oid, onJogStepper],
  );

  // A running jog follows direction at once and speed after a moment's typing.
  useEffect(() => {
    if (!running || mode !== 'continuous') return undefined;
    const prev = paramsRef.current;
    if (prev.forward === forward && prev.speedHz === speedHz) return undefined;
    const onlySpeed = prev.forward === forward;
    paramsRef.current = { forward, speedHz };
    const timer = setTimeout(
      () => applyContinuousJog({ forward, speedHz }).catch(() => setStepperRunning(oid, false)),
      onlySpeed ? 150 : 0,
    );
    return () => clearTimeout(timer);
  }, [forward, speedHz, running, mode, applyContinuousJog, oid, setStepperRunning]);

  const run = async () => {
    setBusy(true);
    try {
      paramsRef.current = { forward, speedHz };
      await onJogStepper(oid, { forward, speedHz, steps: mode === 'distance' ? steps : 0 });
      setStepperRunning(oid, mode === 'continuous');
    } finally {
      setBusy(false);
    }
  };

  const stop = async () => {
    setBusy(true);
    try {
      await onStopStepper(oid);
    } finally {
      setBusy(false);
    }
  };

  const changeMode = async (next) => {
    if (running && mode === 'continuous' && next !== 'continuous') await stop();
    setMode(next);
  };

  return (
    <Block title="Jog the motor">
      <div className="flex flex-wrap gap-2">
        <Segmented label="Direction" value={forward} options={DIRECTIONS} onChange={setForward} />
        <Segmented label="Mode" value={mode} options={MODES} onChange={changeMode} />
      </div>
      <Stepper
        id={`speed-${oid}`}
        label="Speed"
        value={speedHz}
        onChange={setSpeedHz}
        step={100}
        min={1}
        max={50000}
        unit="steps/s"
      />
      {mode === 'distance' && (
        <NumberField
          id={`steps-${oid}`}
          label="Steps"
          value={steps}
          min={1}
          max={1000000}
          onChange={setSteps}
        />
      )}
      <div className="flex flex-wrap items-center gap-2">
        <Button disabled={busy || (mode === 'continuous' && running)} onClick={run}>
          {mode === 'continuous' ? 'Run' : `Run ${steps} steps`}
        </Button>
        {mode === 'continuous' && running && (
          <Button variant="outline" disabled={busy} onClick={stop}>
            Stop
          </Button>
        )}
        {liveReading?.value != null && (
          <span className="ml-auto text-sm text-muted-foreground">
            at <span className="font-mono text-foreground">{Number(liveReading.value)}</span> steps
          </span>
        )}
      </div>
    </Block>
  );
}
