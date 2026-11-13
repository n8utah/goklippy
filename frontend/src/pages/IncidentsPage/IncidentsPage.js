import { ChevronRight, RefreshCw, Trash2 } from 'lucide-react';
import { useEffect, useState } from 'react';
import { useSearchParams } from 'react-router';

import ConfirmClear from '@/components/ConfirmClear/ConfirmClear';
import { useJourneyModal } from '@/components/JourneyModal/JourneyModal.logic';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { formatBytes } from '@/helpers/format';
import { cn } from '@/lib/utils';

import IncidentCameras from './components/IncidentCameras';
import { useIncidentsPageLogic } from './IncidentsPage.logic';

const when = (t) => (t ? new Date(t).toLocaleString() : '—');

/** Jumps to an incident by its number. */
function GoTo({ onGo }) {
  const [n, setN] = useState('');
  return (
    <form
      className="flex items-center gap-1"
      onSubmit={(e) => {
        e.preventDefault();
        if (n.trim()) onGo(n.trim().replace(/^#/, ''));
      }}
    >
      <label htmlFor="incident-go" className="text-sm text-muted-foreground">
        Incident #
      </label>
      <Input
        id="incident-go"
        className="h-9 w-20"
        inputMode="numeric"
        value={n}
        onChange={(e) => setN(e.target.value)}
      />
      <Button type="submit" variant="ghost" size="sm" disabled={!n.trim()}>
        Go
      </Button>
    </form>
  );
}

function Status({ incident }) {
  return (
    <Badge variant={incident.resolved_at ? 'idle' : 'warning'} className="self-start">
      {incident.resolved_at
        ? `${incident.resolution || 'Resolved'} · ${when(incident.resolved_at)}`
        : 'Open'}
    </Badge>
  );
}

function Summary({ incident, onOpenPiece }) {
  return (
    <span className="min-w-0 flex-1 text-sm">
      <span className="font-medium">{incident.title}</span>
      {/* A phone's list keeps it short; the modal has it all. */}
      <span className="line-clamp-2 block text-muted-foreground md:line-clamp-none">
        {incident.message}
      </span>
      {incident.piece_id && (
        <button
          type="button"
          className="text-sm font-medium text-primary-text hover:underline"
          onClick={(e) => {
            e.stopPropagation();
            onOpenPiece(incident.piece_id);
          }}
        >
          Its piece
        </button>
      )}
    </span>
  );
}

/** One incident in full, for a phone: what happened and every channel then. */
function IncidentModal({ incident, onClose, onOpenPiece }) {
  return (
    <Dialog open={Boolean(incident)} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-h-[92dvh] overflow-y-auto">
        {incident && (
          <>
            <DialogHeader className="text-left">
              <DialogTitle>
                <span className="font-mono">#{incident.id}</span> {incident.title}
              </DialogTitle>
              <DialogDescription>{when(incident.opened_at)}</DialogDescription>
            </DialogHeader>
            <p className="text-sm text-muted-foreground">{incident.message}</p>
            <div className="flex flex-wrap items-center gap-3">
              <Status incident={incident} />
              {incident.piece_id && (
                <Button
                  variant="link"
                  size="sm"
                  className="h-auto p-0"
                  onClick={() => onOpenPiece(incident.piece_id)}
                >
                  Its piece
                </Button>
              )}
            </div>
            {incident.cameras?.length ? (
              <IncidentCameras incident={incident} />
            ) : (
              <p className="text-sm text-muted-foreground">
                No camera pictures kept for this one: it’s from before they were kept, or they were
                cleared.
              </p>
            )}
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}

/**
 * Run › Incidents (/run/incidents): every problem the sort met, by number, each with
 * every channel's picture and state from when it opened, a red box where the problem
 * is when that's known. On a wide screen the cameras show in the list; on a phone an
 * incident opens in a modal. ?incident=12 finds one.
 */
const IncidentsPage = () => {
  const [params, setParams] = useSearchParams();
  const journey = useJourneyModal();
  const logic = useIncidentsPageLogic();
  const [confirm, setConfirm] = useState(null);
  const { pictures, incidents, wide, setOpenId } = logic;
  const focusId = params.get('incident') ?? '';
  // Every incident goes but the one the machine is stopped for now.
  const removable = incidents.filter((i) => i.id !== logic.heldIncidentId).length;
  const found = incidents.some((i) => String(i.id) === focusId);

  // A linked incident is scrolled to once it's listed; on a phone it opens.
  useEffect(() => {
    if (!focusId || !found) return;
    document
      .getElementById(`incident-${focusId}`)
      ?.scrollIntoView({ block: 'start', behavior: 'smooth' });
    if (!wide) setOpenId(Number(focusId));
  }, [focusId, found, wide, setOpenId]);

  return (
    <div className="ds flex flex-col gap-4">
      {logic.error && <p className="text-sm text-destructive">{logic.error}</p>}
      {logic.message && <p className="text-sm text-muted-foreground">{logic.message}</p>}
      <section className="rounded-xl border border-border bg-card">
        <div className="flex flex-wrap items-center gap-1 border-b border-border px-4 py-2.5 sm:px-5">
          <span className="mr-auto text-sm text-muted-foreground">
            {incidents.length} incident{incidents.length === 1 ? '' : 's'}
            {focusId && !found && logic.loaded && (
              <span className="text-warning"> · there’s no incident #{focusId}</span>
            )}
          </span>
          <GoTo onGo={(id) => setParams({ incident: id })} />
          <Button variant="ghost" size="sm" onClick={logic.refresh}>
            <RefreshCw /> Refresh
          </Button>
          <Button
            variant="ghost"
            size="sm"
            className="text-destructive"
            disabled={logic.busy || !pictures?.files}
            onClick={() =>
              setConfirm({
                title: 'Clear the incident pictures?',
                text: `This removes the ${(pictures?.files ?? 0).toLocaleString()} camera pictures (${formatBytes(pictures?.bytes)}) and the channel states kept with the incidents. The incidents stay. New incidents keep theirs as usual.`,
                confirm: 'Clear pictures',
                action: logic.clearPictures,
              })
            }
          >
            <Trash2 /> Clear pictures{pictures?.files ? ` (${formatBytes(pictures.bytes)})` : ''}
          </Button>
          <Button
            variant="ghost"
            size="sm"
            className="text-destructive"
            disabled={logic.busy || removable === 0}
            onClick={() =>
              setConfirm({
                title: 'Clear the incidents?',
                text: `This removes ${removable} incident${removable === 1 ? '' : 's'} with their pictures.${logic.heldIncidentId ? ' The one the machine is stopped for now stays.' : ''}`,
                confirm: 'Clear incidents',
                action: logic.clearIncidents,
              })
            }
          >
            <Trash2 /> Clear incidents
          </Button>
        </div>

        {incidents.length === 0 ? (
          <p className="px-5 py-6 text-sm text-muted-foreground">
            {logic.loaded
              ? 'No incidents. Problems during a sort that needed a person show here, with every camera’s picture from that moment.'
              : 'Loading…'}
          </p>
        ) : (
          <ul className="divide-y divide-border">
            {incidents.map((i) => (
              <li
                key={i.id}
                id={`incident-${i.id}`}
                // On a phone the whole incident opens in a modal.
                {...(!wide && {
                  role: 'button',
                  tabIndex: 0,
                  onClick: () => setOpenId(i.id),
                  onKeyDown: (e) => (e.key === 'Enter' || e.key === ' ') && setOpenId(i.id),
                })}
                className={cn(
                  'flex scroll-mt-20 flex-col gap-3 px-4 py-4 sm:px-5',
                  !wide && 'cursor-pointer active:bg-accent',
                  String(i.id) === focusId && 'bg-primary/5 ring-2 ring-primary ring-inset',
                )}
              >
                <div className="flex gap-3 sm:gap-4">
                  <span className="flex w-24 shrink-0 flex-col gap-0.5 sm:w-40">
                    <span className="font-mono text-base font-semibold">#{i.id}</span>
                    <span className="font-mono text-xs text-muted-foreground">
                      {when(i.opened_at)}
                    </span>
                  </span>
                  <div className="flex min-w-0 flex-1 flex-col gap-2 sm:flex-row sm:items-start sm:gap-4">
                    <Summary incident={i} onOpenPiece={journey.open} />
                    <Status incident={i} />
                  </div>
                  {!wide && (
                    <ChevronRight
                      className="size-5 shrink-0 self-center text-muted-foreground"
                      aria-hidden
                    />
                  )}
                </div>
                {wide && <IncidentCameras incident={i} />}
              </li>
            ))}
          </ul>
        )}
      </section>

      <IncidentModal
        incident={wide ? null : logic.open}
        onClose={() => setOpenId(null)}
        onOpenPiece={journey.open}
      />
      <ConfirmClear what={confirm} onClose={() => setConfirm(null)} />
    </div>
  );
};

export default IncidentsPage;
