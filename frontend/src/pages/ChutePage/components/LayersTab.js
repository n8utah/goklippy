import { Plus, Trash2 } from 'lucide-react';

import GearCalibration from '@/components/GearCalibration/GearCalibration';
import NumberField from '@/components/NumberField/NumberField';
import ObjectSelect from '@/components/ObjectSelect/ObjectSelect';
import SettingField from '@/components/SettingField/SettingField';
import { Button } from '@/components/ui/button';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { cn } from '@/lib/utils';

/**
 * Chute › Layers: the layer doors (one servo each: the chosen layer opens, the rest close),
 * how many bins each layer has, and the chute's motor and gearing.
 */
export default function LayersTab({ motors }) {
  const { distributor, servoObjects, stepperObjects, connected } = motors;
  const { config, activeLayer, moving, oid } = distributor;

  return (
    <div className="grid items-start gap-4 xl:grid-cols-2">
      <section className="flex flex-col gap-4 rounded-xl border border-border bg-card p-5">
        <div>
          <h2 className="font-display text-xl font-semibold">Layers</h2>
          <p className="text-sm text-muted-foreground">
            Pick a layer to open its door and close the others.
          </p>
        </div>
        <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
          {distributor.layers.map((layer, i) => (
            <button
              key={i}
              type="button"
              disabled={!connected}
              onClick={() => motors.selectLayer(i)}
              aria-pressed={i === activeLayer}
              className={cn(
                'flex flex-col items-start rounded-lg border px-3 py-2 text-left text-sm transition-colors disabled:opacity-60',
                i === activeLayer ? 'border-primary bg-accent' : 'border-border hover:bg-accent',
              )}
            >
              <span className="font-medium">Layer {i + 1}</span>
              <span className="text-muted-foreground">{layer.bins} bins</span>
              <span className="text-xs text-muted-foreground">
                {layer.servoObject
                  ? i === activeLayer
                    ? `Open ${layer.openAngle}°`
                    : `Closed ${layer.closedAngle}°`
                  : 'No servo'}
              </span>
            </button>
          ))}
        </div>

        <ol className="flex flex-col gap-2">
          {distributor.layers.map((layer, i) => (
            <li
              key={i}
              className="flex flex-wrap items-end gap-3 rounded-lg border border-border px-3 py-3"
            >
              <span
                className="mb-1.5 flex size-8 items-center justify-center rounded-full bg-muted font-mono text-sm"
                aria-label={`Layer ${i + 1}`}
              >
                {i + 1}
              </span>
              <label className="flex flex-col gap-1.5 text-sm">
                <span className="font-medium">Door servo</span>
                <ObjectSelect
                  value={layer.servo}
                  objects={servoObjects}
                  allowNone
                  className="w-36"
                  onChange={(servo) => motors.setLayer(i, { servo })}
                />
              </label>
              <label className="flex flex-col gap-1.5 text-sm">
                <span className="font-medium">Bins</span>
                <Select
                  value={String(layer.bins)}
                  onValueChange={(v) => motors.setLayer(i, { bins: Number(v) })}
                >
                  <SelectTrigger className="w-20">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="18">18</SelectItem>
                    <SelectItem value="12">12</SelectItem>
                  </SelectContent>
                </Select>
              </label>
              <NumberField
                id={`layer-${i}-open`}
                label="Open"
                value={layer.openAngle}
                min={0}
                max={180}
                unit="°"
                inputClassName="w-16"
                onChange={(openAngle) => motors.setLayer(i, { openAngle })}
              />
              <NumberField
                id={`layer-${i}-closed`}
                label="Closed"
                value={layer.closedAngle}
                min={0}
                max={180}
                unit="°"
                inputClassName="w-16"
                onChange={(closedAngle) => motors.setLayer(i, { closedAngle })}
              />
              <Button
                variant="ghost"
                size="icon"
                className="ml-auto"
                disabled={distributor.layers.length <= 1}
                aria-label={`Remove layer ${i + 1}`}
                onClick={() => motors.removeLayer(i)}
              >
                <Trash2 />
              </Button>
            </li>
          ))}
        </ol>
        <div>
          <Button variant="outline" onClick={motors.addLayer}>
            <Plus /> Add layer
          </Button>
        </div>
      </section>

      <section className="flex flex-col gap-2 rounded-xl border border-border bg-card px-5 py-4">
        <div className="flex items-center justify-between">
          <h2 className="font-display text-xl font-semibold">Chute motor</h2>
          <span className="font-mono text-sm text-muted-foreground">
            {moving ? `moving ${distributor.measuredRpm.toFixed(1)} rpm` : 'stopped'}
          </span>
        </div>
        {connected && oid == null && (
          <p className="rounded-md bg-warning-bg px-3 py-2 text-sm text-warning">
            {config.stepper} isn’t in the connected config. Choose its motor, or add it under
            Machine › Board › Wiring.
          </p>
        )}
        <label className="flex flex-col gap-1.5 py-2 text-sm">
          <span className="font-medium">Motor</span>
          <ObjectSelect
            value={config.stepper}
            objects={stepperObjects}
            onChange={(stepper) => motors.setDistributor({ stepper })}
          />
        </label>
        <div className="divide-y divide-border border-y border-border">
          <SettingField settingKey="distributor.rpm" />
          <SettingField settingKey="distributor.accel" />
        </div>
        <div className="pt-4">
          <GearCalibration
            id="gear-chute"
            config={config}
            microsteps={distributor.microsteps}
            microstepsAssumed={distributor.microstepsAssumed}
            stepsPerRev={distributor.stepsPerRev}
            slot={{ degrees: distributor.sectionPitch, name: 'section' }}
            canTurn={connected && oid != null && !moving}
            onChange={motors.setDistributor}
            onTurn={motors.turnDistributorRev}
          />
        </div>
      </section>
    </div>
  );
}
