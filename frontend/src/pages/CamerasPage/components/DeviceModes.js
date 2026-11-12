import { useState } from 'react';

import { Button } from '@/components/ui/button';
import { listSorterCameraModes } from '@/helpers/api';

/**
 * The sizes a camera offers, checked on request, against the size it opens at (its
 * channel's own, when `own`, else every camera's).
 */
export default function DeviceModes({ deviceId, width, height, own, disabled }) {
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
    <div className="flex flex-col items-start gap-1">
      <Button variant="ghost" size="sm" disabled={disabled || state.loading} onClick={check}>
        {state.loading ? 'Asking the camera…' : 'Check its sizes'}
      </Button>
      {state.error && <p className="text-xs text-muted-foreground">{state.error}</p>}
      {state.sizes?.length > 0 && (
        <p className={fits ? 'text-xs text-muted-foreground' : 'text-xs text-warning'}>
          {fits
            ? `Takes ${width}×${height}. `
            : own
              ? `Doesn’t offer ${width}×${height}, the size set for its channel, so it won’t open. Change it on the channel’s tab. `
              : `Doesn’t offer ${width}×${height}, the size every camera uses, so it won’t open. Change the size on any channel tab. `}
          Offers: {state.sizes.map((s) => `${s.key} (${s.fps} fps)`).join(', ')}
        </p>
      )}
    </div>
  );
}
