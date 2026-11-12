import NumberField from '@/components/NumberField/NumberField';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { sorterCameraFindImageURL } from '@/helpers/api';
import { cn } from '@/lib/utils';

import { ALL_CHANNELS, CHANNEL_LABELS, deviceLabel, useCameraFinder } from '../CamerasPage.logic';

const pct = (v) => `${(v * 100).toFixed(1)}%`;
const label = (ch) => CHANNEL_LABELS[ch] ?? 'C-channel 1';

/** A picture with the compared middle outlined. */
function FindPicture({ channel, which, stamp, centerShare, caption }) {
  const margin = `${((1 - centerShare) / 2) * 100}%`;
  return (
    <figure className="flex flex-col gap-1">
      <div className="relative overflow-hidden rounded-md bg-media">
        <img
          src={sorterCameraFindImageURL(channel, which, stamp)}
          alt={caption}
          className="w-full"
        />
        <span
          className="absolute border-2 border-dashed border-primary"
          style={{ inset: margin }}
        />
      </div>
      <figcaption className="text-xs text-muted-foreground">{caption}</figcaption>
    </figure>
  );
}

function FindResult({ finder, logic }) {
  const r = finder.result;
  const name = (id) => {
    const d = logic.devices.list.find((x) => x.id === id);
    return d ? deviceLabel(d) : 'a camera no longer listed';
  };
  const proposed = Object.entries(r.proposed ?? {});
  return (
    <div className="flex flex-col gap-3 border-t border-border pt-4">
      <p className="text-xs text-muted-foreground">
        Searched {new Date(r.finished).toLocaleString()}.
      </p>
      {r.error && <p className="text-sm text-destructive">{r.error}</p>}
      {r.notes?.map((n) => (
        <p key={n} className="text-sm text-warning">
          {n}
        </p>
      ))}

      {proposed.length > 0 && (
        <>
          <h4 className="font-medium">Proposed</h4>
          {proposed.map(([ch, id]) => (
            <div key={ch} className="flex flex-col gap-2">
              <p className="text-sm font-medium">
                {label(ch)}: {name(id)}
              </p>
              <div className="grid max-w-xl grid-cols-2 gap-2">
                <FindPicture
                  channel={ch}
                  which="before"
                  stamp={finder.stamp}
                  centerShare={r.center_share}
                  caption="Before the turn"
                />
                <FindPicture
                  channel={ch}
                  which="after"
                  stamp={finder.stamp}
                  centerShare={r.center_share}
                  caption="After the turn"
                />
              </div>
            </div>
          ))}
          <div className="flex flex-wrap items-center gap-3">
            <Button onClick={finder.apply}>Use these</Button>
            <span className="text-sm text-muted-foreground">
              Cameras not proposed keep their channel.
            </span>
          </div>
        </>
      )}

      {r.scores && (
        <details>
          <summary className="cursor-pointer text-sm text-muted-foreground">
            How much each picture changed
          </summary>
          <div className="mt-2 overflow-x-auto">
            <table className="text-sm">
              <thead>
                <tr className="border-b border-border text-left text-muted-foreground">
                  <th className="py-1 pr-4 font-medium">Turned</th>
                  {r.devices.map((d) => (
                    <th key={d} className="py-1 pr-4 font-medium">
                      {name(d)}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody className="font-mono">
                {ALL_CHANNELS.filter((ch) => r.scores[ch]).map((ch) => (
                  <tr key={ch} className="border-b border-border">
                    <td className="py-1 pr-4 font-sans">{label(ch)}</td>
                    {r.devices.map((d) => (
                      <td
                        key={d}
                        className={cn(
                          'py-1 pr-4',
                          r.proposed?.[ch] === d && 'font-semibold text-running-text',
                        )}
                      >
                        {pct(r.scores[ch][d] ?? 0)}
                      </td>
                    ))}
                  </tr>
                ))}
                <tr>
                  <td className="py-1 pr-4 font-sans">Nothing moving</td>
                  {r.devices.map((d) => (
                    <td key={d} className="py-1 pr-4">
                      {pct(r.noise?.[d] ?? 0)}
                    </td>
                  ))}
                </tr>
              </tbody>
            </table>
          </div>
        </details>
      )}
    </div>
  );
}

/** Finds which camera is where by turning each channel a little while watching them. */
export default function CameraFinder({ logic }) {
  const finder = useCameraFinder(logic);
  const { devices } = logic;
  return (
    <section className="flex flex-col gap-4 rounded-xl border border-border bg-card p-5">
      <div>
        <h2 className="font-display text-xl font-semibold">Find which camera is where</h2>
        <p className="max-w-[75ch] text-sm text-muted-foreground">
          Turns C1–C4 one at a time a few degrees and back while the ticked cameras watch; the
          camera whose picture changes most is over that channel. Only the middle of each picture
          counts, so a neighbouring channel at the edge doesn’t. Something on each channel has to
          show the turn: a piece or a strip of tape.
        </p>
      </div>
      <div className="flex flex-wrap gap-x-6 gap-y-2">
        {devices.list.map((d) => (
          <label key={d.id} className="flex items-center gap-2 text-sm">
            <Checkbox
              checked={finder.isPicked(d.id)}
              onCheckedChange={() => finder.toggle(d.id)}
              disabled={finder.running}
            />
            {deviceLabel(d)}
          </label>
        ))}
      </div>
      <div className="flex flex-wrap items-end gap-3">
        <NumberField
          id="find-degrees"
          label="Turn each channel"
          value={finder.degrees}
          min={1}
          max={90}
          unit="°"
          disabled={finder.running}
          onChange={finder.setDegrees}
        />
        {finder.running ? (
          <Button variant="outline" onClick={finder.cancel}>
            Cancel
          </Button>
        ) : (
          <Button disabled={Boolean(finder.blocked)} onClick={finder.start}>
            Find cameras
          </Button>
        )}
      </div>
      {finder.blocked && !finder.running && (
        <p className="text-sm text-muted-foreground">{finder.blocked}</p>
      )}
      {finder.job && (
        <p className="text-sm">
          {finder.job.total > 0 && `Step ${finder.job.step} of ${finder.job.total}: `}
          {finder.job.detail}
        </p>
      )}
      {finder.error && <p className="text-sm text-destructive">{finder.error}</p>}
      {finder.result && !finder.running && <FindResult finder={finder} logic={logic} />}
    </section>
  );
}
