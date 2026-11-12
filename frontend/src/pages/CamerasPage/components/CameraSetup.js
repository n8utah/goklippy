import { RefreshCw } from 'lucide-react';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { cn } from '@/lib/utils';

import { CAMERA_CHANNELS, CHANNEL_LABELS, cameraSizeFor, deviceLabel } from '../CamerasPage.logic';
import CameraFinder from './CameraFinder';
import DeviceModes from './DeviceModes';
import DevicePreview from './DevicePreview';

const NOT_USED = '__none__';

function DeviceCard({ device, logic }) {
  const on = logic.channelOf(device);
  const shown = logic.previewing[device.id];
  return (
    <div
      className={cn(
        'flex flex-col gap-3 rounded-xl border bg-card p-4',
        on ? 'border-primary/60' : 'border-border',
      )}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="font-medium">{device.name}</p>
          {device.hint && (
            <p className="font-mono text-xs text-muted-foreground">USB {device.hint}</p>
          )}
        </div>
        <label className="flex flex-col gap-1 text-sm">
          <span className="text-xs text-muted-foreground">Watches</span>
          <Select
            value={on || NOT_USED}
            onValueChange={(v) => logic.assign(device, v === NOT_USED ? '' : v)}
          >
            <SelectTrigger size="sm" className="w-44">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={NOT_USED}>Not used</SelectItem>
              {CAMERA_CHANNELS.map((c) => (
                <SelectItem key={c} value={c}>
                  {CHANNEL_LABELS[c]}
                  {logic.cams.devices?.[c] &&
                  logic.cams.devices[c] !== device.id &&
                  logic.cams.devices[c] !== device.name
                    ? ' (replaces its camera)'
                    : ''}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </label>
      </div>
      {shown ? (
        <DevicePreview deviceId={device.id} />
      ) : (
        <div className="flex aspect-[4/3] items-center justify-center rounded-lg bg-muted text-sm text-muted-foreground">
          Picture off
        </div>
      )}
      <div className="flex flex-wrap gap-2">
        <Button
          variant="outline"
          size="sm"
          disabled={logic.simulated}
          onClick={() => logic.togglePreview(device.id)}
        >
          {shown ? 'Hide picture' : 'Show picture'}
        </Button>
        {on && (
          <Button variant="outline" size="sm" onClick={() => logic.setView(on)}>
            Draw {CHANNEL_LABELS[on]} zones
          </Button>
        )}
      </div>
      <DeviceModes
        deviceId={device.id}
        width={cameraSizeFor(logic.cams, on).width}
        height={cameraSizeFor(logic.cams, on).height}
        own={cameraSizeFor(logic.cams, on).own}
        disabled={logic.simulated}
      />
    </div>
  );
}

/** Cameras › Setup: every camera found, a live picture of each, and which channel it watches. */
export default function CameraSetup({ logic }) {
  const { devices } = logic;
  return (
    <div className="flex flex-col gap-4">
      <section className="flex flex-col gap-3 rounded-xl border border-border bg-card p-5">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <p className="max-w-[75ch] text-sm text-muted-foreground">
            Show a camera’s picture to see which one it is (wave a hand in front of it), then choose
            the channel it watches. Leave the laptop’s own camera on Not used. Cameras are known by
            their USB port, so plug each back into the same port.
          </p>
          <Button
            variant="outline"
            disabled={devices.loading}
            onClick={logic.findDevices}
            className="shrink-0"
          >
            <RefreshCw className={devices.loading ? 'animate-spin' : ''} />
            {devices.loading ? 'Looking…' : 'Look for cameras again'}
          </Button>
        </div>
        {devices.ffmpeg && (
          <p className="font-mono text-xs break-all text-muted-foreground">
            ffmpeg {devices.version || '(version unknown)'} · {devices.ffmpeg}
          </p>
        )}
        {logic.missing.map((m) => (
          <p key={m.channel} className="rounded-md bg-warning-bg px-3 py-2 text-sm text-warning">
            {CHANNEL_LABELS[m.channel]}’s camera, {m.label}, isn’t connected. Plug it back into the
            USB port it was in, or choose another camera for {CHANNEL_LABELS[m.channel]} below.
          </p>
        ))}
        {devices.warning && (
          <p className="rounded-md bg-warning-bg px-3 py-2 text-sm text-warning">
            {devices.warning}
          </p>
        )}
        {devices.error && <p className="text-sm text-destructive">{devices.error}</p>}
      </section>

      {!devices.loading && devices.list.length === 0 && !devices.error && (
        <p className="text-sm text-muted-foreground">
          No cameras found. Plug one in and look again.
        </p>
      )}
      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        {devices.list.map((d) => (
          <DeviceCard key={d.id} device={d} logic={logic} />
        ))}
      </div>

      {devices.list.length > 0 && <CameraFinder logic={logic} />}

      <section className="rounded-xl border border-border bg-card">
        <h2 className="border-b border-border px-5 py-3 font-display text-lg font-semibold">
          Channels
        </h2>
        <ul className="divide-y divide-border">
          {CAMERA_CHANNELS.map((c) => {
            const id = logic.cams.devices?.[c];
            const dev = devices.list.find((d) => d.id === id || d.name === id);
            const st = logic.statusOf(c);
            const running = st?.camera?.running;
            return (
              <li
                key={c}
                className="flex flex-col gap-1 px-5 py-3 sm:flex-row sm:items-center sm:gap-4"
              >
                <span className="w-40 shrink-0 font-medium">{CHANNEL_LABELS[c]}</span>
                <span className="min-w-0 flex-1 truncate text-sm">
                  {dev ? deviceLabel(dev) : id ? logic.cams.deviceNames?.[c] || id : 'No camera'}
                </span>
                {id && (
                  <span className="flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
                    <Badge
                      variant={running ? 'running' : st?.camera?.error ? 'destructive' : 'idle'}
                    >
                      {running
                        ? `${st.camera.fps.toFixed(1)} fps`
                        : st?.camera?.error
                          ? 'Not running'
                          : 'Starting'}
                    </Badge>
                    {running && !st.zones_set && 'zones not drawn'}
                    {running && st.zones_set && !st.has_background && 'no empty-channel picture'}
                    {st?.camera?.error && <span className="truncate">{st.camera.error}</span>}
                  </span>
                )}
                <Button variant="ghost" size="sm" onClick={() => logic.setView(c)}>
                  Open
                </Button>
              </li>
            );
          })}
        </ul>
      </section>
    </div>
  );
}
