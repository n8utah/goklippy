import { Plus, Trash2 } from 'lucide-react';

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
import { cn } from '@/lib/utils';

import EditableBoardDiagram from './EditableBoardDiagram';
import { usePictureEditorLogic } from './PictureEditor.logic';

/**
 * Board › Board picture: where each pin and connector sits on the board's photo, for the
 * Wiring tab. Drag overlays, add one for each slot, and save them as the board's defaults.
 */
export default function PictureEditor() {
  const logic = usePictureEditorLogic();
  const { hotspots, selectedHotspot: sel, selectedId } = logic;
  const set = (patch) => logic.updateHotspot(sel.id, patch);

  return (
    <div className="ds flex flex-col gap-4">
      <section className="flex flex-col gap-3 rounded-xl border border-border bg-card px-5 py-4 sm:flex-row sm:items-end sm:justify-between">
        <p className="max-w-[70ch] text-sm text-muted-foreground">
          Where each pin and connector sits on the photo, for picking them on the Wiring tab. Drag
          an overlay to move it and its corners to resize it, then save them as the board’s
          defaults.
        </p>
        <div className="flex shrink-0 items-end gap-2">
          <label className="flex flex-col gap-1.5 text-sm font-medium">
            Board
            <Select value={logic.profileId} onValueChange={logic.setProfileId}>
              <SelectTrigger className="w-36">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {logic.profileIds.map((id) => (
                  <SelectItem key={id} value={id}>
                    {id}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </label>
          <Button
            onClick={logic.saveHotspots}
            disabled={logic.saving || !logic.dirty || logic.loading}
          >
            {logic.saving ? 'Saving…' : logic.dirty ? 'Save as defaults' : 'Saved'}
          </Button>
        </div>
      </section>
      {logic.error && <p className="text-sm text-destructive">{logic.error}</p>}
      {logic.message && <p className="text-sm text-running-text">{logic.message}</p>}

      <div className="grid items-start gap-4 xl:grid-cols-[minmax(0,1fr)_340px]">
        <div className="flex flex-col gap-2">
          {logic.loading ? (
            <div className="flex aspect-[4/3] items-center justify-center rounded-xl border border-dashed border-border text-sm text-muted-foreground">
              Loading…
            </div>
          ) : (
            <EditableBoardDiagram
              diagram={logic.diagram}
              profileId={logic.profileId}
              hotspots={hotspots}
              selectedId={selectedId}
              onSelect={logic.setSelectedId}
              onBeginDrag={logic.beginDrag}
              onBeginResize={logic.beginResize}
              onMoveDrag={logic.moveDrag}
              onEndDrag={logic.endDrag}
            />
          )}
          <p className="text-sm text-muted-foreground">{logic.profileName || logic.profileId}</p>
        </div>

        <aside className="flex flex-col gap-4">
          {sel && (
            <section className="flex flex-col gap-3 rounded-xl border border-primary/50 bg-card p-4">
              <div className="flex items-start justify-between gap-2">
                <div className="min-w-0">
                  <h2 className="font-display font-semibold">Picked overlay</h2>
                  <p className="truncate font-mono text-xs text-muted-foreground">
                    {sel.id} · {(sel.slot_ids || []).join(', ')}
                  </p>
                </div>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  aria-label="Remove this overlay"
                  className="text-destructive"
                  onClick={() => logic.removeHotspot(sel.id)}
                >
                  <Trash2 />
                </Button>
              </div>
              <label htmlFor="overlay-label" className="flex flex-col gap-1.5 text-sm font-medium">
                Label
                <Input
                  id="overlay-label"
                  value={sel.label || ''}
                  onChange={(e) => set({ label: e.target.value })}
                />
              </label>
              {sel.shape !== 'circle' && (
                <div className="grid grid-cols-2 gap-3">
                  {[
                    ['x', 'X'],
                    ['y', 'Y'],
                    ['width', 'Width'],
                    ['height', 'Height'],
                  ].map(([key, label]) => (
                    <NumberField
                      key={key}
                      id={`overlay-${key}`}
                      label={label}
                      value={Number.isFinite(sel[key]) ? sel[key] : 0}
                      inputClassName="w-full"
                      onChange={(v) => set({ [key]: v })}
                    />
                  ))}
                </div>
              )}
            </section>
          )}

          <section className="rounded-xl border border-border bg-card">
            <h2 className="border-b border-border px-4 py-3 font-display font-semibold">
              Overlays ({hotspots.length})
            </h2>
            <ul className="max-h-72 overflow-y-auto p-1.5">
              {hotspots.length === 0 && (
                <li className="px-2 py-1.5 text-sm text-muted-foreground">None yet.</li>
              )}
              {hotspots.map((h) => (
                <li key={h.id}>
                  <button
                    type="button"
                    onClick={() => logic.setSelectedId(h.id)}
                    aria-current={selectedId === h.id ? 'true' : undefined}
                    className={cn(
                      'w-full truncate rounded-md px-2 py-1.5 text-left text-sm hover:bg-accent',
                      selectedId === h.id && 'bg-accent font-medium',
                    )}
                  >
                    {h.label}
                  </button>
                </li>
              ))}
            </ul>
          </section>

          <section className="rounded-xl border border-border bg-card">
            <h2 className="border-b border-border px-4 py-3 font-display font-semibold">
              Add from the board’s slots
            </h2>
            <ul className="max-h-72 overflow-y-auto p-1.5">
              {logic.availableSlots.length === 0 && (
                <li className="px-2 py-1.5 text-sm text-muted-foreground">
                  Every slot has an overlay.
                </li>
              )}
              {logic.availableSlots.map((slot) => (
                <li key={slot.id}>
                  <button
                    type="button"
                    onClick={() => logic.addHotspotForSlot(slot.id)}
                    className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm hover:bg-accent"
                  >
                    <Plus className="size-4 shrink-0 text-muted-foreground" aria-hidden />
                    <span className="truncate">{slot.label}</span>
                    <span className="ml-auto shrink-0 text-xs text-muted-foreground">
                      {slot.type}
                    </span>
                  </button>
                </li>
              ))}
            </ul>
          </section>
        </aside>
      </div>
    </div>
  );
}
