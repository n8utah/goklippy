import styles from '../TuningPage.module.css';

const NUMBERS = [
  { key: 'topOhms', label: 'Resistor from 12 V to the pin', unit: 'Ω', step: 100 },
  { key: 'bottomOhms', label: 'Resistor from the pin to ground', unit: 'Ω', step: 100 },
  {
    key: 'referenceV',
    label: 'ADC reference',
    unit: 'V',
    step: 0.01,
    hint: 'The Mega’s 5 V. Measure it for a closer reading.',
  },
  {
    key: 'calibration',
    label: 'Calibration',
    unit: '×',
    step: 0.005,
    hint: 'Meter reading ÷ shown reading, to correct the resistors’ tolerance.',
  },
  {
    key: 'minVolts',
    label: 'Hold back below',
    unit: 'V',
    step: 0.5,
    hint: 'Homing and sorting are refused, and a run stops, when the supply drops under this. 0 turns that off.',
  },
];

/** The motors' supply voltage: which sensor reads it and how to turn its reading into volts. */
const SupplyPanel = ({ logic }) => {
  const sensor = logic.value('supply', 'sensor');
  return (
    <div className={styles.panel}>
      <div className={styles.panelHeader}>
        <h3 className={styles.panelTitle}>Motor power</h3>
      </div>
      <p className={styles.muted}>
        The 12 V supply to the RAMPS 5 A input (motors and D10), read through a resistor divider
        into a free analog pin. Add it to the board config as{' '}
        <code>[{sensor || 'voltage_sensor supply'}]</code> with <code>sensor_pin: analog5</code> (or
        the pin it&apos;s wired to). Until then it shows as not measured and holds nothing back. The
        reading is on the Dashboard.
      </p>
      <div className={styles.row}>
        <div className={styles.rowText}>
          <strong>Board config section</strong>
        </div>
        <div className={styles.rowInput}>
          <input
            className={styles.textInput}
            value={sensor}
            onChange={(e) => logic.set('supply', 'sensor', e.target.value)}
          />
        </div>
      </div>
      {NUMBERS.map((f) => (
        <div key={f.key} className={styles.row}>
          <div className={styles.rowText}>
            <strong>{f.label}</strong>
            {f.hint && <span className={styles.muted}>{f.hint}</span>}
          </div>
          <div className={styles.rowInput}>
            <input
              type="number"
              min="0"
              step={f.step}
              value={logic.value('supply', f.key)}
              onChange={(e) => {
                if (e.target.value !== '' && Number(e.target.value) >= 0)
                  logic.set('supply', f.key, Number(e.target.value));
              }}
            />
            <span className={styles.muted}>{f.unit}</span>
          </div>
        </div>
      ))}
    </div>
  );
};

export default SupplyPanel;
