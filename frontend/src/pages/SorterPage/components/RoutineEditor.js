import { useState } from 'react';

import { newMove, newSorterRoutine } from '../SorterPage.logic';

import styles from '../SorterPage.module.css';
import appStyles from '../../../styles/app.module.css';

function PositiveInput({ label, value, unit, onChange }) {
  return (
    <label className={styles.field}>
      <span className={styles.fieldLabel}>{label}</span>
      <span className={styles.fieldInput}>
        <input
          type="number"
          min="0.01"
          step="any"
          value={value}
          onChange={(e) => Number(e.target.value) > 0 && onChange(Number(e.target.value))}
        />
        <span className={styles.unit}>{unit}</span>
      </span>
    </label>
  );
}

/** Sorter routines: moves in rotor degrees and rpm, repeated in order until stopped. */
const RoutineEditor = ({ routines, onChange }) => {
  const [selectedId, setSelectedId] = useState(routines[0]?.id ?? '');
  const routine = routines.find((r) => r.id === selectedId) || routines[0];

  const update = (patch) => onChange(routines.map((r) => (r.id === routine.id ? { ...r, ...patch } : r)));
  const updateMove = (id, patch) => update({ moves: routine.moves.map((m) => (m.id === id ? { ...m, ...patch } : m)) });

  const add = () => {
    const created = newSorterRoutine();
    onChange([...routines, created]);
    setSelectedId(created.id);
  };

  const remove = () => {
    onChange(routines.filter((r) => r.id !== routine.id));
    setSelectedId('');
  };

  return (
    <section className={styles.routineEditor}>
      <header className={styles.cardHeader}>
        <h3 className={styles.cardTitle}>Routines</h3>
        <button type="button" className={appStyles.buttonSecondary} onClick={add}>
          New routine
        </button>
      </header>
      <p className={styles.statNote}>
        Moves are rotor degrees and rpm, so a routine does the same motion on any channel, and repeat in order until
        stopped. Forward follows each channel&apos;s direction setting.
      </p>

      {routine ? (
        <>
          <div className={styles.fields}>
            <label className={styles.field}>
              <span className={styles.fieldLabel}>Routine</span>
              <select value={routine.id} onChange={(e) => setSelectedId(e.target.value)}>
                {routines.map((r) => (
                  <option key={r.id} value={r.id}>
                    {r.name}
                  </option>
                ))}
              </select>
            </label>
            <label className={styles.field}>
              <span className={styles.fieldLabel}>Name</span>
              <input value={routine.name} onChange={(e) => update({ name: e.target.value })} />
            </label>
            <button type="button" className={`${appStyles.buttonSecondary} ${styles.alignEnd}`} onClick={remove}>
              Delete routine
            </button>
          </div>

          {routine.moves.map((move, i) => (
            <div key={move.id} className={styles.moveRow}>
              <span className={styles.moveNumber}>{i + 1}</span>
              <label className={styles.field}>
                <span className={styles.fieldLabel}>Direction</span>
                <select value={move.forward ? 'forward' : 'reverse'} onChange={(e) => updateMove(move.id, { forward: e.target.value === 'forward' })}>
                  <option value="forward">Forward</option>
                  <option value="reverse">Reverse</option>
                </select>
              </label>
              <PositiveInput label="Turn" value={move.degrees} unit="°" onChange={(degrees) => updateMove(move.id, { degrees })} />
              <PositiveInput label="Speed" value={move.rpm} unit="rpm" onChange={(rpm) => updateMove(move.id, { rpm })} />
              <button
                type="button"
                className={`${appStyles.buttonSecondary} ${styles.alignEnd}`}
                disabled={routine.moves.length === 1}
                onClick={() => update({ moves: routine.moves.filter((m) => m.id !== move.id) })}
              >
                Remove
              </button>
            </div>
          ))}
          <div>
            <button type="button" className={appStyles.buttonSecondary} onClick={() => update({ moves: [...routine.moves, newMove()] })}>
              Add move
            </button>
          </div>
        </>
      ) : (
        <p className={appStyles.muted}>No routines yet.</p>
      )}
    </section>
  );
};

export default RoutineEditor;
