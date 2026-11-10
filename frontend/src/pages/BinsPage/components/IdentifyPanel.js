import { useState } from 'react';

import { identifySorterPiece } from '../../../helpers/api';
import { ROUTE_REASONS } from '../BinsPage.logic';

import styles from '../BinsPage.module.css';
import appStyles from '../../../styles/app.module.css';

const MAX_PHOTOS = 8;
const pct = (score) => `${Math.round(score * 100)}%`;

/**
 * Identify a piece from photos (Brickognize) and see where the active profile would
 * send it, without the cameras. Nothing is saved.
 */
const IdentifyPanel = () => {
  const [files, setFiles] = useState([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [result, setResult] = useState(null);

  const identify = async () => {
    setBusy(true);
    setError('');
    setResult(null);
    try {
      setResult(await identifySorterPiece(files));
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  };

  const items = result?.brickognize.items.slice(0, 3) ?? [];
  const color = result?.brickognize.colors[0];
  const route = result?.route;

  return (
    <div className={styles.panel}>
      <h3 className={styles.panelTitle}>Identify from photos</h3>
      <p className={styles.muted}>
        Up to {MAX_PHOTOS} photos of one piece (JPEG or PNG). They go to Brickognize, which needs internet. Nothing is
        saved.
      </p>
      <div className={styles.buttonRow}>
        <input
          type="file"
          accept="image/jpeg,image/png"
          multiple
          onChange={(e) => {
            setFiles(Array.from(e.target.files || []).slice(0, MAX_PHOTOS));
            setResult(null);
          }}
        />
        <button type="button" className={appStyles.button} disabled={busy || files.length === 0} onClick={identify}>
          {busy ? 'Asking Brickognize…' : `Identify (${files.length})`}
        </button>
      </div>
      {error && <p className={styles.alert}>{error}</p>}
      {result && (
        <div className={styles.identifyResult}>
          {items.length === 0 ? (
            <p className={styles.muted}>No part found.</p>
          ) : (
            <ul className={styles.matchList}>
              {items.map((item) => (
                <li key={`${item.id}-${item.rank}`} className={styles.match}>
                  {item.img_url && <img src={item.img_url} alt="" className={styles.matchImg} />}
                  <span>
                    <strong>{item.name}</strong> <span className={styles.muted}>{item.id}</span>
                    <br />
                    <span className={styles.muted}>
                      {item.category} · {pct(item.score)}
                    </span>
                  </span>
                </li>
              ))}
            </ul>
          )}
          {color && (
            <p className={styles.routeResult}>
              Color: <strong>{color.name}</strong> <span className={styles.muted}>(id {color.id}, {pct(color.score)})</span>
            </p>
          )}
          {route && (
            <p className={styles.routeResult}>
              <strong>{route.category_name}</strong> →{' '}
              {route.choice.bin ? `layer ${route.choice.bin.layer}, bin ${route.choice.bin.bin}` : 'the bucket'}:{' '}
              {ROUTE_REASONS[route.choice.reason] ?? route.choice.reason}.
            </p>
          )}
          {result.route_error && <p className={styles.muted}>{result.route_error}</p>}
        </div>
      )}
    </div>
  );
};

export default IdentifyPanel;
