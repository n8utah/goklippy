import { useEffect, useState } from 'react';

import { useMCUSession } from '../../../../context/MCUSessionContext';

import ServoGroupsSection from './ServoGroupsSection';

import styles from '../GlobalSteppersPanel/GlobalSteppersPanel.module.css';
import appStyles from '../../../../styles/app.module.css';
import {
  clampServoAngle,
  DEFAULT_SERVO_ANGLE,
  FAST_FACTOR,
  SLOW_FACTOR,
} from '../../../../helpers/servoGroups';

function servoLabel(obj) {
  return obj.label || obj.slot_id || obj.section || `OID ${obj.oid}`;
}

const GlobalServosPanel = () => {
  const {
    connected,
    servoObjects,
    servoAngles,
    setServo,
    setAllServosAngle,
    flipAllServos,
    scaleAllServoAngles,
  } = useMCUSession();

  const [busy, setBusy] = useState(false);
  const [applyAngle, setApplyAngle] = useState(DEFAULT_SERVO_ANGLE);

  const run = async (action) => {
    setBusy(true);
    try {
      await action();
    } finally {
      setBusy(false);
    }
  };

  return (
    <aside className={styles.panel}>
      <h3 className={styles.heading}>All servos</h3>
      <p className={styles.subtitle}>Global angle controls</p>

      {!connected && (
        <p className={styles.connectHint}>Connect to move servos. Groups can be edited anytime.</p>
      )}

      <label className={styles.label} htmlFor="global-apply-angle">
        Apply angle (°)
      </label>
      <input
        id="global-apply-angle"
        type="number"
        min="0"
        max="180"
        className={styles.input}
        value={applyAngle}
        disabled={busy || !connected}
        onChange={(e) => setApplyAngle(clampServoAngle(Number(e.target.value)))}
      />

      <div className={styles.actions}>
        <button
          type="button"
          className={appStyles.button}
          disabled={busy || !connected || servoObjects.length === 0}
          onClick={() => run(() => setAllServosAngle(applyAngle))}
        >
          Apply all
        </button>
        <button
          type="button"
          className={appStyles.buttonSecondary}
          disabled={busy || !connected || servoObjects.length === 0}
          onClick={() => run(() => setAllServosAngle(DEFAULT_SERVO_ANGLE))}
        >
          Center all
        </button>
        <button
          type="button"
          className={appStyles.buttonSecondary}
          disabled={busy || !connected || servoObjects.length === 0}
          onClick={() => run(flipAllServos)}
        >
          Flip all
        </button>
      </div>

      <div className={styles.speedRow}>
        <button
          type="button"
          className={styles.speedBtn}
          disabled={busy || !connected || servoObjects.length === 0}
          onClick={() => run(() => scaleAllServoAngles(SLOW_FACTOR))}
        >
          Lower
          <span className={styles.speedHint}>×{SLOW_FACTOR}</span>
        </button>
        <button
          type="button"
          className={styles.speedBtn}
          disabled={busy || !connected || servoObjects.length === 0}
          onClick={() => run(() => scaleAllServoAngles(FAST_FACTOR))}
        >
          Raise
          <span className={styles.speedHint}>×{FAST_FACTOR}</span>
        </button>
      </div>

      {connected && servoObjects.length > 0 && (
        <div className={styles.listBlock}>
          <p className={styles.listLabel}>Individual</p>
          <ul className={styles.list}>
            {servoObjects.map((obj) => (
              <ServoRow
                key={obj.oid}
                obj={obj}
                angle={servoAngles[obj.oid] ?? DEFAULT_SERVO_ANGLE}
                busy={busy}
                onApply={(angle) => run(() => setServo(obj.oid, angle))}
              />
            ))}
          </ul>
        </div>
      )}

      <ServoGroupsSection connected={connected} busy={busy} onRun={run} />

      {connected && servoObjects.length > 0 && (
        <p className={styles.footerMeta}>{servoObjects.length} servo(s) configured</p>
      )}
    </aside>
  );
};

function ServoRow({ obj, angle, busy, onApply }) {
  const [localAngle, setLocalAngle] = useState(angle);

  useEffect(() => {
    setLocalAngle(angle);
  }, [angle]);

  return (
    <li className={styles.listItem}>
      <span className={styles.listName}>{servoLabel(obj)}</span>
      <div className={styles.servoRowControls}>
        <button
          type="button"
          className={styles.stepBtnInline}
          aria-label="Decrease angle by 5"
          disabled={busy}
          onClick={() => setLocalAngle((prev) => clampServoAngle(prev - 5))}
        >
          −
        </button>
        <input
          type="number"
          min="0"
          max="180"
          className={styles.memberSpeedInput}
          value={localAngle}
          disabled={busy}
          onChange={(e) => setLocalAngle(clampServoAngle(Number(e.target.value)))}
        />
        <button
          type="button"
          className={styles.stepBtnInline}
          aria-label="Increase angle by 5"
          disabled={busy}
          onClick={() => setLocalAngle((prev) => clampServoAngle(prev + 5))}
        >
          +
        </button>
      </div>
      <input
        type="range"
        min="0"
        max="180"
        value={localAngle}
        disabled={busy}
        className={styles.servoRange}
        onChange={(e) => setLocalAngle(Number(e.target.value))}
      />
      <button
        type="button"
        className={appStyles.buttonSecondary}
        disabled={busy}
        onClick={() => onApply(localAngle)}
      >
        Apply
      </button>
    </li>
  );
}

export default GlobalServosPanel;
