import { useState } from 'react';

import {
  parseI2CAddress,
  PCA9685_CHANNELS,
  PCA9685_DEFAULTS,
  validPCA9685Name,
} from '../../../../helpers/pca9685Config';

import styles from '../GlobalSteppersPanel/GlobalSteppersPanel.module.css';
import localStyles from './PCA9685Section.module.css';
import appStyles from '../../../../styles/app.module.css';

const STATE_LABELS = {
  ok: 'Working',
  missing: 'Not found',
  error: 'Error',
  checking: 'Checking',
  idle: 'Not checked',
};

// boardStatus turns a check result (or its absence) into a badge state and message.
function boardStatus(result, connected) {
  if (!connected) {
    return { state: 'idle', label: 'Offline', message: 'Connect to check this board.' };
  }
  if (!result) {
    // In the config but not in this session: added after connecting.
    return {
      state: 'idle',
      label: STATE_LABELS.idle,
      message: 'Not in this session yet. Disconnect and connect again.',
    };
  }
  const state = STATE_LABELS[result.state] ? result.state : 'error';
  return { state, label: STATE_LABELS[state], message: result.message || '' };
}

function formError(form) {
  if (!validPCA9685Name(form.name)) {
    return 'Name: one word of letters, numbers and _';
  }
  if (Number.isNaN(parseI2CAddress(form.address))) {
    return 'Address: 0x00 to 0x7f (the board default is 0x40)';
  }
  if (!Number.isInteger(form.channels) || form.channels < 1 || form.channels > PCA9685_CHANNELS) {
    return `Servos: 1 to ${PCA9685_CHANNELS}`;
  }
  if (!(form.minPulseUs > 0 && form.maxPulseUs > form.minPulseUs && form.maxPulseUs < 20000)) {
    return 'Pulse: min must be above 0 and below max, max below 20000 µs';
  }
  return '';
}

/**
 * PCA9685 servo boards on the RAMPS I2C pins (SCL D21, SDA D20). Adding one writes a
 * [pca9685] section plus a [servo] per channel; they show up as servos after reconnecting.
 */
const PCA9685Section = ({ boards, connected, busy, onAdd, status = {}, onCheck }) => {
  const [form, setForm] = useState(null);
  const [added, setAdded] = useState('');
  const checking = Object.values(status).some((s) => s.state === 'checking');

  const error = form ? formError(form) : '';
  const field =
    (key, parse = (v) => v) =>
    (e) =>
      setForm((prev) => ({ ...prev, [key]: parse(e.target.value) }));

  const submit = async () => {
    if (error) return;
    if (await onAdd(form)) {
      setAdded(form.name);
      setForm(null);
    }
  };

  return (
    <div className={styles.groupsBlock}>
      <div className={styles.groupsHeader}>
        <p className={styles.listLabel}>PCA9685 servo boards</p>
        {!form && (
          <button
            type="button"
            className={styles.linkBtn}
            disabled={busy}
            onClick={() => {
              setAdded('');
              setForm({
                ...PCA9685_DEFAULTS,
                name: boards.length ? `servo_board${boards.length + 1}` : PCA9685_DEFAULTS.name,
              });
            }}
          >
            + Add board
          </button>
        )}
      </div>

      {boards.length > 0 && (
        <ul className={styles.groupList}>
          {boards.map((board) => {
            const { state, label, message } = boardStatus(status[board.name], connected);
            return (
              <li key={board.name} className={styles.groupCard}>
                <div className={styles.groupCardHead}>
                  <span className={styles.groupName}>
                    {board.name}
                    <span className={`${localStyles.badge} ${localStyles[state]}`}>{label}</span>
                  </span>
                </div>
                <p className={styles.groupMembers}>
                  I2C {board.address} · {board.servos} servo(s)
                </p>
                {message && (
                  <p
                    className={
                      state === 'ok' || state === 'idle' || state === 'checking'
                        ? localStyles.statusMessage
                        : localStyles.statusMessageBad
                    }
                  >
                    {message}
                  </p>
                )}
              </li>
            );
          })}
        </ul>
      )}

      {connected && boards.length > 0 && (
        <div className={styles.actions}>
          <button
            type="button"
            className={appStyles.buttonSecondary}
            disabled={checking}
            onClick={onCheck}
          >
            {checking ? 'Checking…' : 'Check boards'}
          </button>
        </div>
      )}

      {added && (
        <p className={styles.groupWarn}>
          Added {added}. {connected ? 'Disconnect and connect again' : 'Connect'} to use its servos.
        </p>
      )}

      {form && (
        <div className={styles.groupEditor}>
          <p className={styles.routinesHint}>
            Wire SDA to D20, SCL to D21, VCC to 5 V and GND; servos get their own supply on V+.
          </p>
          <label className={styles.label} htmlFor="pca-name">
            Name
          </label>
          <input
            id="pca-name"
            className={styles.input}
            value={form.name}
            onChange={field('name')}
          />
          <label className={styles.label} htmlFor="pca-address">
            I2C address
          </label>
          <input
            id="pca-address"
            className={styles.input}
            value={form.address}
            onChange={field('address')}
          />
          <label className={styles.label} htmlFor="pca-channels">
            Servos (channels 0 up)
          </label>
          <input
            id="pca-channels"
            type="number"
            min="1"
            max={PCA9685_CHANNELS}
            className={styles.input}
            value={form.channels}
            onChange={field('channels', Number)}
          />
          <label className={styles.label} htmlFor="pca-min-pulse">
            Pulse at 0° (µs)
          </label>
          <input
            id="pca-min-pulse"
            type="number"
            className={styles.input}
            value={form.minPulseUs}
            onChange={field('minPulseUs', Number)}
          />
          <label className={styles.label} htmlFor="pca-max-pulse">
            Pulse at 180° (µs)
          </label>
          <input
            id="pca-max-pulse"
            type="number"
            className={styles.input}
            value={form.maxPulseUs}
            onChange={field('maxPulseUs', Number)}
          />
          {error && <p className={styles.groupWarn}>{error}</p>}
          <div className={styles.editorActions}>
            <button
              type="button"
              className={appStyles.button}
              disabled={busy || Boolean(error)}
              onClick={submit}
            >
              {busy ? 'Adding…' : 'Add to config'}
            </button>
            <button
              type="button"
              className={appStyles.buttonSecondary}
              disabled={busy}
              onClick={() => setForm(null)}
            >
              Cancel
            </button>
          </div>
        </div>
      )}
    </div>
  );
};

export default PCA9685Section;
