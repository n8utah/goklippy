import { Link } from 'react-router';

import CameraView from '@/components/CameraView/CameraView';
import { Badge } from '@/components/ui/badge';

/**
 * One channel's camera: live picture with zones, and what's on it now. A camera that's
 * running shows its picture even when it can't see pieces yet, with the reason (no
 * zones, empty-channel pictures to capture again, …) and where to fix it.
 */
export default function CameraTile({ channel, summary, camera, simulated }) {
  const live = summary?.valid && summary.age_ms < 2000;
  const running = Boolean(camera?.camera?.running);
  const zones = [
    summary?.in_drop && 'drop',
    summary?.in_precise && 'precise',
    summary?.in_exit && 'exit',
  ].filter(Boolean);
  // Why a running camera isn't seeing pieces.
  const why = !summary?.valid && running ? camera.message || 'Not seeing pieces yet.' : '';

  return (
    <div className="flex min-w-0 flex-col gap-2 rounded-xl border border-border bg-card p-3">
      <div className="flex items-center justify-between gap-2">
        <p className="font-medium">
          {channel.label} <span className="text-sm text-muted-foreground">· {channel.name}</span>
        </p>
        <Badge variant={live ? 'running' : summary?.valid || running ? 'warning' : 'idle'}>
          {live ? 'Live' : summary?.valid ? 'Stale' : running ? 'Not seeing' : 'No camera'}
        </Badge>
      </div>
      {simulated || summary?.valid || running ? (
        <div className="overflow-hidden rounded-lg bg-media">
          <CameraView channel={channel.id} compact />
        </div>
      ) : (
        // Fills the tile beside a live camera; stays short when stacked on a phone.
        <div className="flex min-h-32 flex-1 items-center justify-center rounded-lg bg-muted px-6 py-6 text-center text-sm text-muted-foreground">
          <span>
            No picture from {channel.label}.{' '}
            <Link to="/machine/cameras" className="font-medium text-primary-text hover:underline">
              Choose its camera under Machine › Cameras
            </Link>
          </span>
        </div>
      )}
      <p className="text-sm text-muted-foreground">
        {live ? (
          <>
            <span className="font-mono text-foreground">{summary.pieces}</span>{' '}
            {summary.pieces === 1 ? 'piece' : 'pieces'}
            {zones.length > 0 && ` · in ${zones.join(', ')}`}
            {summary.falls > 0 && ` · ${summary.falls} pushed off this run`}
            {summary.backed_up && (
              <span className="text-warning"> · backed up, so the channel before waits</span>
            )}
          </>
        ) : why ? (
          <>
            <span className="text-warning">{why}</span>{' '}
            <Link
              to={`/machine/cameras/${channel.id}`}
              className="font-medium text-primary-text hover:underline"
            >
              Fix under Machine › Cameras
            </Link>
          </>
        ) : (
          'Waiting for frames'
        )}
      </p>
    </div>
  );
}
