import { CHANNEL_IDS, FEEDER_IDS, useSorterPageLogic } from './SorterPage.logic';
import ChannelCard from './components/ChannelCard';
import Distributor from './components/Distributor';
import MachineDiagram from './components/MachineDiagram';

import styles from './SorterPage.module.css';
import appStyles from '../../styles/app.module.css';

function NumberField({ label, value, min, step, unit, onChange }) {
  return (
    <label className={styles.field}>
      <span className={styles.fieldLabel}>{label}</span>
      <span className={styles.fieldInput}>
        <input
          type="number"
          min={min}
          step={step}
          value={value}
          onChange={(e) => {
            const next = Number(e.target.value);
            if (next >= min) onChange(next);
          }}
        />
        {unit && <span className={styles.unit}>{unit}</span>}
      </span>
    </label>
  );
}

const SorterPage = ({ onGoBoard }) => {
  const logic = useSorterPageLogic();
  const { connected, channels, settings, setSetting, speedsOrdered } = logic;

  const feedersRunning = FEEDER_IDS.some((id) => channels[id].running);
  const { showSettings } = settings;

  return (
    <section className={appStyles.card}>
      <div className={styles.headerRow}>
        <div>
          <h2 className={styles.title}>LEGO Sorter</h2>
          <p className={appStyles.muted}>
            {connected ? (
              <>
                Live on <strong>{logic.port}</strong>
                {logic.wsConnected ? '' : ' — waiting for live data…'}
              </>
            ) : (
              'Not connected. Controls are off until a board is connected.'
            )}
          </p>
        </div>
        <div className={styles.headerActions}>
          <button type="button" className={appStyles.buttonSecondary} aria-expanded={showSettings} onClick={() => setSetting('showSettings', !showSettings)}>
            {showSettings ? 'Hide settings' : 'Show settings'}
          </button>
          {!connected && (
            <button type="button" className={appStyles.button} onClick={onGoBoard}>
              Connect on Board Setup
            </button>
          )}
          <button type="button" className={appStyles.buttonSecondary} disabled={!connected || logic.resetting} onClick={logic.handleReset}>
            {logic.resetting ? 'Resetting…' : 'Reset board'}
          </button>
          <button type="button" className={styles.stopAll} disabled={!connected} onClick={logic.stopAll}>
            Stop all
          </button>
        </div>
      </div>

      {logic.shutdownReason && (
        <p className={styles.alert}>
          Board shut down: <strong>{logic.shutdownReason}</strong>. Reset the board, or start a channel to clear it.
        </p>
      )}
      {logic.error && <p className={appStyles.errorText}>{logic.error}</p>}

      <MachineDiagram channels={channels} indexDegrees={settings.indexDegrees} />

      <div className={styles.controlBar}>
        <div className={styles.controlGroup}>
          <h3 className={styles.groupTitle}>Feeder speeds</h3>
          {showSettings && (
          <div className={styles.fields}>
            <NumberField label="C1 rotor" value={settings.baseRpm} min={0.1} step={0.5} unit="rpm" onChange={(v) => setSetting('baseRpm', v)} />
            <NumberField label="C2 vs C1" value={settings.c2Factor} min={0.1} step={0.1} unit="×" onChange={(v) => setSetting('c2Factor', v)} />
            <NumberField label="C3 vs C2" value={settings.c3Factor} min={0.1} step={0.1} unit="×" onChange={(v) => setSetting('c3Factor', v)} />
          </div>
          )}
          <p className={speedsOrdered ? styles.orderOk : styles.orderBad}>
            {speedsOrdered ? '✓ C1 < C2 < C3' : '⚠ Speeds must increase: C1 < C2 < C3'}
          </p>
          {feedersRunning ? (
            <button type="button" className={appStyles.button} onClick={logic.stopFeeders}>
              Stop feeders
            </button>
          ) : (
            <button type="button" className={appStyles.button} disabled={!connected} onClick={logic.startFeeders}>
              Run feeders
            </button>
          )}
        </div>

        <div className={styles.controlGroup}>
          <h3 className={styles.groupTitle}>C4 indexing</h3>
          {showSettings && (
          <div className={styles.fields}>
            <NumberField label="Per move" value={settings.indexDegrees} min={1} step={1} unit="°" onChange={(v) => setSetting('indexDegrees', v)} />
            <NumberField label="Speed" value={settings.indexRpm} min={0.1} step={1} unit="rpm" onChange={(v) => setSetting('indexRpm', v)} />
            <NumberField label="Dwell" value={settings.dwellMs} min={0} step={100} unit="ms" onChange={(v) => setSetting('dwellMs', v)} />
          </div>
          )}
          <div className={styles.buttonRow}>
            {channels.c4.indexing ? (
              <button type="button" className={appStyles.button} onClick={() => logic.stopChannel(channels.c4)}>
                Stop indexing
              </button>
            ) : (
              <button type="button" className={appStyles.button} disabled={channels.c4.oid == null} onClick={() => logic.runIndex(0)}>
                Auto index
              </button>
            )}
          </div>
        </div>
      </div>

      <div className={styles.cards}>
        {CHANNEL_IDS.map((id) => (
          <ChannelCard
            key={id}
            channel={channels[id]}
            trace={logic.history[channels[id].oid] || []}
            stepperObjects={logic.stepperObjects}
            indexDegrees={settings.indexDegrees}
            connected={connected}
            showSettings={showSettings}
            onConfig={(patch) => logic.setChannelConfig(id, patch)}
            onStart={() => logic.startChannel(channels[id])}
            onStop={() => logic.stopChannel(channels[id])}
            onZero={() => logic.zeroChannel(channels[id])}
            onForwardSlot={() => logic.runIndex(1, true)}
            routines={logic.routines}
            routineId={logic.routineIdFor(id)}
            onChooseRoutine={(routineId) => logic.chooseRoutine(id, routineId)}
            onRunRoutine={() => logic.runRoutine(channels[id])}
          />
        ))}
      </div>

      <Distributor logic={logic} showSettings={showSettings} connected={connected} />

      <p className={styles.footnote}>
        Speed and direction come from the MCU&apos;s step counter. The steppers have no encoders, so skipped steps on
        the shaft won&apos;t show here.
      </p>
    </section>
  );
};

export default SorterPage;
