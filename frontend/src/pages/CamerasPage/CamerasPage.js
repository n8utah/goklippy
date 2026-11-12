import { Check, Circle, CircleHelp } from 'lucide-react';
import { Navigate, useNavigate, useParams } from 'react-router';

import CameraView from '@/components/CameraView/CameraView';
import NumberField from '@/components/NumberField/NumberField';
import SettingField from '@/components/SettingField/SettingField';
import { Button } from '@/components/ui/button';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';

import {
  CAMERA_CHANNELS,
  CAMERA_SIZES,
  HIGH_RES_SIZES,
  TRACK_WIDTH,
  cameraSizeFor,
  CHANNEL_LABELS,
  TEST_DEVICE,
  deviceLabel,
  useCamerasPageLogic,
} from './CamerasPage.logic';
import CameraSetup from './components/CameraSetup';

const PARTS = ['setup', ...CAMERA_CHANNELS];
const NONE = '__none__';
const MISSING = '__missing__';

function Panel({ title, children }) {
  return (
    <section className="flex flex-col gap-3 rounded-xl border border-border bg-card p-5">
      <h2 className="font-display text-lg font-semibold">{title}</h2>
      {children}
    </section>
  );
}

function ZoneEditor({ zones, onField }) {
  const field = (label, path, value, min) => (
    <NumberField
      key={path.join('.')}
      id={`zone-${path.join('-')}`}
      label={label}
      value={value}
      min={min}
      inputClassName="w-24"
      onChange={(v) => onField(path, v)}
    />
  );
  return (
    <div className="flex flex-col gap-4">
      <fieldset className="flex flex-col gap-2">
        <legend className="mb-1 text-sm font-medium text-muted-foreground">Ring, in pixels</legend>
        <div className="flex flex-wrap gap-3">
          {field('Center x', ['ring', 'center', 'x'], zones.ring.center.x)}
          {field('Center y', ['ring', 'center', 'y'], zones.ring.center.y)}
          {field('Inner radius', ['ring', 'inner_radius'], zones.ring.inner_radius, 0)}
          {field('Outer radius', ['ring', 'outer_radius'], zones.ring.outer_radius, 1)}
          {field('Exit radius', ['ring', 'exit_radius'], zones.ring.exit_radius, 1)}
        </div>
      </fieldset>
      <fieldset className="flex flex-col gap-2">
        <legend className="mb-1 text-sm font-medium text-muted-foreground">
          Zones, in degrees (0° right, clockwise)
        </legend>
        <div className="flex flex-wrap gap-3">
          {field('Drop from', ['drop', 'start'], zones.drop.start)}
          {field('Drop to', ['drop', 'end'], zones.drop.end)}
          {field('Exit from', ['exit', 'start'], zones.exit.start)}
          {field('Exit to', ['exit', 'end'], zones.exit.end)}
          {field('Precise width', ['precise_width'], zones.precise_width, 0)}
        </div>
      </fieldset>
    </div>
  );
}

const SAME = '__same__';

/** This channel's camera size: every camera's, or its own (up to 4K). */
function ChannelSize({ logic }) {
  const { cams, channel } = logic;
  const size = cameraSizeFor(cams, channel);
  const key = (w, h) => `${w}x${h}`;
  const listed = [...HIGH_RES_SIZES, ...CAMERA_SIZES];
  return (
    <label className="flex flex-col gap-1.5 text-sm">
      <span className="font-medium">Picture size</span>
      <Select
        value={size.own ? key(size.width, size.height) : SAME}
        onValueChange={(v) => {
          if (v === SAME) return logic.setChannelSize(null);
          const [width, height] = v.split('x').map(Number);
          logic.setChannelSize({ width, height });
        }}
      >
        <SelectTrigger className="w-64">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={SAME}>
            Every camera’s ({cams.width}×{cams.height})
          </SelectItem>
          {listed.map(([w, h]) => (
            <SelectItem key={key(w, h)} value={key(w, h)}>
              {w}×{h}
              {w > TRACK_WIDTH ? ' · full size for photos' : ''}
            </SelectItem>
          ))}
          {size.own && !listed.some(([w, h]) => w === size.width && h === size.height) && (
            <SelectItem value={key(size.width, size.height)}>
              {size.width}×{size.height}
            </SelectItem>
          )}
        </SelectContent>
      </Select>
      <span className="text-xs text-muted-foreground">
        {size.width > TRACK_WIDTH
          ? `Captured at ${size.width}×${size.height} in the camera’s MJPEG. Pieces are tracked on a ${TRACK_WIDTH}-wide copy; the photos sent to Brickognize are cut from the full-size picture. `
          : ''}
        The camera must offer the size (Check its sizes on Setup). Changing it needs the ring
        measured and the empty channel captured again.
      </span>
    </label>
  );
}

/**
 * Machine › Cameras: Setup (which camera watches which channel) and a tab per channel
 * (/machine/cameras/<c2|c3|c4>) to set up its picture: ring and zones, and the empty
 * channel that pieces are found against.
 */
const CamerasPage = () => {
  const { part = 'setup' } = useParams();
  const navigate = useNavigate();
  const logic = useCamerasPageLogic(part, (v) => navigate(`/machine/cameras/${v}`));
  const { cams, channel, zones, chStatus, devices } = logic;

  if (!PARTS.includes(part)) return <Navigate to="/machine/cameras" replace />;

  const device = cams.devices?.[channel] ?? '';
  const known =
    devices.list.some((d) => d.id === device) || device === TEST_DEVICE || device === '';
  const hasPicture = Boolean(device) || logic.simulated;

  return (
    <div className="ds flex flex-col gap-4">
      {logic.simulated && (
        <p className="rounded-xl border border-border bg-card px-4 py-3 text-sm text-muted-foreground">
          The machine is simulated, so channel pictures are drawn and cameras can’t be previewed.
        </p>
      )}
      {logic.error && <p className="text-sm text-destructive">{logic.error}</p>}
      {logic.notice && (
        <p className="rounded-xl border border-running/40 bg-running-bg px-4 py-3 text-sm text-running-text">
          {logic.notice}
        </p>
      )}

      {part === 'setup' ? (
        <CameraSetup logic={logic} />
      ) : (
        <>
          <section className="rounded-xl border border-border bg-card px-5 py-4">
            <h2 className="mb-2 font-display text-lg font-semibold">
              Setting up {CHANNEL_LABELS[channel]}
            </h2>
            <ol className="grid gap-1.5 sm:grid-cols-2 xl:grid-cols-3">
              {logic.setupSteps.map((s) => (
                <li key={s.label} className="flex items-start gap-2 text-sm">
                  {s.done ? (
                    <Check className="mt-0.5 size-4 shrink-0 text-running" aria-label="Done" />
                  ) : s.done === null ? (
                    <CircleHelp className="mt-0.5 size-4 shrink-0 text-idle" aria-label="Unknown" />
                  ) : (
                    <Circle className="mt-0.5 size-4 shrink-0 text-idle" aria-label="To do" />
                  )}
                  <span>
                    {s.label}
                    {s.note && <span className="text-muted-foreground"> ({s.note})</span>}
                  </span>
                </li>
              ))}
            </ol>
          </section>

          <div className="grid items-start gap-4 xl:grid-cols-[minmax(0,1.3fr)_minmax(0,1fr)]">
            <div className="flex flex-col gap-2 xl:sticky xl:top-20">
              {hasPicture ? (
                <div className="overflow-hidden rounded-xl bg-media">
                  <CameraView
                    key={`${channel}-${device}`}
                    channel={channel}
                    zonesOverride={zones}
                    onZonesChange={zones ? logic.setZones : undefined}
                  />
                </div>
              ) : (
                <div className="flex aspect-[4/3] items-center justify-center rounded-xl bg-muted text-sm text-muted-foreground">
                  Pick a camera for this channel.
                </div>
              )}
              <p className="text-xs text-muted-foreground">
                {zones
                  ? 'Drag inside the ring to move it, or a circle’s edge to size it; the wheel sizes the nearest circle (Shift: all of them). Drag a colored section to slide it round, or its dots to stretch it. Double-click puts the center there. '
                  : ''}
                Blue: drop zone · magenta: precise · orange: exit · green boxes: pieces found.
              </p>
              {chStatus && (
                <p className="text-sm">
                  {chStatus.camera?.running
                    ? `Running · ${chStatus.frame_w}×${chStatus.frame_h} · ${chStatus.camera.fps.toFixed(1)} fps`
                    : chStatus.camera?.error
                      ? `Not running: ${chStatus.camera.error}`
                      : device
                        ? 'Starting…'
                        : ''}
                  {chStatus.message && ` · ${chStatus.message}`}
                  {chStatus.has_background &&
                    (chStatus.views > 0
                      ? ` · empty all the way round (${chStatus.views} pictures, matching #${chStatus.matched_view + 1})`
                      : ' · empty-channel picture captured')}
                </p>
              )}
            </div>

            <div className="flex flex-col gap-4">
              <Panel title="Camera">
                <Select
                  value={!known ? MISSING : device || NONE}
                  onValueChange={(v) => v !== MISSING && logic.setDevice(v === NONE ? '' : v)}
                >
                  <SelectTrigger className="w-full" aria-label="Camera for this channel">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value={NONE}>None</SelectItem>
                    {devices.list.map((d) => (
                      <SelectItem key={d.id} value={d.id}>
                        {deviceLabel(d)}
                      </SelectItem>
                    ))}
                    <SelectItem value={TEST_DEVICE}>ffmpeg test picture</SelectItem>
                    {!known && (
                      <SelectItem value={MISSING}>
                        {cams.deviceNames?.[channel] || device} (not found)
                      </SelectItem>
                    )}
                  </SelectContent>
                </Select>
                <div className="flex flex-wrap gap-2">
                  <Button variant="outline" disabled={devices.loading} onClick={logic.findDevices}>
                    {devices.loading ? 'Looking…' : 'Look for cameras'}
                  </Button>
                  <Button disabled={!hasPicture} onClick={logic.captureBackground}>
                    Capture the empty channel
                  </Button>
                </div>
                {devices.error && <p className="text-sm text-muted-foreground">{devices.error}</p>}
                <ChannelSize logic={logic} />
              </Panel>

              <Panel title="Empty all the way round">
                <p className="text-sm text-muted-foreground">
                  For a rotor with a pattern (the cone’s facets): turns the channel across one
                  section ({logic.roundPeriod.toFixed(0)}°, after which it looks the same again),
                  capturing it empty every few degrees, so the pattern turning isn’t taken for
                  pieces. Clear the channel first. It replaces the single picture, and the single
                  picture replaces it.
                </p>
                <div className="flex flex-wrap items-end gap-3">
                  <NumberField
                    id="round-step"
                    label="Every"
                    value={logic.roundStep}
                    min={0.5}
                    max={30}
                    step={0.5}
                    unit="°"
                    disabled={Boolean(logic.roundJob)}
                    onChange={logic.setRoundStep}
                  />
                  <Button
                    disabled={!device || logic.simulated || Boolean(logic.roundJob)}
                    onClick={logic.captureRound}
                  >
                    Capture across a section
                  </Button>
                </div>
                <p className="text-sm text-muted-foreground">
                  {logic.roundJob
                    ? `${logic.roundJob.detail} (${logic.roundJob.step} of ${logic.roundJob.total})`
                    : `${Math.max(1, Math.round(logic.roundPeriod / logic.roundStep))} pictures, about ${Math.ceil((logic.roundPeriod / logic.roundStep) * 0.8)} s.`}
                </p>
              </Panel>

              <Panel title="Ring and zones">
                <div className="flex flex-wrap gap-2">
                  <Button disabled={logic.measuring || !hasPicture} onClick={logic.measureRing}>
                    {logic.measuring ? 'Measuring…' : 'Measure the ring from the picture'}
                  </Button>
                  {!zones && (
                    <Button variant="outline" onClick={logic.startZones}>
                      Draw by hand
                    </Button>
                  )}
                </div>
                <p className="text-sm text-muted-foreground">
                  Finds the bowl’s rim and the hole in its middle and sets the ring between them. It
                  keeps the drop and exit angles; set those by hand.
                </p>
                {zones && <ZoneEditor zones={zones} onField={logic.setZoneField} />}
              </Panel>

              <Panel title="Every camera">
                <label className="flex flex-col gap-1.5 text-sm">
                  <span className="font-medium">Picture size</span>
                  <Select
                    value={`${cams.width}x${cams.height}`}
                    onValueChange={(v) => {
                      const [width, height] = v.split('x').map(Number);
                      logic.setCameras({ width, height });
                    }}
                  >
                    <SelectTrigger className="w-40">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {CAMERA_SIZES.map(([w, h]) => (
                        <SelectItem key={`${w}x${h}`} value={`${w}x${h}`}>
                          {w}×{h}
                        </SelectItem>
                      ))}
                      {!CAMERA_SIZES.some(([w, h]) => w === cams.width && h === cams.height) && (
                        <SelectItem value={`${cams.width}x${cams.height}`}>
                          {cams.width}×{cams.height}
                        </SelectItem>
                      )}
                    </SelectContent>
                  </Select>
                </label>
                <div className="divide-y divide-border border-t border-border">
                  <SettingField settingKey="cameras.fps" />
                  <SettingField settingKey="cameras.threshold" />
                  <SettingField settingKey="cameras.minArea" />
                </div>
              </Panel>
            </div>
          </div>
        </>
      )}
    </div>
  );
};

export default CamerasPage;
