import { useChutePageLogic } from './ChutePage.logic';
import CalibrationPanel from './components/CalibrationPanel';
import TowerMap from './components/TowerMap';

import styles from './ChutePage.module.css';
import appStyles from '../../styles/app.module.css';

const STATE_LABELS = {
  standby: 'Standby',
  homing: 'Homing',
  ready: 'Ready',
  running: 'Sorting',
  paused: 'Paused',
  error: 'Error',
};

function NumberSetting({ label, value, min, step, unit, onChange }) {
  return (
    <label className={styles.field}>
      <span className={styles.fieldLabel}>
        {label}
        {unit && <span className={styles.unit}> ({unit})</span>}
      </span>
      <input
        type="number"
        min={min}
        step={step}
        value={value}
        onChange={(e) => {
          const next = Number(e.target.value);
          if (e.target.value !== '' && next >= min) onChange(next);
        }}
      />
    </label>
  );
}

function JobLine({ job, lastJob, onCancel }) {
  if (job) {
    return (
      <div className={styles.job}>
        <span>
          <strong>{job.name}</strong>
          {job.total > 0 && ` — step ${job.step} of ${job.total}`}
          {job.detail && `: ${job.detail}`}
        </span>
        <button type="button" className={appStyles.buttonSecondary} onClick={onCancel}>
          Cancel
        </button>
      </div>
    );
  }
  if (lastJob?.error) {
    return (
      <p className={styles.alert}>
        {lastJob.name} failed: {lastJob.error}
      </p>
    );
  }
  return null;
}

const ChutePage = () => {
  const logic = useChutePageLogic();
  const { state, status, distributor, busy, sorting, layer, layers, geometry } = logic;
  const homed = Boolean(status?.homed);
  const canMove = Boolean(state?.hardware?.connected) && !busy && !sorting;

  return (
    <section className={appStyles.card}>
      <div className={styles.headerRow}>
        <div>
          <h2 className={styles.title}>Chute</h2>
          <p className={appStyles.muted}>
            {logic.liveConnected ? (
              <>
                Sorter <strong>{STATE_LABELS[state?.state] ?? '…'}</strong>
                {state?.hardware?.connected ? ` on ${state.hardware.port}` : ` — ${state?.hardware?.message ?? 'no board'}`}
              </>
            ) : (
              'Connecting to the backend…'
            )}
          </p>
        </div>
        <div className={styles.buttonRow}>
          <button type="button" className={appStyles.button} disabled={busy || state?.state === 'running'} onClick={logic.home}>
            {homed ? 'Home again' : 'Home'}
          </button>
          <button type="button" className={appStyles.buttonSecondary} disabled={!canMove} onClick={logic.zero} title="Use where the chute points now as 0°, without the home switch">
            Set 0° here
          </button>
        </div>
      </div>

      {state?.error && <p className={styles.alert}>{state.error}</p>}
      <JobLine job={state?.job} lastJob={state?.last_job} onCancel={logic.cancel} />
      {logic.error && <p className={styles.alert}>{logic.error}</p>}

      <div className={styles.stats}>
        <div className={styles.stat}>
          <span className={styles.statLabel}>Chute</span>
          <span className={styles.statValue}>{homed && status.angle != null ? `${status.angle.toFixed(1)}°` : 'Not homed'}</span>
          <span className={styles.statNote}>clockwise from home</span>
        </div>
        <div className={styles.stat}>
          <span className={styles.statLabel}>Home switch</span>
          <span className={styles.statValue}>{status?.endstop ? (status.endstop === 'triggered' ? 'Triggered' : 'Open') : '—'}</span>
          <span className={styles.statNote}>{status?.message || distributor.endstop}</span>
        </div>
        <div className={styles.stat}>
          <span className={styles.statLabel}>Geometry</span>
          <span className={styles.statValue}>{distributor.sectionWidth.toFixed(2)}°</span>
          <span className={styles.statNote}>
            per section, first at {distributor.firstSectionOffset.toFixed(2)}°
          </span>
        </div>
      </div>

      <div className={styles.layout}>
        <div className={styles.panel}>
          <div className={styles.layerTabs}>
            {layers.map((_, i) => (
              <button
                key={i}
                type="button"
                className={i === logic.layerIndex ? styles.tabActive : styles.tab}
                onClick={() => logic.setLayerIndex(i)}
              >
                Layer {i + 1}
              </button>
            ))}
          </div>
          <TowerMap
            bins={layer}
            geometry={geometry}
            angle={homed ? status.angle : null}
            disabled={!canMove || !homed}
            onBin={logic.sendToBin}
          />
          <p className={styles.hint}>
            {homed ? 'Click a bin to select its layer and aim the chute at it.' : 'Home the chute to aim at bins.'}
          </p>
          <div className={styles.buttonRow}>
            <button type="button" className={appStyles.button} disabled={!canMove || !homed} onClick={logic.testLayer}>
              Test every bin on layer {logic.layerIndex + 1}
            </button>
            <NumberSetting label="Pause at each" value={logic.testDwellMs} min={0} step={250} unit="ms" onChange={logic.setTestDwellMs} />
          </div>
        </div>

        <CalibrationPanel logic={logic} canMove={canMove && homed} />
      </div>

      <div className={styles.panel}>
        <h3 className={styles.panelTitle}>Chute settings</h3>
        <div className={styles.fields}>
          <label className={styles.field}>
            <span className={styles.fieldLabel}>Home switch</span>
            <select value={distributor.endstop} onChange={(e) => logic.setDistributor({ endstop: e.target.value })}>
              {!logic.endstopObjects.some((obj) => obj.section === distributor.endstop) && (
                <option value={distributor.endstop}>{distributor.endstop || 'None'}</option>
              )}
              {logic.endstopObjects.map((obj) => (
                <option key={obj.oid} value={obj.section}>
                  {obj.section}
                </option>
              ))}
            </select>
          </label>
          <label className={styles.checkField}>
            <input type="checkbox" checked={distributor.forward} onChange={(e) => logic.setDistributor({ forward: e.target.checked })} />
            Motor forward turns the chute clockwise
          </label>
          <NumberSetting label="Homing speed" value={distributor.homeRpm} min={0.1} step={0.5} unit="rpm" onChange={(v) => logic.setDistributor({ homeRpm: v })} />
          <NumberSetting label="Travel limit" value={distributor.maxAngle} min={1} step={1} unit="°" onChange={(v) => logic.setDistributor({ maxAngle: Math.min(360, v) })} />
          <NumberSetting label="Sections per layer" value={distributor.sections} min={1} step={1} onChange={(v) => logic.setDistributor({ sections: Math.round(v) })} />
          <NumberSetting label="Servo settle" value={distributor.doorSettleMs} min={0} step={50} unit="ms" onChange={(v) => logic.setDistributor({ doorSettleMs: v })} />
        </div>
        <p className={styles.hint}>
          The home switch is an <code>endstop_pin</code> on the chute&apos;s stepper section (Machine › Board › Wiring). Speed,
          acceleration, gearing and the layer servos are on the LEGO Sorter page.
        </p>
      </div>
    </section>
  );
};

export default ChutePage;
