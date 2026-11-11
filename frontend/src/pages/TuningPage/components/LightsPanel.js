import styles from '../TuningPage.module.css';

/** The machine's light: which board output it is, and whether sorting turns it on. */
const LightsPanel = ({ logic }) => {
  const output = logic.value('lights', 'output');
  const onWhenSorting = logic.value('lights', 'onWhenSorting');
  return (
    <div className={styles.panel}>
      <div className={styles.panelHeader}>
        <h3 className={styles.panelTitle}>Light</h3>
      </div>
      <p className={styles.muted}>
        The light runs off the RAMPS D10 output. Add it to the board config on Board Setup as <code>[{output || 'output_pin leds'}]</code> with{' '}
        <code>pin: ar10</code>, then switch it from the Dashboard.
      </p>
      <div className={styles.row}>
        <div className={styles.rowText}>
          <strong>Board config section</strong>
          <span className={styles.muted}>The [output_pin] the light is wired to.</span>
        </div>
        <div className={styles.rowInput}>
          <input className={styles.textInput} value={output} onChange={(e) => logic.set('lights', 'output', e.target.value)} />
        </div>
      </div>
      <div className={styles.row}>
        <div className={styles.rowText}>
          <strong>Turn on when sorting starts</strong>
          <span className={styles.muted}>The cameras need it. Starting or resuming a sort switches it on; it stays on afterwards.</span>
        </div>
        <div className={styles.rowInput}>
          <input type="checkbox" checked={onWhenSorting} onChange={(e) => logic.set('lights', 'onWhenSorting', e.target.checked)} />
        </div>
      </div>
    </div>
  );
};

export default LightsPanel;
