import { Button } from '@/components/ui/button';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';

import { JOG_STEPS } from '../ChutePage.logic';

const fmt = (angle) => (angle == null ? '—' : `${angle.toFixed(2)}°`);

/**
 * Two captures set the aim for every bin: jog to the middle of a section's first bin,
 * capture, jog to its last bin, capture, and work out the section width and offset.
 */
export default function CalibrationPanel({ logic, canMove }) {
  const { captures, calibration, calLayer, calibrationLayers, distributor } = logic;
  const perSection = calLayer?.perSection;

  return (
    <section className="flex flex-col gap-4 rounded-xl border border-border bg-card p-5">
      <h2 className="font-display text-xl font-semibold">Calibrate the aim</h2>
      <ol className="flex list-decimal flex-col gap-2 pl-5 text-sm marker:text-muted-foreground">
        <li>Home the chute.</li>
        <li>
          <span className="inline-flex flex-wrap items-center gap-2">
            Jog it to the middle of the <strong>first bin</strong> of a section on layer
            <Select
              value={calLayer ? String(calLayer.index) : undefined}
              onValueChange={(v) => logic.setCalibrationLayer(Number(v))}
              disabled={calibrationLayers.length === 0}
            >
              <SelectTrigger size="sm" className="w-52">
                <SelectValue placeholder="No layer fits" />
              </SelectTrigger>
              <SelectContent>
                {calibrationLayers.map((l) => (
                  <SelectItem key={l.index} value={String(l.index)}>
                    {l.index + 1} ({l.perSection} per section)
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            and capture it.
          </span>
        </li>
        <li>
          Jog clockwise to the middle of the <strong>last bin</strong> of the same section
          {perSection ? ` (bin ${perSection} of it)` : ''} and capture it.
        </li>
        <li>Work it out, check the numbers, and use them.</li>
      </ol>

      <div className="flex flex-wrap gap-2" role="group" aria-label="Jog the chute">
        {JOG_STEPS.map((deg) => (
          <Button
            key={deg}
            variant="outline"
            className="font-mono"
            disabled={!canMove}
            onClick={() => logic.jog(deg)}
          >
            {deg > 0 ? `+${deg}°` : `−${-deg}°`}
          </Button>
        ))}
      </div>

      <div className="grid gap-2 sm:grid-cols-2">
        {['first', 'last'].map((which) => (
          <div
            key={which}
            className="flex items-center justify-between gap-2 rounded-lg border border-border px-3 py-2"
          >
            <Button
              variant="outline"
              size="sm"
              disabled={!canMove}
              onClick={() => logic.capture(which)}
            >
              Capture the {which} bin
            </Button>
            <span className="font-mono">{fmt(captures[which])}</span>
          </div>
        ))}
      </div>
      <div>
        <Button
          disabled={captures.first == null || captures.last == null || !calLayer}
          onClick={logic.workItOut}
        >
          Work it out
        </Button>
      </div>

      {calibration && (
        <div className="flex flex-col gap-2 rounded-lg border border-primary/40 bg-accent p-4 text-sm">
          <p>
            Section width{' '}
            <strong className="font-mono">{calibration.section_width.toFixed(2)}°</strong>{' '}
            <span className="text-muted-foreground">
              (now {distributor.sectionWidth.toFixed(2)}°)
            </span>
          </p>
          <p>
            First section starts{' '}
            <strong className="font-mono">{calibration.first_section_offset.toFixed(2)}°</strong>{' '}
            from home{' '}
            <span className="text-muted-foreground">
              (now {distributor.firstSectionOffset.toFixed(2)}°)
            </span>
          </p>
          <div>
            <Button onClick={logic.applyCalibration}>Use these</Button>
          </div>
        </div>
      )}
    </section>
  );
}
