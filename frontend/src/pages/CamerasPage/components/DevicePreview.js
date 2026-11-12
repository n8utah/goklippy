import { useEffect, useState } from 'react';

import { getSorterCameraPreviewStatus, sorterCameraPreviewURL } from '@/helpers/api';
import { cn } from '@/lib/utils';

const STATUS_POLL_MS = 2000;

/** One camera's live picture, by device id, with how its capture is doing. */
export default function DevicePreview({ deviceId }) {
  const [status, setStatus] = useState(null);
  const [error, setError] = useState('');
  const [attempt, setAttempt] = useState(0); // remounts the stream after a failure
  const [loaded, setLoaded] = useState(false);

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
        // 4:3 until the first frame; then the camera's own shape, never stretched.
        className={cn('w-full rounded-lg bg-media object-contain', !loaded && 'aspect-[4/3]')}
        src={sorterCameraPreviewURL(deviceId)}
        alt="Live picture"
        onLoad={() => setLoaded(true)}
        onError={() => {
          setLoaded(false);
          setTimeout(() => setAttempt((n) => n + 1), STATUS_POLL_MS);
        }}
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
