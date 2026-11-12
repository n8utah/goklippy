import { useEffect, useState } from 'react';

import { getSorterCameraPreviewStatus, sorterCameraPreviewURL } from '@/helpers/api';

const STATUS_POLL_MS = 2000;

/** One camera's live picture, by device id, with how its capture is doing. */
export default function DevicePreview({ deviceId }) {
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
        (err) => !cancelled && setError(err.message),
      );
    poll();
    const timer = setInterval(poll, STATUS_POLL_MS);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, [deviceId]);

  return (
    <div className="flex flex-col gap-1">
      <img
        key={attempt}
        className="aspect-[4/3] w-full rounded-lg bg-media object-contain"
        src={sorterCameraPreviewURL(deviceId)}
        alt="Live picture"
        onError={() => setTimeout(() => setAttempt((n) => n + 1), STATUS_POLL_MS)}
      />
      <p className="text-xs text-muted-foreground">
        {error
          ? error
          : status?.running
            ? `Live · ${status.fps ? status.fps.toFixed(1) : '…'} fps`
            : status?.error
              ? `Not running: ${status.error}`
              : 'Starting the camera…'}
      </p>
    </div>
  );
}
