import { useState } from 'react';

import { listSorterCameraModes } from '../../../helpers/api';

import styles from '../CamerasPage.module.css';
import appStyles from '../../../styles/app.module.css';

/** The sizes a camera offers, checked on request, against the size every camera uses. */
const DeviceModes = ({ deviceId, width, height, disabled }) => {
  const [state, setState] = useState({ loading: false, sizes: null, error: '' });

  const check = async () => {
    setState({ loading: true, sizes: null, error: '' });
    try {
      const data = await listSorterCameraModes(deviceId);
      const sizes = [];
      for (const m of data.modes) {
        const key = `${m.width}×${m.height}`;
        const found = sizes.find((s) => s.key === key);
        if (found) found.fps = Math.max(found.fps, m.fps);
        else sizes.push({ key, width: m.width, height: m.height, fps: m.fps });
      }
      sizes.sort((a, b) => a.width * a.height - b.width * b.height);
      setState({ loading: false, sizes, error: data.error || '' });
    } catch (err) {
      setState({ loading: false, sizes: null, error: err.message });
    }
  };

  const fits = state.sizes?.some((s) => s.width === width && s.height === height);
  return (
    <div className={styles.modes}>
      <button
        type="button"
        className={appStyles.buttonSecondary}
        disabled={disabled || state.loading}
        onClick={check}
      >
        {state.loading ? 'Asking the camera…' : 'Check sizes'}
      </button>
      {state.error && <span className={styles.hint}>{state.error}</span>}
      {state.sizes && state.sizes.length > 0 && (
        <span className={fits ? styles.hint : styles.warnText}>
          {fits
            ? `Takes ${width}×${height}. `
            : `Doesn’t offer ${width}×${height}, the size set for every camera, so it won’t open at it. Change the size under All cameras on any channel tab. `}
          Offers: {state.sizes.map((s) => `${s.key} (${s.fps} fps)`).join(', ')}
        </span>
      )}
    </div>
  );
};

export default DeviceModes;
