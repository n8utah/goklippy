import { PackageSearch } from 'lucide-react';
import { useState } from 'react';

import { cn } from '@/lib/utils';

/**
 * A part or piece's picture: the camera photo or Brickognize's ("official") one, as
 * `mode` asks, falling back to the other when it's missing or won't load. Camera
 * photos fill the square; official pictures sit on white, as they're drawn.
 */
export default function Picture({ camera, official, mode, alt = '', className }) {
  const order = mode === 'camera' ? [camera, official] : [official, camera];
  const [failed, setFailed] = useState(() => new Set());
  const src = order.find((u) => u && !failed.has(u));
  const isCamera = src && src === camera;
  return (
    <span
      className={cn(
        'flex shrink-0 items-center justify-center overflow-hidden rounded-md border border-border',
        // Behind a photo, so it's the same in both themes.
        isCamera ? 'bg-media' : 'bg-white',
        className,
      )}
    >
      {src ? (
        <img
          key={src}
          src={src}
          alt={alt}
          loading="lazy"
          referrerPolicy="no-referrer"
          className={cn('size-full', isCamera ? 'object-cover' : 'object-contain p-1')}
          onError={() => setFailed((s) => new Set(s).add(src))}
        />
      ) : (
        <PackageSearch className="size-1/3 text-muted-foreground" aria-hidden />
      )}
    </span>
  );
}
