import { SECTIONS, useTuningPageLogic } from './TuningPage.logic';
import LightsPanel from './components/LightsPanel';
import SupplyPanel from './components/SupplyPanel';

import styles from './TuningPage.module.css';
import appStyles from '../../styles/app.module.css';

const TuningPage = () => {
  const logic = useTuningPageLogic();

  return (
    <section className={appStyles.card}>
      <h2 className={styles.title}>Tuning</h2>
      <p className={appStyles.muted}>
        Changes save on their own and apply to a running sort straight away. Change a few at a time
        and watch the next pieces.
      </p>
      {logic.error && <p className={styles.alert}>{logic.error}</p>}

      {SECTIONS.map((section) => (
        <div key={section.group} className={styles.panel}>
          <div className={styles.panelHeader}>
            <h3 className={styles.panelTitle}>{section.title}</h3>
            <button
              type="button"
              className={appStyles.buttonSecondary}
              onClick={() => logic.resetAll(section.group)}
            >
              Reset all to defaults
            </button>
          </div>
          <p className={styles.muted}>{section.blurb}</p>
          {section.fields.map((f) => {
            const v = logic.value(section.group, f.key);
            const d = logic.defaultOf(section.group, f.key);
            const changed = v !== d;
            return (
              <div key={f.key} className={changed ? styles.rowChanged : styles.row}>
                <div className={styles.rowText}>
                  <strong>{f.label}</strong>
                  {f.hint && <span className={styles.muted}>{f.hint}</span>}
                </div>
                <div className={styles.rowInput}>
                  <input
                    type="number"
                    min="0"
                    step={f.step}
                    value={v}
                    onChange={(e) => {
                      if (e.target.value !== '' && Number(e.target.value) >= 0)
                        logic.set(section.group, f.key, Number(e.target.value));
                    }}
                  />
                  {f.unit && <span className={styles.muted}>{f.unit}</span>}
                  {changed && (
                    <button
                      type="button"
                      className={styles.reset}
                      onClick={() => logic.set(section.group, f.key, d)}
                    >
                      Reset to {d}
                    </button>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      ))}
      <LightsPanel logic={logic} />
      <SupplyPanel logic={logic} />
    </section>
  );
};

export default TuningPage;
