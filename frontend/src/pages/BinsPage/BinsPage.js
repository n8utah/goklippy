import { useMemo, useState } from 'react';
import { Link, Navigate, useParams } from 'react-router';

import NumberField from '@/components/NumberField/NumberField';
import SettingField from '@/components/SettingField/SettingField';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { Progress } from '@/components/ui/progress';
import { Sheet, SheetContent } from '@/components/ui/sheet';
import { Switch } from '@/components/ui/switch';
import { cn } from '@/lib/utils';

import { ROUTE_REASONS, useBinsPageLogic } from './BinsPage.logic';
import BinEditor from './components/BinEditor';
import IdentifyPanel from './components/IdentifyPanel';

const PARTS = ['layout', 'lookup'];
const binLabel = (bin) => `layer ${bin.layer}, bin ${bin.bin} (section ${bin.section})`;

function RouteTester({ route, setRoute, onTry, canTry }) {
  const { result } = route;
  const set = (key) => (e) =>
    setRoute((prev) => ({ ...prev, [key]: e.target.value, result: null }));
  const field = (key, label, props) => (
    <label htmlFor={`route-${key}`} className="flex flex-col gap-1.5 text-sm">
      <span className="font-medium">{label}</span>
      <Input
        id={`route-${key}`}
        value={route[key]}
        onChange={set(key)}
        className="w-36"
        {...props}
      />
    </label>
  );
  return (
    <section className="flex flex-col gap-4 rounded-xl border border-border bg-card p-5">
      <div>
        <h2 className="font-display text-xl font-semibold">Where would a piece go?</h2>
        <p className="text-sm text-muted-foreground">
          The bin the collection’s profile would choose for a part and color (and, sorting by part
          type, Brickognize’s category). Nothing is saved.
        </p>
      </div>
      <form
        className="flex flex-wrap items-end gap-3"
        onSubmit={(e) => {
          e.preventDefault();
          onTry();
        }}
      >
        {field('part', 'Part', { placeholder: '3001' })}
        {field('color', 'Color id', { placeholder: '5' })}
        {field('category', 'Part type', { placeholder: 'Plate, Round' })}
        {field('size', 'Longest side (mm)', { type: 'number', min: 0, placeholder: 'optional' })}
        <Button type="submit" disabled={!canTry || !route.part.trim()}>
          Check
        </Button>
      </form>
      {result && (
        <p className="rounded-lg bg-accent px-4 py-3 text-sm">
          <strong>{result.category_name}</strong> →{' '}
          {result.choice.bin ? binLabel(result.choice.bin) : 'the bucket'}:{' '}
          {ROUTE_REASONS[result.choice.reason] ?? result.choice.reason}.
        </p>
      )}
    </section>
  );
}

function Layer({ layer, logic }) {
  const { selected } = logic;
  return (
    <section
      className={cn(
        'flex flex-col gap-4 rounded-xl border border-border bg-card p-5',
        layer.disabled && 'opacity-70',
      )}
    >
      <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h2 className="font-display text-xl font-semibold">Layer {layer.layer}</h2>
          <p className="text-sm text-muted-foreground">
            {layer.bins} bins{layer.max_piece_mm ? `, pieces up to ${layer.max_piece_mm} mm` : ''}
          </p>
        </div>
        <div className="flex flex-wrap items-end gap-4">
          <NumberField
            id={`layer-${layer.layer}-full`}
            label="Full at (0: never)"
            value={layer.max_pieces}
            min={0}
            unit="pieces"
            inputClassName="w-20"
            onChange={(v) => logic.setLayer(layer.layer, { maxPieces: Math.round(v) })}
          />
          <label className="flex h-10 items-center gap-2 text-sm font-medium">
            <Switch
              checked={!layer.disabled}
              onCheckedChange={(on) => logic.setLayer(layer.layer, { disabled: !on })}
            />
            In use
          </label>
        </div>
      </div>
      {layer.sections.length === 0 ? (
        <p className="text-sm text-muted-foreground">
          {layer.bins} bins don’t split into the chute’s sections; fix the bin count under{' '}
          <Link
            to="/machine/chute/layers"
            className="font-medium text-primary-text hover:underline"
          >
            Machine › Chute › Layers
          </Link>
          .
        </p>
      ) : (
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
          {layer.sections.map((section) => (
            <div
              key={section.section}
              className={cn(
                'flex flex-col gap-2 rounded-lg border border-border p-3',
                section.disabled && 'bg-muted/50',
              )}
            >
              <label className="flex items-center gap-2 text-sm font-medium">
                <Checkbox
                  checked={!section.disabled}
                  onCheckedChange={() => logic.toggleSection(layer.layer, section.section)}
                />
                Section {section.section}
              </label>
              <div className="grid grid-cols-3 gap-1.5">
                {section.bins.map((bin) => {
                  const isSelected = selected?.layer === layer.layer && selected?.bin === bin.bin;
                  const cats = bin.categories.map((c) => c.name).join(', ');
                  return (
                    <button
                      key={bin.bin}
                      type="button"
                      disabled={!logic.view.profile}
                      onClick={() => logic.select({ layer: layer.layer, bin: bin.bin })}
                      title={cats || undefined}
                      className={cn(
                        'flex min-h-16 flex-col items-start gap-0.5 rounded-md border px-2 py-1.5 text-left text-xs transition-colors hover:border-border-strong disabled:cursor-default',
                        isSelected ? 'border-primary bg-accent' : 'border-border',
                        !bin.reachable && 'opacity-50',
                        bin.full && 'border-warning bg-warning-bg',
                      )}
                    >
                      <span className="flex w-full items-center justify-between gap-1">
                        <span className="font-mono font-medium">{bin.bin}</span>
                        {bin.count > 0 && (
                          <span
                            className={cn(
                              'font-mono',
                              bin.full ? 'text-warning' : 'text-muted-foreground',
                            )}
                          >
                            {bin.full ? `full ${bin.count}` : bin.count}
                          </span>
                        )}
                      </span>
                      <span
                        className={cn(
                          'line-clamp-2',
                          !bin.categories.length && 'text-muted-foreground',
                        )}
                      >
                        {cats || (bin.reachable ? 'Unassigned' : 'Out of reach')}
                      </span>
                    </button>
                  );
                })}
              </div>
            </div>
          ))}
        </div>
      )}
    </section>
  );
}

/**
 * Rules › Bins: Layout (each layer's bins, what they take and how full they are) and
 * Where would it go (/rules/bins/lookup).
 */
const BinsPage = () => {
  const { part = 'layout' } = useParams();
  const logic = useBinsPageLogic();
  const { view, selected, selectedBin } = logic;
  const [layerTab, setLayerTab] = useState(1);

  // Which bins each category is in, for "now in …" in the editor.
  const binOf = useMemo(() => {
    const out = {};
    for (const layer of view?.layers ?? []) {
      for (const section of layer.sections) {
        for (const bin of section.bins) {
          for (const c of bin.categories)
            (out[c.id] ??= []).push({ layer: layer.layer, bin: bin.bin });
        }
      }
    }
    return out;
  }, [view]);

  if (!PARTS.includes(part)) return <Navigate to="/rules/bins" replace />;
  if (!view) {
    return (
      <div className="ds text-sm text-muted-foreground">
        {logic.error ? <span className="text-destructive">{logic.error}</span> : 'Loading…'}
      </div>
    );
  }
  const layer = view.layers.find((l) => l.layer === layerTab) ?? view.layers[0];

  return (
    <div className="ds flex flex-col gap-4">
      <section className="flex flex-col gap-3 rounded-xl border border-border bg-card px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
        <p className="text-sm">
          {!view.collection ? (
            <>
              No collection is picked: these are a collection’s bins.{' '}
              <Link to="/run/sort" className="font-medium text-primary-text hover:underline">
                Pick one on Run › Sort
              </Link>
            </>
          ) : view.profile ? (
            <>
              <Link
                to={`/run/collections/${view.collection.id}`}
                className="font-semibold hover:underline"
              >
                {view.collection.name}
              </Link>
              ’s bins, sorting with <strong>{view.profile.name}</strong>. A category with no bin
              gets the first free one when its first piece arrives.
            </>
          ) : (
            <>
              {view.collection.name} has no sorting profile, so bins can’t be assigned.{' '}
              <Link
                to={`/run/collections/${view.collection.id}`}
                className="font-medium text-primary-text hover:underline"
              >
                Pick its profile
              </Link>
            </>
          )}
        </p>
        {part === 'layout' && (
          <div className="flex shrink-0 flex-wrap gap-2">
            <Button
              variant="outline"
              disabled={!view.profile || !view.collection}
              onClick={logic.preassignBins}
              title="Gives the profile’s categories, in order, the bins that hold no pieces"
            >
              Pre-assign from the profile
            </Button>
            <Button
              variant="outline"
              disabled={!view.profile || !view.collection}
              onClick={logic.resetBins}
            >
              Unassign every bin
            </Button>
          </div>
        )}
      </section>
      {logic.error && <p className="text-sm text-destructive">{logic.error}</p>}

      {part === 'layout' ? (
        <>
          <div className="flex flex-wrap gap-2" role="tablist" aria-label="Layer">
            {view.layers.map((l) => (
              <button
                key={l.layer}
                type="button"
                role="tab"
                aria-selected={l.layer === layer?.layer}
                onClick={() => setLayerTab(l.layer)}
                className={cn(
                  'h-9 rounded-full border px-4 text-sm font-medium',
                  l.layer === layer?.layer
                    ? 'border-primary bg-primary text-primary-foreground'
                    : 'border-border text-muted-foreground hover:bg-accent',
                )}
              >
                Layer {l.layer}
                {l.disabled ? ' (off)' : ''}
              </button>
            ))}
          </div>
          {layer && <Layer layer={layer} logic={logic} />}

          <div className="grid items-start gap-4 lg:grid-cols-2">
            <section className="rounded-xl border border-border bg-card px-5 py-2">
              <SettingField settingKey="distributor.sharedBins" />
            </section>
            {logic.kits.length > 0 && (
              <section className="flex flex-col gap-3 rounded-xl border border-border bg-card p-5">
                <h2 className="font-display text-lg font-semibold">Kits</h2>
                {logic.kits.map((k) => (
                  <div key={k.category} className="flex items-center gap-3 text-sm">
                    <span className="w-40 truncate">{k.name}</span>
                    <Progress value={(k.found / Math.max(1, k.need)) * 100} className="flex-1" />
                    <span className="font-mono text-xs text-muted-foreground">
                      {k.found}/{k.need}
                    </span>
                  </div>
                ))}
              </section>
            )}
          </div>

          <Sheet open={Boolean(selectedBin)} onOpenChange={(o) => !o && logic.select(null)}>
            <SheetContent side="right" className="w-full p-5 sm:max-w-md">
              {selectedBin && (
                <BinEditor
                  key={`${selected.layer}-${selected.bin}`}
                  bin={selectedBin}
                  layer={selected.layer}
                  categories={view.categories}
                  binOf={binOf}
                  onSave={logic.saveBin}
                  onPoint={logic.pointChute}
                  onEmpty={logic.emptyBin}
                />
              )}
            </SheetContent>
          </Sheet>
        </>
      ) : (
        <div className="grid items-start gap-4 xl:grid-cols-2">
          <RouteTester
            route={logic.route}
            setRoute={logic.setRoute}
            onTry={logic.tryRoute}
            canTry={Boolean(view.profile)}
          />
          <IdentifyPanel />
        </div>
      )}
    </div>
  );
};

export default BinsPage;
