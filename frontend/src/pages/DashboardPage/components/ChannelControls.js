import { useState } from 'react';

import styles from '../DashboardPage.module.css';
import appStyles from '../../../styles/app.module.css';

const CHANNELS = [
  { id: 'c1', label: 'C1' },
  { id: 'c2', label: 'C2' },
  { id: 'c3', label: 'C3' },
  { id: 'c4', label: 'C4' },
];

/** Turn a channel by hand, or shake it to free a stuck piece. */
const ChannelControls = ({ shakes, disabled, onTurn, onShake }) => {
  const [amount, setAmount] = useState(10);
  const [shake, setShake] = useState('heavy');

  return (
    <div className={styles.controls}>
      <h3 className={styles.panelTitle}>Turn or shake a channel</h3>
      <p className={styles.muted}>These move the real channel at once. Pause or stop the sort first.</p>
      <div className={styles.controlRow}>
        <label className={styles.inlineField}>
          Turn by
          <input type="number" min="0.1" max="360" step="1" value={amount} onChange={(e) => Number(e.target.value) > 0 && setAmount(Number(e.target.value))} />°
        </label>
        <label className={styles.inlineField}>
          Shake
          <select value={shake} onChange={(e) => setShake(e.target.value)}>
            {shakes.map((s) => (
              <option key={s.key} value={s.key} title={s.blurb}>
                {s.label}
              </option>
            ))}
          </select>
        </label>
      </div>
      <div className={styles.channelButtons}>
        {CHANNELS.map((ch) => (
          <div key={ch.id} className={styles.channelButtonGroup}>
            <strong>{ch.label}</strong>
            <button type="button" className={appStyles.buttonSecondary} disabled={disabled} onClick={() => onTurn(ch.id, -amount)} title="Back">
              ←
            </button>
            <button type="button" className={appStyles.buttonSecondary} disabled={disabled} onClick={() => onTurn(ch.id, amount)} title="Forward">
              →
            </button>
            <button type="button" className={appStyles.buttonSecondary} disabled={disabled} onClick={() => onShake(ch.id, shake)}>
              Shake
            </button>
          </div>
        ))}
      </div>
    </div>
  );
};

export default ChannelControls;
