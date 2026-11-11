import { useEffect, useState } from 'react';

import { getSorterCameraPreviewStatus, sorterCameraPreviewURL } from '../../../helpers/api';

import styles from '../CamerasPage.module.css';

const STATUS_POLL_MS = 2000;

/** One camera's live picture, by device id, with how its capture is doing. */
const DevicePreview = ({ deviceId }) => {
  const [status, setStatus] = useState(null);
  const [error, setError] = useState('');
  const [attempt, setAttempt] = useState(0); // remounts the stream after a failure

  useEffect(() => {
    let cancelled = false;
    const poll = () =>
      getSorterCameraPreviewStatus(deviceId).then(
        (data) => {
          if (cancelled) return;
          setStatus(data.camera);
          setError('');
        },
        (err) => {
          if (!cancelled) setError(err.message);
        },
      );
    poll();
    const timer = setInterval(poll, STATUS_POLL_MS);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, [deviceId]);

  const cam = status;
  return (
    <div className={styles.preview}>
      <img
        key={attempt}
        className={styles.previewImg}
        src={sorterCameraPreviewURL(deviceId)}
        alt="Live picture"
        onError={() => setTimeout(() => setAttempt((n) => n + 1), STATUS_POLL_MS)}
      />
      <p className={styles.hint}>
        {error
          ? error
          : cam?.running
            ? `Live · ${cam.fps ? cam.fps.toFixed(1) : '…'} fps`
            : cam?.error
              ? `Not running: ${cam.error}`
              : 'Starting the camera…'}
      </p>
    </div>
  );
};

export default DevicePreview;
