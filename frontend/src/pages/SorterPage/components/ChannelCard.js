import { indexAlignment } from '../SorterPage.logic';
import SpeedTrace from './SpeedTrace';

import styles from '../SorterPage.module.css';
import appStyles from '../../../styles/app.module.css';

const NAMES = { c1: 'C1 feeder', c2: 'C2 feeder', c3: 'C3 feeder', c4: 'C4 carousel' };

function directionText(direction) {
  if (direction > 0) return 'Forward';
  if (direction < 0) return 'Reverse';
  return 'Stopped';
}

function Stat({ label, value, note }) {
  return (
    <div className={styles.stat}>
      <span className={styles.statLabel}>{label}</span>
      <span className={styles.statValue}>{value}</span>
      {note && <span className={styles.statNote}>{note}</span>}
    </div>
  );
}

const ChannelCard = ({
  channel,
  trace,
  stepperObjects,
  indexDegrees,
  connected,
  showSettings,
  routines,
  routineId,
  onChooseRoutine,
  onRunRoutine,
  onConfig,
  onStart,
  onStop,
  onZero,
  onForwardSlot,
}) => {
  const isCarousel = channel.id === 'c4';
  const missing = channel.oid == null;
  const alignment = indexAlignment(channel.angle, indexDegrees);

  return (
    <article className={styles.card}>
      <header className={styles.cardHeader}>
        <h3 className={styles.cardTitle}>{NAMES[channel.id]}</h3>
        <span className={channel.direction !== 0 ? styles.badgeOn : styles.badgeOff}>{directionText(channel.direction)}</span>
      </header>

      {connected && missing && <p className={styles.alert}>{channel.config.stepper} isn&apos;t in the connected config.</p>}
      {channel.error && <p className={appStyles.errorText}>{channel.error}</p>}

      <div className={styles.stats}>
        <Stat label="Measured" value={`${channel.measuredRpm.toFixed(1)} rpm`} note={`set ${channel.targetRpm.toFixed(1)} rpm`} />
        {isCarousel && (
          <Stat label="Slot" value={`${alignment.slot + 1} of ${alignment.slots}`} note={`${alignment.offset >= 0 ? '+' : ''}${alignment.offset.toFixed(1)}° off`} />
        )}
      </div>

      <SpeedTrace values={trace} stepsPerRev={channel.stepsPerRev} />

      {showSettings && (
        <>
      <div className={styles.configRow}>
        <label className={styles.field}>
          <span className={styles.fieldLabel}>Motor</span>
          <select value={channel.config.stepper} onChange={(e) => onConfig({ stepper: e.target.value })}>
            {!stepperObjects.some((obj) => obj.section === channel.config.stepper) && (
              <option value={channel.config.stepper}>{channel.config.stepper}</option>
            )}
            {stepperObjects.map((obj) => (
              <option key={obj.oid} value={obj.section}>
                {obj.section}
              </option>
            ))}
          </select>
        </label>
        <label className={styles.field}>
          <span className={styles.fieldLabel}>Gear ratio (motor:rotor)</span>
          <input
            type="number"
            min="0.1"
            step="0.1"
            value={channel.config.gear}
            onChange={(e) => {
              const gear = Number(e.target.value);
              if (gear > 0) onConfig({ gear });
            }}
          />
        </label>
      </div>

          <label className={styles.field}>
            <span className={styles.fieldLabel}>Routine</span>
            <select value={routineId} disabled={routines.length === 0} onChange={(e) => onChooseRoutine(e.target.value)}>
              {routines.length === 0 && <option value="">No saved routines</option>}
              {routines.map((routine) => (
                <option key={routine.id} value={routine.id}>
                  {routine.name}
                </option>
              ))}
            </select>
          </label>
          <div className={styles.buttonRow}>
            <button type="button" className={appStyles.buttonSecondary} onClick={() => onConfig({ forward: !channel.config.forward })}>
              Direction: {channel.config.forward ? 'Forward' : 'Reverse'}
            </button>
            {isCarousel && (
              <button type="button" className={appStyles.buttonSecondary} disabled={missing} onClick={onZero}>
                Set as slot 1
              </button>
            )}
          </div>
        </>
      )}

      <div className={styles.buttonRow}>
        {isCarousel && (
          <button
            type="button"
            className={appStyles.button}
            disabled={missing || channel.indexing || channel.direction !== 0}
            onClick={onForwardSlot}
          >
            Forward 1 slot ({indexDegrees}°)
          </button>
        )}
        {!isCarousel &&
          (channel.running ? (
            <button type="button" className={appStyles.button} onClick={onStop}>
              Stop
            </button>
          ) : (
            <button type="button" className={appStyles.button} disabled={missing} onClick={onStart}>
              Run
            </button>
          ))}
        {channel.routineRunning ? (
          <button type="button" className={appStyles.button} onClick={onStop}>
            Stop routine
          </button>
        ) : (
          <button type="button" className={appStyles.buttonSecondary} disabled={missing || !routineId} onClick={onRunRoutine}>
            Run routine
          </button>
        )}
      </div>
    </article>
  );
};

export default ChannelCard;
