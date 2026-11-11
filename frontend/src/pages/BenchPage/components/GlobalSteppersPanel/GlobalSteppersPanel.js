import { useState } from 'react';

import { useMCUSession } from '../../../../context/MCUSessionContext';

import StepperGroupsSection from './StepperGroupsSection';
import StepperRoutinesSection from './StepperRoutinesSection';

import styles from './GlobalSteppersPanel.module.css';
import appStyles from '../../../../styles/app.module.css';
import { DEFAULT_STEPPER_SPEED, FAST_FACTOR, SLOW_FACTOR } from '../../../../helpers/stepperGroups';

function stepperLabel(obj) {
  return obj.label || obj.slot_id || obj.section || `OID ${obj.oid}`;
}

const GlobalSteppersPanel = () => {
  const {
    connected,
    stepperObjects,
    stepperJogState,
    startAllSteppers,
    stopAllSteppers,
    reverseAllSteppers,
    scaleAllStepperSpeeds,
  } = useMCUSession();

  const [busy, setBusy] = useState(false);
  const [startSpeed, setStartSpeed] = useState(DEFAULT_STEPPER_SPEED);
  const [actionError, setActionError] = useState('');

  const clampSpeed = (value) => Math.min(50000, Math.max(1, value));

  const runningEntries = Object.entries(stepperJogState);
  const hasRunning = runningEntries.length > 0;

  const run = async (action) => {
    setBusy(true);
    setActionError('');
    try {
      await action();
    } catch (err) {
      setActionError(err.message || 'Stepper command failed');
    } finally {
      setBusy(false);
    }
  };

  return (
    <aside className={styles.panel}>
      <h3 className={styles.heading}>All steppers</h3>
      <p className={styles.subtitle}>Global controls for continuous jog</p>

      {!connected && (
        <p className={styles.connectHint}>Connect to run steppers. Groups can be edited anytime.</p>
      )}

      {actionError && <p className={styles.groupWarn}>{actionError}</p>}

      <label className={styles.label} htmlFor="global-start-speed">
        Start speed (steps/s)
      </label>
      <input
        id="global-start-speed"
        type="number"
        min="1"
        max="50000"
        className={styles.input}
        value={startSpeed}
        disabled={busy || !connected}
        onChange={(e) => setStartSpeed(clampSpeed(Number(e.target.value) || 1))}
      />

      <div className={styles.actions}>
        <button
          type="button"
          className={appStyles.button}
          disabled={busy || !connected || stepperObjects.length === 0}
          onClick={() => run(() => startAllSteppers({ forward: true, speedHz: startSpeed }))}
        >
          Start all
        </button>
        <button
          type="button"
          className={appStyles.buttonSecondary}
          disabled={busy || !connected || stepperObjects.length === 0}
          onClick={() => run(stopAllSteppers)}
        >
          Stop all
        </button>
        <button
          type="button"
          className={appStyles.buttonSecondary}
          disabled={busy || !connected || !hasRunning}
          onClick={() => run(reverseAllSteppers)}
        >
          Reverse all
        </button>
      </div>

      <div className={styles.speedRow}>
        <button
          type="button"
          className={styles.speedBtn}
          disabled={busy || !connected || !hasRunning}
          onClick={() => run(() => scaleAllStepperSpeeds(SLOW_FACTOR))}
        >
          Slower
          <span className={styles.speedHint}>×{SLOW_FACTOR}</span>
        </button>
        <button
          type="button"
          className={styles.speedBtn}
          disabled={busy || !connected || !hasRunning}
          onClick={() => run(() => scaleAllStepperSpeeds(FAST_FACTOR))}
        >
          Faster
          <span className={styles.speedHint}>×{FAST_FACTOR}</span>
        </button>
      </div>

      {connected && (
        <div className={styles.listBlock}>
          <p className={styles.listLabel}>Running</p>
          {!hasRunning && <p className={appStyles.muted}>No continuous jogs active</p>}
          {hasRunning && (
            <ul className={styles.list}>
              {runningEntries.map(([oidStr, state]) => {
                const oid = Number(oidStr);
                const obj = stepperObjects.find((item) => item.oid === oid);
                return (
                  <li key={oidStr} className={styles.listItem}>
                    <span className={styles.listName}>
                      {obj ? stepperLabel(obj) : `OID ${oid}`}
                    </span>
                    <span className={styles.listMeta}>
                      {state.forward ? 'Fwd' : 'Rev'} · {state.speedHz} steps/s
                    </span>
                  </li>
                );
              })}
            </ul>
          )}
        </div>
      )}

      <StepperGroupsSection connected={connected} busy={busy} onRun={run} />

      <StepperRoutinesSection connected={connected} busy={busy} onRun={run} />

      {connected && stepperObjects.length > 0 && (
        <p className={styles.footerMeta}>{stepperObjects.length} stepper(s) configured</p>
      )}
    </aside>
  );
};

export default GlobalSteppersPanel;
