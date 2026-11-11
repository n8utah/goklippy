import CameraFinder from './CameraFinder';
import DeviceModes from './DeviceModes';
import DevicePreview from './DevicePreview';
import { CAMERA_CHANNELS, CHANNEL_LABELS, deviceLabel } from '../CamerasPage.logic';

import styles from '../CamerasPage.module.css';
import appStyles from '../../../styles/app.module.css';

function DeviceCard({ device, logic }) {
  const on = logic.channelOf(device);
  const shown = logic.previewing[device.id];
  return (
    <div className={on ? styles.deviceCardOn : styles.deviceCard}>
      <div className={styles.deviceHeader}>
        <div className={styles.deviceText}>
          <strong>{device.name}</strong>
          {device.hint && <span className={styles.hint}>USB {device.hint}</span>}
        </div>
        <label className={styles.field}>
          <span className={styles.fieldLabel}>Watches</span>
          <select value={on} onChange={(e) => logic.assign(device, e.target.value)}>
            <option value="">Not used</option>
            {CAMERA_CHANNELS.map((c) => (
              <option key={c} value={c}>
                {CHANNEL_LABELS[c]}
                {logic.cams.devices?.[c] &&
                logic.cams.devices[c] !== device.id &&
                logic.cams.devices[c] !== device.name
                  ? ' (replaces its camera)'
                  : ''}
              </option>
            ))}
          </select>
        </label>
      </div>
      {shown ? (
        <DevicePreview deviceId={device.id} />
      ) : (
        <div className={styles.previewOff}>Picture off</div>
      )}
      <div className={styles.buttonRow}>
        <button
          type="button"
          className={appStyles.buttonSecondary}
          disabled={logic.simulated}
          onClick={() => logic.togglePreview(device.id)}
        >
          {shown ? 'Hide picture' : 'Show picture'}
        </button>
        {on && (
          <button
            type="button"
            className={appStyles.buttonSecondary}
            onClick={() => logic.setView(on)}
          >
            Draw {CHANNEL_LABELS[on]} zones
          </button>
        )}
      </div>
      <DeviceModes
        deviceId={device.id}
        width={logic.cams.width}
        height={logic.cams.height}
        disabled={logic.simulated}
      />
    </div>
  );
}

/** Every camera found, a live picture of each, and which channel it watches. */
const CameraSetup = ({ logic }) => {
  const { devices } = logic;
  return (
    <div className={styles.setup}>
      <p className={styles.hint}>
        Show a camera&apos;s picture to see which one it is (wave a hand in front of it), then
        choose the channel it watches. Leave the laptop&apos;s own camera on Not used. Cameras are
        remembered by their USB port: plug each one back into the same port.
      </p>

      {logic.missing.map((m) => (
        <p key={m.channel} className={styles.alert}>
          {CHANNEL_LABELS[m.channel]}&apos;s camera, {m.label}, isn&apos;t connected. Plug it back
          into the USB port it was in, or choose another camera for {CHANNEL_LABELS[m.channel]}{' '}
          below.
        </p>
      ))}
      {devices.warning && <p className={styles.alert}>{devices.warning}</p>}
      {devices.error && <p className={styles.alert}>{devices.error}</p>}

      <div className={styles.buttonRow}>
        <button
          type="button"
          className={appStyles.buttonSecondary}
          disabled={devices.loading}
          onClick={logic.findDevices}
        >
          {devices.loading ? 'Looking…' : 'Look for cameras again'}
        </button>
        <span className={styles.hint}>
          {devices.ffmpeg && `ffmpeg ${devices.version || '(version unknown)'} · ${devices.ffmpeg}`}
        </span>
      </div>

      {!devices.loading && devices.list.length === 0 && !devices.error && (
        <p className={styles.hint}>No cameras found. Plug one in and look again.</p>
      )}
      <div className={styles.deviceGrid}>
        {devices.list.map((d) => (
          <DeviceCard key={d.id} device={d} logic={logic} />
        ))}
      </div>

      {devices.list.length > 0 && <CameraFinder logic={logic} />}

      <div className={styles.panel}>
        <h3 className={styles.panelTitle}>Channels</h3>
        {CAMERA_CHANNELS.map((c) => {
          const id = logic.cams.devices?.[c];
          const dev = devices.list.find((d) => d.id === id || d.name === id);
          const st = logic.statusOf(c);
          return (
            <div key={c} className={styles.channelRow}>
              <strong>{CHANNEL_LABELS[c]}</strong>
              <span>
                {dev ? deviceLabel(dev) : id ? logic.cams.deviceNames?.[c] || id : 'No camera'}
              </span>
              <span className={styles.hint}>
                {!id
                  ? ''
                  : st?.camera?.running
                    ? `running, ${st.camera.fps.toFixed(1)} fps${st.zones_set ? '' : ' · zones not drawn'}${st.has_background ? '' : ' · no empty-channel picture'}`
                    : st?.camera?.error
                      ? `not running: ${st.camera.error}`
                      : 'starting…'}
              </span>
            </div>
          );
        })}
      </div>
    </div>
  );
};

export default CameraSetup;
