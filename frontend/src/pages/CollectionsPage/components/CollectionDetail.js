import {
  Archive,
  ArchiveRestore,
  ChevronLeft,
  ExternalLink,
  Pencil,
  SlidersHorizontal,
  Trash2,
} from 'lucide-react';
import { useState } from 'react';
import { Link, useNavigate } from 'react-router';

import ConfirmClear from '@/components/ConfirmClear/ConfirmClear';
import { useJourneyModal } from '@/components/JourneyModal/JourneyModal.logic';
import PieceCard from '@/components/PieceCard/PieceCard';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';

import { useCollectionDetail, useSharing } from '../CollectionsPage.logic';
import ChangeProfileDialog from './ChangeProfileDialog';
import { SharingStatus } from './SharingPanel';

const when = (t) => (t ? new Date(t).toLocaleString() : '—');

function Stat({ label, value, detail }) {
  return (
    <div className="rounded-xl border border-border bg-card px-4 py-3">
      <p className="text-sm text-muted-foreground">{label}</p>
      <p className="font-mono text-2xl font-medium">{value}</p>
      {detail && <p className="truncate text-xs text-muted-foreground">{detail}</p>}
    </div>
  );
}

function NameEditor({ name, busy, onSave }) {
  const [editing, setEditing] = useState(false);
  const [value, setValue] = useState(name);
  if (!editing) {
    return (
      <div className="flex min-w-0 items-center gap-2">
        <h2 className="truncate font-display text-2xl font-semibold">{name}</h2>
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label="Rename"
          onClick={() => {
            setValue(name);
            setEditing(true);
          }}
        >
          <Pencil />
        </Button>
      </div>
    );
  }
  return (
    <form
      className="flex items-center gap-2"
      onSubmit={(e) => {
        e.preventDefault();
        if (value.trim() && value.trim() !== name) onSave(value.trim());
        setEditing(false);
      }}
    >
      <Input
        autoFocus
        aria-label="Collection name"
        value={value}
        onChange={(e) => setValue(e.target.value)}
        className="w-64"
      />
      <Button type="submit" size="sm" disabled={busy || !value.trim()}>
        Save
      </Button>
      <Button type="button" variant="ghost" size="sm" onClick={() => setEditing(false)}>
        Cancel
      </Button>
    </form>
  );
}

/** The bins the collection is being sorted into: its plan, i.e. the groups it comes back in. */
function PlanSection({ plan, canChange, onChange }) {
  const bins = plan?.bins ?? [];
  return (
    <section className="flex flex-col gap-3">
      <div className="flex flex-wrap items-end justify-between gap-2">
        <div>
          <h3 className="font-display text-lg font-semibold">Sorting plan</h3>
          <p className="text-sm text-muted-foreground">
            {plan?.profile ? (
              <>
                Sorted by <strong className="text-foreground">{plan.profile.name}</strong>. Each bin
                is a group it comes back in; counts are since its bins started afresh.
              </>
            ) : (
              'No profile yet: pick the one it sorts by before sorting into it.'
            )}
          </p>
        </div>
        <Button
          variant="outline"
          disabled={!canChange}
          title={canChange ? undefined : 'Stop the run first'}
          onClick={onChange}
        >
          <SlidersHorizontal /> {plan?.profile ? 'Change profile' : 'Pick a profile'}
        </Button>
      </div>
      <div className="rounded-xl border border-border bg-card">
        {bins.length === 0 ? (
          <p className="px-5 py-6 text-sm text-muted-foreground">
            No bins given yet. Each category gets a bin the first time a piece of it arrives.
          </p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="pl-5">Bin</TableHead>
                <TableHead>Takes</TableHead>
                <TableHead className="pr-5 text-right">Pieces</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {bins.map((b) => (
                <TableRow key={`${b.layer}-${b.bin}`}>
                  <TableCell className="pl-5 font-mono whitespace-nowrap">
                    Layer {b.layer} · bin {b.bin}
                  </TableCell>
                  <TableCell>
                    {b.categories.length ? (
                      b.categories.map((c) => c.name || c.id).join(', ')
                    ) : (
                      <span className="text-muted-foreground">Nothing now</span>
                    )}
                  </TableCell>
                  <TableCell className="pr-5 text-right font-mono">{b.pieces}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
        {plan?.bucket > 0 && (
          <p className="border-t border-border px-5 py-3 text-sm text-muted-foreground">
            <span className="font-mono text-foreground">{plan.bucket}</span> went to the bucket
            under the tower (no bin, too big, or the chute left out).
          </p>
        )}
      </div>
    </section>
  );
}

/** One collection (/run/collections/<id>). */
export default function CollectionDetail({ id }) {
  const logic = useCollectionDetail(id);
  const sharing = useSharing();
  const journey = useJourneyModal();
  const navigate = useNavigate();
  const [changing, setChanging] = useState(false);
  const col = logic.collection;

  return (
    <div className="ds flex flex-col gap-6">
      <Link
        to="/run/collections"
        className="flex items-center gap-1 text-sm font-medium text-primary-text hover:underline"
      >
        <ChevronLeft className="size-4" /> Collections
      </Link>

      {logic.error && <p className="text-sm text-destructive">{logic.error}</p>}

      {col && (
        <>
          <section className="flex flex-col gap-3 rounded-xl border border-border bg-card p-5 sm:flex-row sm:items-center sm:justify-between">
            <div className="min-w-0">
              <NameEditor key={col.name} name={col.name} busy={logic.busy} onSave={logic.rename} />
              <p className="flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
                {logic.sortingHere && <Badge variant="running">Sorting now</Badge>}
                {!logic.sortingHere && col.current && <Badge variant="running">Current</Badge>}
                {col.archived_at && <Badge variant="idle">Archived</Badge>}
                Made {when(col.created_at)}
              </p>
            </div>
            <div className="flex shrink-0 flex-wrap gap-2">
              <Button variant="outline" asChild>
                <Link to={`/pieces/parts?collection=${col.id}`}>Its parts</Link>
              </Button>
              <Button variant="outline" asChild>
                <Link to={`/pieces/all?collection=${col.id}`}>Every piece</Link>
              </Button>
              <Button
                variant="outline"
                disabled={logic.busy || logic.sortingHere}
                onClick={() => logic.setArchived(!col.archived_at)}
              >
                {col.archived_at ? <ArchiveRestore /> : <Archive />}
                {col.archived_at ? 'Unarchive' : 'Archive'}
              </Button>
            </div>
          </section>

          <section className="flex flex-col gap-2 rounded-xl border border-border bg-card px-5 py-4 sm:flex-row sm:items-center sm:justify-between">
            <label className="flex items-center gap-3">
              <Switch
                checked={col.shared}
                disabled={logic.busy || (!col.shared && !sharing.ready)}
                onCheckedChange={logic.setShared}
              />
              <span>
                <span className="block font-medium">
                  Share to {sharing.siteHost || 'the sort viewer site'}
                </span>
                <span className="block text-sm text-muted-foreground">
                  {!sharing.ready && !col.shared ? (
                    <>
                      Set up sharing on{' '}
                      <Link to="/run/collections" className="text-primary-text hover:underline">
                        Run › Collections
                      </Link>{' '}
                      first.
                    </>
                  ) : col.shared ? (
                    'Sent as it sorts, so its owner can watch. Viewers are given it on the site.'
                  ) : (
                    'Turning it on sends everything sorted into it so far, then keeps up.'
                  )}
                </span>
              </span>
            </label>
            {col.shared && (
              <div className="flex flex-col gap-1 text-sm sm:items-end">
                <SharingStatus status={sharing.status} />
                {sharing.status?.url && (
                  <a
                    href={`${sharing.status.url}/c/${col.id}`}
                    target="_blank"
                    rel="noreferrer"
                    className="flex items-center gap-1 font-medium text-primary-text hover:underline"
                  >
                    Open on {sharing.siteHost} <ExternalLink className="size-3.5" />
                  </a>
                )}
              </div>
            )}
          </section>

          <section aria-label="Totals" className="grid grid-cols-2 gap-3 lg:grid-cols-4">
            <Stat
              label="Pieces"
              value={col.pieces.toLocaleString()}
              detail={`Over ${col.runs} run${col.runs === 1 ? '' : 's'}`}
            />
            <Stat label="Identified" value={col.identified.toLocaleString()} />
            <Stat label="Unknown" value={col.unknown.toLocaleString()} detail="No part found" />
            <Stat
              label="Last sorted"
              value={col.last_sorted_at ? new Date(col.last_sorted_at).toLocaleDateString() : '—'}
            />
          </section>

          <PlanSection
            plan={logic.plan}
            canChange={!logic.busy && !logic.sortingHere}
            onChange={() => setChanging(true)}
          />
          {changing && (
            <ChangeProfileDialog
              open
              collection={col}
              current={col.current}
              busy={logic.busy}
              onCancel={() => setChanging(false)}
              onSave={(profileId, bins) => {
                setChanging(false);
                logic.changeProfile(profileId, bins);
              }}
            />
          )}

          <section className="flex flex-col gap-3">
            <div className="flex flex-wrap items-end justify-between gap-2">
              <div>
                <h3 className="font-display text-lg font-semibold">Unknowns</h3>
                <p className="text-sm text-muted-foreground">
                  Pieces with no part found. Before feeding the unknowns through again, clear them
                  so they aren’t counted twice.
                </p>
              </div>
              <Button
                variant="outline"
                disabled={logic.busy || logic.sortingHere || logic.unknowns.total === 0}
                title={logic.sortingHere ? 'Stop the run first' : undefined}
                onClick={logic.askClearUnknowns}
              >
                <Trash2 /> Clear unknowns
              </Button>
            </div>
            {logic.unknowns.total === 0 ? (
              <p className="text-sm text-muted-foreground">No unknowns.</p>
            ) : (
              <>
                <div className="flex flex-wrap gap-3">
                  {logic.unknowns.pieces.map((p) => (
                    <PieceCard
                      key={p.id}
                      piece={p}
                      layout="tile"
                      onOpen={() => journey.open(p.id)}
                    />
                  ))}
                </div>
                {logic.unknowns.total > logic.unknowns.pieces.length && (
                  <p className="text-sm text-muted-foreground">
                    Showing {logic.unknowns.pieces.length} of {logic.unknowns.total}.
                  </p>
                )}
              </>
            )}
          </section>

          <section className="flex flex-col gap-3">
            <h3 className="font-display text-lg font-semibold">Runs</h3>
            <div className="rounded-xl border border-border bg-card">
              {logic.runs.length === 0 ? (
                <p className="px-5 py-6 text-sm text-muted-foreground">
                  No runs yet. Pick it on Run › Sort and start.
                </p>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead className="pl-5">Run</TableHead>
                      <TableHead>Profile</TableHead>
                      <TableHead className="hidden sm:table-cell">Started</TableHead>
                      <TableHead className="text-right">Pieces</TableHead>
                      <TableHead className="pr-5 text-right">Sorted</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {logic.runs.map((r) => (
                      <TableRow
                        key={r.id}
                        className="cursor-pointer"
                        onClick={() => navigate(`/pieces/all?run=${r.id}`)}
                      >
                        <TableCell className="pl-5 font-mono">{r.id}</TableCell>
                        <TableCell>{r.profile_name || '—'}</TableCell>
                        <TableCell className="hidden sm:table-cell">{when(r.started_at)}</TableCell>
                        <TableCell className="text-right font-mono">{r.pieces}</TableCell>
                        <TableCell className="pr-5 text-right font-mono">
                          {r.ended_at ? r.distributed : <Badge variant="running">Running</Badge>}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </div>
          </section>
        </>
      )}

      <ConfirmClear what={logic.confirm} onClose={logic.closeConfirm} />
    </div>
  );
}
