import { useState } from 'react';

import NumberField from '@/components/NumberField/NumberField';
import { Button } from '@/components/ui/button';
import { Slider } from '@/components/ui/slider';
import { useMCUSession } from '@/context/MCUSessionContext';
import {
  clampServoAngle,
  DEFAULT_SERVO_ANGLE,
  FAST_FACTOR,
  SLOW_FACTOR,
} from '@/helpers/servoGroups';

import { BenchCard, ScaleButtons } from '../BenchUi';
import PCA9685Section from './PCA9685Section';
import ServoGroupsSection from './ServoGroupsSection';

const servoLabel = (obj) => obj.label || obj.slot_id || obj.section || `object ${obj.oid}`;

/** Bench › Servos: every servo at once and one by one, groups of them, and PCA9685 boards. */
export default function GlobalServosPanel({
  pcaBoards = [],
  configBusy = false,
  onAddPCA9685Board,
}) {
  const {
    connected,
    servoObjects,
    servoAngles,
    setServo,
    setAllServosAngle,
    flipAllServos,
    scaleAllServoAngles,
    pcaStatus,
    checkPCA9685Boards,
  } = useMCUSession();
  const [busy, setBusy] = useState(false);
  const [applyAngle, setApplyAngle] = useState(DEFAULT_SERVO_ANGLE);
  const [moveError, setMoveError] = useState('');
  const none = !connected || servoObjects.length === 0;

  const run = async (action) => {
    setBusy(true);
    setMoveError('');
    try {
      await action();
    } catch (err) {
      setMoveError(err.message || 'The servo move failed');
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="grid items-start gap-4 xl:grid-cols-2">
      <BenchCard
        title="All servos"
        about={
          connected
            ? `${servoObjects.length} servo${servoObjects.length === 1 ? '' : 's'} in the connected config.`
            : 'Connect the board to move them. Groups can be edited without it.'
        }
      >
        {moveError && <p className="text-sm text-destructive">{moveError}</p>}
        <div className="flex flex-wrap items-end gap-2">
          <NumberField
            id="all-servo-angle"
            label="Angle"
            value={applyAngle}
            min={0}
            max={180}
            unit="°"
            disabled={busy || !connected}
            onChange={(v) => setApplyAngle(clampServoAngle(v))}
          />
          <Button disabled={busy || none} onClick={() => run(() => setAllServosAngle(applyAngle))}>
            Move all
          </Button>
          <Button
            variant="outline"
            disabled={busy || none}
            onClick={() => run(() => setAllServosAngle(DEFAULT_SERVO_ANGLE))}
          >
            Center all
          </Button>
          <Button variant="outline" disabled={busy || none} onClick={() => run(flipAllServos)}>
            Flip all
          </Button>
        </div>
        <ScaleButtons
          lower="Lower"
          higher="Raise"
          lowFactor={SLOW_FACTOR}
          highFactor={FAST_FACTOR}
          disabled={busy || none}
          onScale={(f) => run(() => scaleAllServoAngles(f))}
        />
        {connected && servoObjects.length > 0 && (
          <div>
            <p className="mb-2 text-sm font-medium">One by one</p>
            <ul className="divide-y divide-border rounded-md border border-border">
              {servoObjects.map((obj) => (
                <ServoRow
                  key={obj.oid}
                  obj={obj}
                  angle={servoAngles[obj.oid] ?? DEFAULT_SERVO_ANGLE}
                  busy={busy}
                  onApply={(angle) => run(() => setServo(obj.oid, angle))}
                />
              ))}
            </ul>
          </div>
        )}
      </BenchCard>

      <ServoGroupsSection connected={connected} busy={busy} onRun={run} />

      {onAddPCA9685Board && (
        <PCA9685Section
          boards={pcaBoards}
          connected={connected}
          busy={configBusy}
          onAdd={onAddPCA9685Board}
          status={pcaStatus}
          onCheck={checkPCA9685Boards}
        />
      )}
    </div>
  );
}

function ServoRow({ obj, angle, busy, onApply }) {
  const [local, setLocal] = useState(angle);
  // Follow the angle when it's moved elsewhere (all, a group).
  const [shown, setShown] = useState(angle);
  if (angle !== shown) {
    setShown(angle);
    setLocal(angle);
  }
  return (
    <li className="flex flex-wrap items-center gap-3 px-3 py-2">
      <span className="w-28 truncate text-sm">{servoLabel(obj)}</span>
      <Slider
        min={0}
        max={180}
        step={1}
        value={[local]}
        disabled={busy}
        aria-label={`${servoLabel(obj)} angle`}
        className="min-w-32 flex-1"
        onValueChange={([v]) => setLocal(v)}
      />
      <span className="w-10 text-right font-mono text-sm">{local}°</span>
      <Button
        size="sm"
        variant="outline"
        disabled={busy || local === angle}
        onClick={() => onApply(local)}
      >
        Move
      </Button>
    </li>
  );
}
