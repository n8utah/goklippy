import GearCalibration from '@/components/GearCalibration/GearCalibration';
import NumberField from '@/components/NumberField/NumberField';
import ObjectSelect from '@/components/ObjectSelect/ObjectSelect';
import SettingField from '@/components/SettingField/SettingField';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { indexAlignment } from '@/helpers/useMachineMotors';

import SpeedTrace from './SpeedTrace';

const NAMES = {
  c1: 'C1 · bulk feeder',
  c2: 'C2 · feeder',
  c3: 'C3 · feeder',
  c4: 'C4 · classification',
};
const NO_ROUTINE = '__none__';

function Stat({ label, value, note }) {
  return (
    <div>
      <p className="text-sm text-muted-foreground">{label}</p>
      <p className="font-mono text-2xl font-medium">{value}</p>
      {note && <p className="text-xs text-muted-foreground">{note}</p>}
    </div>
  );
}

/** One channel: live speed and controls, then its settings, motor and gearing. */
export default function ChannelTab({ id, logic }) {
  const channel = logic.channels[id];
  const { connected, settings } = logic;
  const c4 = id === 'c4';
  const missing = channel.oid == null;
  const busy = channel.running || channel.indexing || channel.routineRunning;
  const alignment = indexAlignment(channel.angle, settings.indexDegrees);
  const routineId = logic.routineIdFor(id);

  return (
    <div className="grid items-start gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(0,1.1fr)]">
      {/* Live and controls. */}
      <section className="flex flex-col gap-4 rounded-xl border border-border bg-card p-5">
        <div className="flex items-center justify-between gap-2">
          <h2 className="font-display text-xl font-semibold">{NAMES[id]}</h2>
          <Badge variant={channel.direction !== 0 ? 'running' : 'idle'}>
            {channel.direction > 0 ? 'Forward' : channel.direction < 0 ? 'Reverse' : 'Stopped'}
          </Badge>
        </div>
        {connected && missing && (
          <p className="rounded-md bg-warning-bg px-3 py-2 text-sm text-warning">
            {channel.config.stepper} isn’t in the connected config. Choose its motor below, or add
            it under Machine › Board › Wiring.
          </p>
        )}
        {channel.error && <p className="text-sm text-destructive">{channel.error}</p>}
        <div className="grid grid-cols-2 gap-4">
          <Stat
            label="Measured"
            value={`${channel.measuredRpm.toFixed(1)} rpm`}
            note={`set ${channel.targetRpm.toFixed(1)} rpm`}
          />
          {c4 && (
            <Stat
              label="Slot"
              value={`${alignment.slot + 1} of ${alignment.slots}`}
              note={`${alignment.offset >= 0 ? '+' : ''}${alignment.offset.toFixed(1)}° off`}
            />
          )}
        </div>
        <SpeedTrace values={logic.history[channel.oid] || []} stepsPerRev={channel.stepsPerRev} />

        <div className="flex flex-wrap gap-2">
          {channel.running ? (
            <Button onClick={() => logic.stopChannel(channel)}>Stop</Button>
          ) : (
            <Button
              disabled={!connected || missing || channel.indexing || channel.routineRunning}
              onClick={() => logic.startChannel(channel)}
            >
              Run
            </Button>
          )}
          {c4 &&
            (channel.indexing ? (
              <Button onClick={() => logic.stopChannel(channel)}>Stop indexing</Button>
            ) : (
              <>
                <Button
                  variant="outline"
                  disabled={!connected || missing || channel.direction !== 0}
                  onClick={() => logic.runIndex(1, true)}
                >
                  Forward 1 slot ({settings.indexDegrees}°)
                </Button>
                <Button
                  variant="outline"
                  disabled={!connected || missing || channel.running}
                  onClick={() => logic.runIndex(0)}
                >
                  Auto index
                </Button>
              </>
            ))}
          {c4 && (
            <Button variant="ghost" disabled={missing} onClick={() => logic.zeroChannel(channel)}>
              Set as slot 1
            </Button>
          )}
        </div>

        <div className="flex flex-wrap items-end gap-2 border-t border-border pt-4">
          <label className="flex flex-col gap-1.5 text-sm">
            <span className="font-medium">Routine</span>
            <Select
              value={routineId || NO_ROUTINE}
              onValueChange={(v) => logic.chooseRoutine(id, v === NO_ROUTINE ? '' : v)}
              disabled={logic.routines.length === 0}
            >
              <SelectTrigger className="w-52">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={NO_ROUTINE}>
                  {logic.routines.length === 0 ? 'No routines yet' : 'None'}
                </SelectItem>
                {logic.routines.map((r) => (
                  <SelectItem key={r.id} value={r.id}>
                    {r.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </label>
          {channel.routineRunning ? (
            <Button onClick={() => logic.stopChannel(channel)}>Stop routine</Button>
          ) : (
            <Button
              variant="outline"
              disabled={!routineId || !connected || missing || busy}
              onClick={() => logic.runRoutine(channel)}
            >
              Run routine
            </Button>
          )}
        </div>
      </section>

      {/* Settings. */}
      <section className="flex flex-col gap-2 rounded-xl border border-border bg-card px-5 py-4">
        <h2 className="font-display text-lg font-semibold">Settings</h2>
        <div className="divide-y divide-border">
          {c4 ? (
            <>
              <SettingField settingKey="indexRpm" />
              <SettingField settingKey="indexAccel" />
            </>
          ) : (
            <SettingField settingKey={`feederRpm.${id}`} />
          )}
        </div>
        <div className="flex flex-wrap gap-4 border-t border-border py-4">
          <label className="flex flex-col gap-1.5 text-sm">
            <span className="font-medium">Motor</span>
            <ObjectSelect
              value={channel.config.stepper}
              objects={logic.stepperObjects}
              onChange={(stepper) => logic.setChannelConfig(id, { stepper })}
            />
          </label>
          <label className="flex flex-col gap-1.5 text-sm">
            <span className="font-medium">Reversed</span>
            <span className="flex h-10 items-center gap-2">
              <Switch
                checked={!channel.config.forward}
                onCheckedChange={(rev) => logic.setChannelConfig(id, { forward: !rev })}
                aria-label="Reversed"
              />
              <span className="text-muted-foreground">
                {channel.config.forward ? 'Forward' : 'Reverse'}
              </span>
            </span>
          </label>
          {c4 ? (
            <>
              <NumberField
                id="c4-index"
                label="Per index"
                value={settings.indexDegrees}
                min={1}
                max={360}
                unit="°"
                onChange={(v) => logic.setSetting('indexDegrees', v)}
              />
              <NumberField
                id="c4-dwell"
                label="Dwell"
                value={settings.dwellMs}
                min={0}
                step={100}
                unit="ms"
                onChange={(v) => logic.setSetting('dwellMs', v)}
              />
            </>
          ) : (
            <NumberField
              id={`${id}-sections`}
              label="Rotor sections"
              value={channel.config.sections ?? 0}
              min={0}
              max={36}
              onChange={(sections) => logic.setChannelConfig(id, { sections })}
            />
          )}
        </div>
        <div className="border-t border-border pt-4">
          <GearCalibration
            id={`gear-${id}`}
            config={channel.config}
            microsteps={channel.microsteps}
            microstepsAssumed={channel.microstepsAssumed}
            stepsPerRev={channel.stepsPerRev}
            slot={c4 ? { degrees: settings.indexDegrees, name: 'slot' } : null}
            canTurn={connected && !missing && channel.direction === 0 && !busy}
            onChange={(patch) => logic.setChannelConfig(id, patch)}
            onTurn={() => logic.turnChannelRev(channel)}
          />
        </div>
      </section>
    </div>
  );
}
