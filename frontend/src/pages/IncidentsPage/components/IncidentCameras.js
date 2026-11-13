import { useState } from 'react';

import { Badge } from '@/components/ui/badge';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { sorterIncidentCameraURL } from '@/helpers/api';
import { cn } from '@/lib/utils';

const NAMES = { c1: 'Bulk', c2: 'C-channel 2', c3: 'C-channel 3', c4: 'Classification' };

/** What a channel's summary says, as short phrases. */
function facts(summary) {
  if (!summary) return [];
  const zones = [
    summary.in_drop && 'drop',
    summary.in_precise && 'precise',
    summary.in_exit && 'exit',
  ].filter(Boolean);
  return [
    summary.valid ? `${summary.pieces} piece${summary.pieces === 1 ? '' : 's'}` : 'not seeing',
    zones.length > 0 && `in ${zones.join(', ')}`,
    summary.backed_up && 'backed up',
    summary.ignored > 0 && `leaving out ${summary.ignored} stuck`,
    summary.falls > 0 && `${summary.falls} pushed off this run`,
  ].filter(Boolean);
}

/**
 * The kept picture with what the camera saw drawn over it: piece boxes (with track
 * numbers) and spots left out. Drawn over the photo, so the marks don't follow the theme.
 */
function Shot({ incidentId, cam, className }) {
  const [size, setSize] = useState(null);
  const obs = cam.state?.observation;
  const focus = cam.state?.focus ?? [];
  const w = obs?.frame_w || size?.w || 640;
  const h = obs?.frame_h || size?.h || 480;
  return (
    <div
      className={cn('relative overflow-hidden bg-media', className)}
      style={{ aspectRatio: `${w} / ${h}` }}
    >
      <img
        src={sorterIncidentCameraURL(incidentId, cam.channel)}
        alt={`${cam.channel.toUpperCase()} when it happened`}
        loading="lazy"
        className="absolute inset-0 size-full object-contain"
        onLoad={(e) =>
          setSize({ w: e.currentTarget.naturalWidth, h: e.currentTarget.naturalHeight })
        }
      />
      {(obs || focus.length > 0) && (
        <svg
          viewBox={`0 0 ${w} ${h}`}
          preserveAspectRatio="none"
          className="pointer-events-none absolute inset-0 size-full"
        >
          {obs?.ignored?.map((b, i) => (
            <rect
              key={`i${i}`}
              x={b.x1}
              y={b.y1}
              width={b.x2 - b.x1}
              height={b.y2 - b.y1}
              fill="none"
              stroke="#a3a3a3"
              strokeWidth={Math.max(2, w / 320)}
              strokeDasharray="8 6"
            />
          ))}
          {obs?.pieces?.map((p, i) => (
            <g key={i}>
              <rect
                x={p.box.x1}
                y={p.box.y1}
                width={p.box.x2 - p.box.x1}
                height={p.box.y2 - p.box.y1}
                fill="none"
                stroke="#22c55e"
                strokeWidth={Math.max(2, w / 320)}
              />
              {p.track_id ? (
                <text
                  x={p.box.x1}
                  y={p.box.y1 - 4}
                  fill="#22c55e"
                  fontSize={Math.max(14, w / 40)}
                  fontWeight="600"
                >
                  #{p.track_id}
                </text>
              ) : null}
            </g>
          ))}
          {/* Where the problem is: a red box a little outside the piece, drawn last. */}
          {focus.map((b, i) => {
            const pad = Math.max(6, w / 100);
            return (
              <rect
                key={`f${i}`}
                x={b.x1 - pad}
                y={b.y1 - pad}
                width={b.x2 - b.x1 + 2 * pad}
                height={b.y2 - b.y1 + 2 * pad}
                rx={pad / 2}
                fill="none"
                stroke="#ef4444"
                strokeWidth={Math.max(3, w / 180)}
              />
            );
          })}
        </svg>
      )}
    </div>
  );
}

function State({ cam }) {
  const list = facts(cam.state?.summary);
  return (
    <div className="flex flex-col gap-1 text-xs">
      {cam.state?.doing && <p className="text-foreground">{cam.state.doing}</p>}
      {list.length > 0 && <p className="text-muted-foreground">{list.join(' · ')}</p>}
      {!cam.state?.doing && list.length === 0 && (
        <p className="text-muted-foreground">Nothing recorded.</p>
      )}
    </div>
  );
}

/**
 * Every channel as it was when an incident opened: C2–C4's pictures with what each camera
 * saw, and what each channel (C1 too) was doing. A picture opens large.
 */
export default function IncidentCameras({ incident }) {
  const [open, setOpen] = useState(null); // the camera shown large
  const cams = incident.cameras ?? [];
  if (cams.length === 0) return null;
  return (
    <>
      <ul className="grid grid-cols-2 gap-2 lg:grid-cols-4">
        {cams.map((cam) => (
          <li
            key={cam.channel}
            className={cn(
              'flex flex-col gap-1.5 rounded-lg border bg-background p-2',
              cam.state?.focus?.length ? 'border-destructive/60' : 'border-border',
            )}
          >
            <p className="flex items-center gap-2 text-xs font-medium">
              <span>
                {cam.channel.toUpperCase()}{' '}
                <span className="font-normal text-muted-foreground">· {NAMES[cam.channel]}</span>
              </span>
              {cam.state?.focus?.length > 0 && (
                <Badge variant="destructive" className="ml-auto px-1.5 py-0 text-[11px]">
                  Look here
                </Badge>
              )}
            </p>
            {cam.image ? (
              <button
                type="button"
                onClick={() => setOpen(cam)}
                className="block cursor-zoom-in overflow-hidden rounded-md"
                aria-label={`Show ${cam.channel.toUpperCase()} large`}
              >
                <Shot incidentId={incident.id} cam={cam} />
              </button>
            ) : (
              <div className="flex aspect-video items-center justify-center rounded-md bg-muted px-2 text-center text-xs text-muted-foreground">
                {cam.channel === 'c1' ? 'No camera on C1' : 'No picture'}
              </div>
            )}
            <State cam={cam} />
          </li>
        ))}
      </ul>

      <Dialog open={Boolean(open)} onOpenChange={(o) => !o && setOpen(null)}>
        <DialogContent className="max-h-[94dvh] overflow-y-auto sm:max-w-5xl">
          {open && (
            <>
              <DialogHeader>
                <DialogTitle>
                  Incident #{incident.id} · {open.channel.toUpperCase()} · {NAMES[open.channel]}
                </DialogTitle>
                <DialogDescription>
                  {incident.title} · {new Date(open.taken_at).toLocaleString()}
                  {open.state?.focus?.length > 0 && ' · the red box is where the problem is'}
                </DialogDescription>
              </DialogHeader>
              <Shot incidentId={incident.id} cam={open} className="w-full rounded-lg" />
              <div className="flex flex-wrap items-start gap-3">
                <Badge variant="idle">Machine {open.state?.machine_state || '—'}</Badge>
                <State cam={open} />
              </div>
            </>
          )}
        </DialogContent>
      </Dialog>
    </>
  );
}
