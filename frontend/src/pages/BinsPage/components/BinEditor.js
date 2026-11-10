import { useMemo, useState } from 'react';

import styles from '../BinsPage.module.css';
import appStyles from '../../../styles/app.module.css';

/**
 * Chooses a bin's categories. A category already in another bin moves here when
 * saved. The parent remounts this (key) for each bin, so the draft starts fresh.
 */
const BinEditor = ({ bin, layer, categories, binOf, onSave, onPoint, onClose }) => {
  const [chosen, setChosen] = useState(() => bin.categories.map((c) => c.id));
  const [filter, setFilter] = useState('');

  const shown = useMemo(() => {
    const q = filter.trim().toLowerCase();
    return q ? categories.filter((c) => c.name.toLowerCase().includes(q) || c.id.toLowerCase().includes(q)) : categories;
  }, [categories, filter]);

  const toggle = (id) => setChosen((prev) => (prev.includes(id) ? prev.filter((c) => c !== id) : [...prev, id]));

  return (
    <div className={styles.editor}>
      <div className={styles.editorHeader}>
        <h3 className={styles.panelTitle}>
          Layer {layer}, bin {bin.bin}{' '}
          <span className={styles.muted}>
            (section {bin.section}, bin {bin.in_section} of it, {bin.angle.toFixed(1)}°)
          </span>
        </h3>
        <button type="button" className={appStyles.buttonSecondary} onClick={onClose}>
          Close
        </button>
      </div>
      {!bin.reachable && <p className={styles.alert}>The chute can&apos;t reach this bin, so it gets no pieces.</p>}

      <input className={styles.search} placeholder="Find a category" value={filter} onChange={(e) => setFilter(e.target.value)} />
      <ul className={styles.categoryList}>
        {shown.map((c) => {
          const elsewhere = binOf[c.id]?.filter((b) => !(b.layer === layer && b.bin === bin.bin));
          return (
            <li key={c.id}>
              <label className={styles.categoryItem}>
                <input type="checkbox" checked={chosen.includes(c.id)} onChange={() => toggle(c.id)} />
                <span>{c.name}</span>
                {elsewhere?.length > 0 && (
                  <span className={styles.muted}>now in {elsewhere.map((b) => `L${b.layer} bin ${b.bin}`).join(', ')}</span>
                )}
              </label>
            </li>
          );
        })}
        {shown.length === 0 && <li className={styles.muted}>No categories match.</li>}
      </ul>

      <div className={styles.buttonRow}>
        <button type="button" className={appStyles.button} onClick={() => onSave(chosen)}>
          Save
        </button>
        <button type="button" className={appStyles.buttonSecondary} onClick={() => onSave([])}>
          Unassign
        </button>
        <button type="button" className={appStyles.buttonSecondary} disabled={!bin.reachable} onClick={onPoint}>
          Point the chute here
        </button>
      </div>
    </div>
  );
};

export default BinEditor;
