import { Navigate, useParams } from 'react-router';

import NumberField from '@/components/NumberField/NumberField';
import ObjectSelect from '@/components/ObjectSelect/ObjectSelect';
import SettingField from '@/components/SettingField/SettingField';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Progress } from '@/components/ui/progress';
import { sentence } from '@/helpers/text';
import { useMachineMotors } from '@/helpers/useMachineMotors';
import { cn } from '@/lib/utils';

import { useChutePageLogic } from './ChutePage.logic';
import CalibrationPanel from './components/CalibrationPanel';
import LayersTab from './components/LayersTab';
import TowerMap from './components/TowerMap';

const PARTS = ['calibrate', 'layers', 'test'];

function Stat({ label, value, note }) {
  return (
    <div className="rounded-xl border border-border bg-card px-4 py-3">
      <p className="text-sm text-muted-foreground">{label}</p>
      <p className="font-mono text-xl font-medium">{value}</p>
      {note && <p className="truncate text-xs text-muted-foreground">{note}</p>}
    </div>
  );
}

/**
 * Machine › Chute: Calibrate (home, aim, chute settings), Layers (doors, bins per layer,
 * motor and gearing) and Test bins, at /machine/chute/<part>.
 */
const ChutePage = () => {
  const { part = 'calibrate' } = useParams();
  const logic = useChutePageLogic();
  const motors = useMachineMotors();
  const { state, status, distributor, busy, sorting, layers, geometry } = logic;
  const homed = Boolean(status?.homed);
  const canMove = Boolean(state?.hardware?.connected) && !busy && !sorting;

  if (!PARTS.includes(part)) return <Navigate to="/machine/chute" replace />;

  return (
    <div className="ds flex flex-col gap-4">
      {/* The chute now, and homing it. */}
      <section className="flex flex-col gap-3 rounded-xl border border-border bg-card px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
        <p className="text-sm">
          {!logic.liveConnected
            ? 'Connecting to the backend…'
            : state?.hardware?.connected
              ? `Board on ${state.hardware.port}.`
              : sentence(state?.hardware?.message) || 'No board connected.'}{' '}
          {sorting && 'Moving the chute by hand waits until the sort stops.'}
        </p>
        <div className="flex shrink-0 gap-2">
          <Button
            variant="outline"
            disabled={!canMove}
            onClick={logic.zero}
            title="Use where the chute points now as 0°, without the home switch"
          >
            Set 0° here
          </Button>
          <Button
            disabled={busy || state?.state === 'running' || !state?.hardware?.connected}
            onClick={logic.home}
          >
            {homed ? 'Home again' : 'Home'}
          </Button>
        </div>
      </section>

      {state?.job && (
        <section className="flex flex-col gap-2 rounded-xl border border-border bg-card px-4 py-3">
          <div className="flex items-center justify-between gap-2">
            <p className="text-sm">
              <span className="font-medium capitalize">{state.job.name}</span>
              {state.job.detail && (
                <span className="text-muted-foreground"> · {state.job.detail}</span>
              )}
            </p>
            <Button variant="outline" size="sm" onClick={logic.cancel}>
              Cancel
            </Button>
          </div>
          {state.job.total > 0 && <Progress value={(state.job.step / state.job.total) * 100} />}
        </section>
      )}
      {!state?.job && state?.last_job?.error && (
        <p className="text-sm text-destructive">
          {state.last_job.name} failed: {state.last_job.error}
        </p>
      )}
      {state?.error && <p className="text-sm text-destructive">{state.error}</p>}
      {logic.error && <p className="text-sm text-destructive">{logic.error}</p>}

      <div className="grid gap-3 sm:grid-cols-3">
        <Stat
          label="Chute"
          value={homed && status.angle != null ? `${status.angle.toFixed(1)}°` : 'Not homed'}
          note="clockwise from home"
        />
        <Stat
          label="Home switch"
          value={status?.endstop ? (status.endstop === 'triggered' ? 'Pressed' : 'Open') : '—'}
          note={sentence(status?.message) || distributor.endstop}
        />
        <Stat
          label="Each section"
          value={`${distributor.sectionWidth.toFixed(2)}°`}
          note={`first starts ${distributor.firstSectionOffset.toFixed(2)}° from home`}
        />
      </div>

      {part === 'calibrate' && (
        <div className="grid items-start gap-4 xl:grid-cols-2">
          <CalibrationPanel logic={logic} canMove={canMove && homed} />
          <section className="flex flex-col gap-2 rounded-xl border border-border bg-card px-5 py-4">
            <h2 className="font-display text-xl font-semibold">Chute settings</h2>
            <div className="flex flex-wrap gap-4 py-2">
              <label className="flex flex-col gap-1.5 text-sm">
                <span className="font-medium">Home switch</span>
                <ObjectSelect
                  value={distributor.endstop}
                  objects={logic.endstopObjects}
                  allowNone
                  onChange={(endstop) => logic.setDistributor({ endstop })}
                />
              </label>
              <NumberField
                id="chute-max"
                label="Travel limit"
                value={distributor.maxAngle}
                min={1}
                max={360}
                unit="°"
                onChange={(maxAngle) => logic.setDistributor({ maxAngle })}
              />
              <NumberField
                id="chute-sections"
                label="Sections per layer"
                value={distributor.sections}
                min={1}
                max={12}
                onChange={(v) => logic.setDistributor({ sections: Math.round(v) })}
              />
            </div>
            <label className="flex items-center gap-2 pb-2 text-sm">
              <Checkbox
                checked={distributor.forward}
                onCheckedChange={(v) => logic.setDistributor({ forward: Boolean(v) })}
              />
              The motor’s forward turns the chute clockwise, seen from above
            </label>
            <div className="divide-y divide-border border-t border-border">
              <SettingField settingKey="distributor.homeRpm" />
              <SettingField settingKey="distributor.doorSettleMs" />
            </div>
            <p className="pt-2 text-sm text-muted-foreground">
              The home switch is an <code className="font-mono">endstop_pin</code> on the chute’s
              stepper section, under Machine › Board › Wiring.
            </p>
          </section>
        </div>
      )}

      {part === 'layers' && <LayersTab motors={motors} />}

      {part === 'test' && (
        <section className="flex flex-col gap-4 rounded-xl border border-border bg-card p-5">
          <div className="flex flex-wrap gap-2" role="tablist" aria-label="Layer">
            {layers.map((_, i) => (
              <button
                key={i}
                type="button"
                role="tab"
                aria-selected={i === logic.layerIndex}
                onClick={() => logic.setLayerIndex(i)}
                className={cn(
                  'h-9 rounded-full border px-4 text-sm font-medium',
                  i === logic.layerIndex
                    ? 'border-primary bg-primary text-primary-foreground'
                    : 'border-border text-muted-foreground hover:bg-accent',
                )}
              >
                Layer {i + 1}
              </button>
            ))}
          </div>
          <TowerMap
            bins={logic.layer}
            geometry={geometry}
            angle={homed ? status.angle : null}
            disabled={!canMove || !homed}
            onBin={logic.sendToBin}
          />
          <p className="text-center text-sm text-muted-foreground">
            {homed
              ? 'Pick a bin to open its layer and aim the chute at it.'
              : 'Home the chute to aim at bins.'}
          </p>
          <div className="flex flex-wrap items-end justify-center gap-3 border-t border-border pt-4">
            <NumberField
              id="test-dwell"
              label="Pause at each"
              value={logic.testDwellMs}
              min={0}
              step={250}
              unit="ms"
              onChange={logic.setTestDwellMs}
            />
            <Button disabled={!canMove || !homed} onClick={logic.testLayer}>
              Test every bin on layer {logic.layerIndex + 1}
            </Button>
          </div>
        </section>
      )}
    </div>
  );
};

export default ChutePage;
