import { useState } from 'react';

import styles from '../SorterPage.module.css';
import appStyles from '../../../styles/app.module.css';

function whole(value) {
  return Math.round(value).toLocaleString();
}

/**
 * Gearing for one motor: the ratio typed in or worked out from tooth counts, what that means in
 * steps, and a check that turns one rotor rev so the ratio can be corrected from a measurement.
 */
const GearCalibration = ({ config, microsteps, microstepsAssumed, stepsPerRev, slot, canTurn, onChange, onTurn }) => {
  const [turned, setTurned] = useState(false);
  const [actual, setActual] = useState(360);
  const teeth = config.teeth || {};

  const setTeeth = (patch) => {
    const next = { ...teeth, ...patch };
    const gear = next.motor > 0 && next.rotor > 0 ? next.rotor / next.motor : config.gear;
    onChange({ teeth: next, gear });
  };

  const turn = async () => {
    if (await onTurn()) {
      setActual(360);
      setTurned(true);
    }
  };

  const apply = () => {
    onChange({ gear: Math.round(((config.gear * 360) / actual) * 10000) / 10000, teeth: null });
    setTurned(false);
  };

  return (
    <fieldset className={styles.gearBox}>
      <legend className={styles.fieldLabel}>Gearing</legend>
      <div className={styles.gearFields}>
        <label className={styles.field}>
          <span className={styles.fieldLabel}>Motor turns per rotor turn</span>
          <input
            type="number"
            min="0.01"
            step="any"
            value={config.gear}
            onChange={(e) => Number(e.target.value) > 0 && onChange({ gear: Number(e.target.value), teeth: null })}
          />
        </label>
        <label className={styles.field}>
          <span className={styles.fieldLabel}>or motor gear teeth</span>
          <input type="number" min="1" step="1" value={teeth.motor ?? ''} onChange={(e) => setTeeth({ motor: Number(e.target.value) || '' })} />
        </label>
        <label className={styles.field}>
          <span className={styles.fieldLabel}>rotor gear teeth</span>
          <input type="number" min="1" step="1" value={teeth.rotor ?? ''} onChange={(e) => setTeeth({ rotor: Number(e.target.value) || '' })} />
        </label>
      </div>
      <p className={styles.statNote}>
        {whole(stepsPerRev)} steps per rotor rev
        {slot && `, ${whole((stepsPerRev * slot.degrees) / 360)} per ${slot.name}`}. {microsteps} microsteps{' '}
        {microstepsAssumed ? '(assumed; the motor isn’t in the board config)' : 'from the board config'}.
      </p>
      {turned ? (
        <div className={styles.calibrateRow}>
          <label className={styles.field}>
            <span className={styles.fieldLabel}>Once it stops, how far did the rotor really turn?</span>
            <span className={styles.fieldInput}>
              <input type="number" min="1" step="any" value={actual} onChange={(e) => setActual(Number(e.target.value))} />
              <span className={styles.unit}>°</span>
            </span>
          </label>
          <button type="button" className={appStyles.button} disabled={!(actual > 0)} onClick={apply}>
            Correct ratio
          </button>
          <button type="button" className={appStyles.buttonSecondary} onClick={() => setTurned(false)}>
            Cancel
          </button>
        </div>
      ) : (
        <div className={styles.calibrateRow}>
          <button type="button" className={appStyles.buttonSecondary} disabled={!canTurn} onClick={turn}>
            Calibrate: turn 1 rotor rev
          </button>
          <span className={styles.statNote}>Mark the rotor first. If it stops short or overshoots, enter how far it went.</span>
        </div>
      )}
    </fieldset>
  );
};

export default GearCalibration;
