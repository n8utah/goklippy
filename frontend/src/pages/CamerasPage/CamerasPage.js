import {
  CAMERA_CHANNELS,
  CAMERA_SIZES,
  CHANNEL_LABELS,
  TEST_DEVICE,
  deviceLabel,
  useCamerasPageLogic,
} from './CamerasPage.logic';
import CameraSetup from './components/CameraSetup';
import CameraView from '../../components/CameraView/CameraView';

import styles from './CamerasPage.module.css';
import appStyles from '../../styles/app.module.css';

function Num({ label, value, onChange, step = 1, min }) {
  return (
    <label className={styles.field}>
      <span className={styles.fieldLabel}>{label}</span>
      <input
        type="number"
        step={step}
        min={min}
        value={value}
        onChange={(e) => {
          if (e.target.value !== '') onChange(Number(e.target.value));
        }}
      />
    </label>
  );
}

function ZoneEditor({ zones, onField }) {
  return (
    <div className={styles.zoneGrid}>
      <fieldset className={styles.group}>
        <legend>Ring (pixels)</legend>
        <Num
          label="Center x"
          value={zones.ring.center.x}
          onChange={(v) => onField(['ring', 'center', 'x'], v)}
        />
        <Num
          label="Center y"
          value={zones.ring.center.y}
          onChange={(v) => onField(['ring', 'center', 'y'], v)}
        />
        <Num
          label="Inner radius"
          value={zones.ring.inner_radius}
          min={0}
          onChange={(v) => onField(['ring', 'inner_radius'], v)}
        />
        <Num
          label="Outer radius"
          value={zones.ring.outer_radius}
          min={1}
          onChange={(v) => onField(['ring', 'outer_radius'], v)}
        />
        <Num
          label="Exit radius"
          value={zones.ring.exit_radius}
          min={1}
          onChange={(v) => onField(['ring', 'exit_radius'], v)}
        />
      </fieldset>
      <fieldset className={styles.group}>
        <legend>Zones (degrees, 0° right, clockwise)</legend>
        <Num
          label="Drop from"
          value={zones.drop.start}
          onChange={(v) => onField(['drop', 'start'], v)}
        />
        <Num label="Drop to" value={zones.drop.end} onChange={(v) => onField(['drop', 'end'], v)} />
        <Num
          label="Exit from"
          value={zones.exit.start}
          onChange={(v) => onField(['exit', 'start'], v)}
        />
        <Num label="Exit to" value={zones.exit.end} onChange={(v) => onField(['exit', 'end'], v)} />
        <Num
          label="Precise width"
          value={zones.precise_width}
          min={0}
          onChange={(v) => onField(['precise_width'], v)}
        />
      </fieldset>
    </div>
  );
}

const CamerasPage = () => {
  const logic = useCamerasPageLogic();
  const { cams, channel, zones, chStatus, devices } = logic;
  const device = cams.devices?.[channel] ?? '';
  const known =
    devices.list.some((d) => d.id === device) || device === TEST_DEVICE || device === '';

  return (
    <section className={appStyles.card}>
      <h2 className={styles.title}>Cameras</h2>
      <p className={appStyles.muted}>
        One camera over each of C2, C3 and C4. Choose which camera watches which channel under
        Setup, then on each channel&apos;s tab draw its ring and zones over its picture and capture
        the channel empty: pieces are found as what differs from that picture.
        {logic.simulated &&
          ' The machine is simulated, so the channel pictures are drawn and cameras can’t be previewed.'}
      </p>

      <div className={styles.tabs}>
        <button
          type="button"
          className={logic.view === 'setup' ? styles.tabActive : styles.tab}
          onClick={() => logic.setView('setup')}
        >
          Setup
        </button>
        {CAMERA_CHANNELS.map((ch) => (
          <button
            key={ch}
            type="button"
            className={logic.view === ch ? styles.tabActive : styles.tab}
            onClick={() => logic.setView(ch)}
          >
            {CHANNEL_LABELS[ch]}
          </button>
        ))}
      </div>

      {logic.error && <p className={styles.alert}>{logic.error}</p>}
      {logic.notice && <p className={styles.notice}>{logic.notice}</p>}

      {logic.view === 'setup' ? (
        <CameraSetup logic={logic} />
      ) : (
        <div className={styles.layout}>
          <div className={styles.viewCol}>
            {device || logic.simulated ? (
              <CameraView
                key={`${channel}-${device}`}
                channel={channel}
                zonesOverride={zones}
                onPick={zones ? logic.pickCenter : undefined}
              />
            ) : (
              <div className={styles.placeholder}>Pick a camera for this channel.</div>
            )}
            <p className={styles.hint}>
              {zones ? 'Click the picture to put the ring’s center there. ' : ''}
              Blue: drop zone · magenta: precise · orange: exit · green boxes: pieces found.
            </p>
            {chStatus && (
              <p className={styles.status}>
                {chStatus.camera?.running
                  ? `Running · ${chStatus.frame_w}×${chStatus.frame_h} · ${chStatus.camera.fps.toFixed(1)} fps`
                  : chStatus.camera?.error
                    ? `Not running: ${chStatus.camera.error}`
                    : device
                      ? 'Starting…'
                      : ''}
                {chStatus.message && ` · ${chStatus.message}`}
                {chStatus.has_background && ' · empty-channel picture captured'}
              </p>
            )}
          </div>

          <div className={styles.side}>
            <div className={styles.panel}>
              <h3 className={styles.panelTitle}>Camera</h3>
              <label className={styles.field}>
                <span className={styles.fieldLabel}>Device</span>
                <select
                  value={known ? device : '__custom'}
                  onChange={(e) =>
                    logic.setDevice(e.target.value === '__custom' ? device : e.target.value)
                  }
                >
                  <option value="">None</option>
                  {devices.list.map((d) => (
                    <option key={d.id} value={d.id}>
                      {deviceLabel(d)}
                    </option>
                  ))}
                  <option value={TEST_DEVICE}>ffmpeg test picture</option>
                  {!known && (
                    <option value="__custom">
                      {cams.deviceNames?.[channel] || device} (not found)
                    </option>
                  )}
                </select>
              </label>
              <div className={styles.buttonRow}>
                <button
                  type="button"
                  className={appStyles.buttonSecondary}
                  disabled={devices.loading}
                  onClick={logic.findDevices}
                >
                  {devices.loading ? 'Looking…' : 'Find cameras'}
                </button>
                <button
                  type="button"
                  className={appStyles.button}
                  disabled={!device && !logic.simulated}
                  onClick={logic.captureBackground}
                >
                  Capture empty channel
                </button>
              </div>
              {devices.error && <p className={styles.hint}>{devices.error}</p>}
            </div>

            <div className={styles.panel}>
              <h3 className={styles.panelTitle}>Zones</h3>
              {zones ? (
                <ZoneEditor zones={zones} onField={logic.setZoneField} />
              ) : (
                <button type="button" className={appStyles.button} onClick={logic.startZones}>
                  Start drawing zones
                </button>
              )}
            </div>

            <div className={styles.panel}>
              <h3 className={styles.panelTitle}>All cameras</h3>
              <div className={styles.inline}>
                <label className={styles.field}>
                  <span className={styles.fieldLabel}>Size</span>
                  <select
                    value={`${cams.width}x${cams.height}`}
                    onChange={(e) => {
                      const [width, height] = e.target.value.split('x').map(Number);
                      logic.setCameras({ width, height });
                    }}
                  >
                    {CAMERA_SIZES.map(([w, h]) => (
                      <option key={`${w}x${h}`} value={`${w}x${h}`}>
                        {w}×{h}
                      </option>
                    ))}
                    {!CAMERA_SIZES.some(([w, h]) => w === cams.width && h === cams.height) && (
                      <option value={`${cams.width}x${cams.height}`}>
                        {cams.width}×{cams.height}
                      </option>
                    )}
                  </select>
                </label>
                <Num
                  label="FPS"
                  value={cams.fps}
                  min={1}
                  onChange={(v) => logic.setCameras({ fps: v })}
                />
              </div>
              <div className={styles.inline}>
                <Num
                  label="Sensitivity (0 = 90)"
                  value={cams.threshold}
                  min={0}
                  onChange={(v) => logic.setCameras({ threshold: v })}
                />
                <Num
                  label="Smallest piece px (0 = scaled)"
                  value={cams.minArea}
                  min={0}
                  onChange={(v) => logic.setCameras({ minArea: v })}
                />
              </div>
              <label className={styles.field}>
                <span className={styles.fieldLabel}>ffmpeg (empty: the one on PATH)</span>
                <input
                  value={cams.ffmpeg}
                  onChange={(e) => logic.setCameras({ ffmpeg: e.target.value })}
                  placeholder="C:\ffmpeg\bin\ffmpeg.exe"
                />
              </label>
            </div>
          </div>
        </div>
      )}
    </section>
  );
};

export default CamerasPage;
