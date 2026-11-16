import { Expand } from 'lucide-react';
import { useState } from 'react';

import { Badge } from '@/components/ui/badge';
import { photoURL } from '@/helpers/api';
import { cn } from '@/lib/utils';

import { formatDuration, pieceBin, pieceName, WATCH_CAMERAS } from '../CollectionPage.logic';
import CameraViewer from './CameraViewer';
import { usePieceButton } from './PieceDialog';
import { Pic } from './PieceTile';

function Stat({ label, value, detail }) {
  return (
    <div className="rounded-xl border border-border bg-card px-4 py-3">
      <p className="text-sm text-muted-foreground">{label}</p>
      <p className="font-mono text-2xl font-medium">{value}</p>
      {detail && <p className="truncate text-xs text-muted-foreground">{detail}</p>}
    </div>
  );
}

/** What the machine is doing, in a word, with why it's stopped when it is. */
function MachineStatus({ state }) {
  if (state.incident) {
    return <Badge variant="warning">Stopped: {state.incident}. Someone’s on it.</Badge>;
  }
  if (state.state === 'paused') return <Badge variant="warning">Paused</Badge>;
  return <Badge variant="running">Sorting</Badge>;
}

/** One camera: its newest picture (click to see it big), and what that part of the
 * machine is doing. */
function Camera({ camera, frameRef, doing, count, onOpen }) {
  return (
    <figure
      aria-label={`Camera over ${camera.label}`}
      className="flex flex-col overflow-hidden rounded-xl border border-border bg-card"
    >
      <button
        type="button"
        onClick={onOpen}
        className="group relative aspect-video cursor-zoom-in bg-media"
        aria-label={`See ${camera.label} big`}
      >
        <img
          ref={frameRef(camera.id, 'tile')}
          alt=""
          className="absolute inset-0 size-full object-contain"
        />
        <span className="absolute top-2 left-2 rounded-md bg-black/60 px-2 py-0.5 font-mono text-xs text-white">
          {camera.label}
        </span>
        <span className="absolute top-2 right-2 rounded-md bg-black/60 p-1 text-white opacity-70 group-hover:opacity-100">
          <Expand className="size-4" />
        </span>
      </button>
      <figcaption className="flex flex-col gap-0.5 px-3 py-2">
        <span className="text-sm font-medium">
          {camera.name}
          {count > 0 && (
            <span className="font-normal text-muted-foreground">
              {' '}
              · {count} {count === 1 ? 'piece' : 'pieces'}
            </span>
          )}
        </span>
        <span className="truncate text-xs text-muted-foreground" title={doing}>
          {doing ? doing.charAt(0).toUpperCase() + doing.slice(1) : ' '}
        </span>
      </figcaption>
    </figure>
  );
}

/** A piece found lately: our photo once it's arrived, else the official picture. */
function FoundPiece({ piece }) {
  const unknown = !piece.part_id;
  const button = usePieceButton();
  return (
    <li
      className="flex cursor-pointer items-center gap-3 rounded-xl border border-border bg-card p-2 hover:border-border-strong focus-visible:outline-2 focus-visible:outline-ring"
      {...button(piece)}
    >
      <Pic src={photoURL(piece.id)} alt="" className="size-14 shrink-0" />
      <Pic
        src={piece.part_img_url}
        alt=""
        className={cn('size-14 shrink-0 bg-white', unknown && 'hidden sm:block')}
      />
      <div className="min-w-0">
        <p className="truncate text-sm font-medium" title={pieceName(piece)}>
          {pieceName(piece)}
        </p>
        <p className="truncate text-xs text-muted-foreground">
          {unknown
            ? 'Not named yet'
            : [piece.part_id, piece.color_name].filter(Boolean).join(' · ')}
        </p>
        {pieceBin(piece) && (
          <p className="truncate text-xs text-muted-foreground">
            {pieceBin(piece)}
            {piece.category_name && ` · ${piece.category_name}`}
          </p>
        )}
      </div>
    </li>
  );
}

/** On the classification plate now: being photographed, then named. */
function Aboard({ pieces }) {
  const button = usePieceButton();
  if (!pieces.length) {
    return <p className="text-sm text-muted-foreground">Nothing on the plate this moment.</p>;
  }
  return (
    <ul className="flex flex-wrap gap-2">
      {pieces.map((p) => (
        <li key={p.id} className="cursor-pointer rounded-md" {...button(p)}>
          <Badge variant={p.part_id ? 'running' : 'default'}>
            {p.part_id
              ? `${pieceName(p)}${p.color_name ? ` · ${p.color_name}` : ''}`
              : 'Looking at it…'}
          </Badge>
        </li>
      ))}
    </ul>
  );
}

/**
 * The Watch tab: the cameras, the pieces going through, what's been found and the
 * run's numbers, live while the collection is sorting.
 */
export default function WatchView({ watch, live }) {
  const { status, state, feed } = watch;
  const [opened, setOpened] = useState(null); // the camera seen big
  if (!live || status === 'ended') {
    return (
      <section className="rounded-xl border border-border bg-card p-5">
        <p className="font-medium">Not sorting right now</p>
        <p className="text-sm text-muted-foreground">
          This comes alive by itself when your LEGO is being sorted: you’ll see the cameras, each
          piece as it’s found, and how it’s going.
        </p>
      </section>
    );
  }
  const run = state?.run;
  const onMachine =
    (state?.on_channel?.c2 ?? 0) + (state?.on_channel?.c3 ?? 0) + (state?.aboard?.length ?? 0);
  return (
    <section className="flex flex-col gap-4" aria-label="Watching the sort">
      <div className="flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
        {state ? <MachineStatus state={state} /> : <Badge>Connecting…</Badge>}
        <span>
          Live from the sorter. Pieces go from C2 to C3, then C4 photographs and names each one.
        </span>
      </div>

      {run && (
        <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
          <Stat label="Sorted this run" value={run.sorted.toLocaleString()} />
          <Stat
            label="Per minute"
            value={run.sorting_ms >= 60000 ? run.per_minute.toFixed(1) : '—'}
            detail={run.sorting_ms >= 60000 ? undefined : 'After the first minute'}
          />
          <Stat
            label="Sorting for"
            value={formatDuration(run.sorting_ms)}
            detail={
              run.blocked_ms >= 1000 ? `Plus ${formatDuration(run.blocked_ms)} stopped` : undefined
            }
          />
          <Stat
            label="On the machine"
            value={onMachine.toLocaleString()}
            detail="Pieces on their way"
          />
        </div>
      )}

      <div className="grid gap-3 lg:grid-cols-3">
        {WATCH_CAMERAS.map((cam) => (
          <Camera
            key={cam.id}
            camera={cam}
            frameRef={watch.frameRef}
            doing={state?.doing?.[cam.id]}
            count={
              cam.id === 'c4' ? (state?.aboard?.length ?? 0) : (state?.on_channel?.[cam.id] ?? 0)
            }
            onOpen={() => setOpened(cam)}
          />
        ))}
      </div>
      <CameraViewer
        camera={opened}
        cameras={WATCH_CAMERAS}
        watch={watch}
        onSwitch={setOpened}
        onClose={() => setOpened(null)}
      />

      <div className="flex flex-col gap-2">
        <h2 className="font-display text-lg font-semibold">On the plate now</h2>
        <Aboard pieces={state?.aboard ?? []} />
      </div>

      <div className="flex flex-col gap-2">
        <h2 className="font-display text-lg font-semibold">Just found</h2>
        {feed.length === 0 ? (
          <p className="text-sm text-muted-foreground">Each piece shows here as it’s sorted.</p>
        ) : (
          <ul className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
            {feed.map((p) => (
              <FoundPiece key={p.id} piece={p} />
            ))}
          </ul>
        )}
      </div>
    </section>
  );
}
