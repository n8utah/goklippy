import { ImageOff, Trash2 } from 'lucide-react';
import { Link, Navigate, NavLink, useParams } from 'react-router';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { cn } from '@/lib/utils';

import { ADMIN_TABS, useAdminPage, useSetData } from './AdminPage.logic';
import KeysPanel from './components/KeysPanel';
import SetDataPanel from './components/SetDataPanel';
import ViewersPanel from './components/ViewersPanel';

function Confirm({ what, onClose }) {
  return (
    <Dialog open={Boolean(what)} onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{what?.title}</DialogTitle>
          <DialogDescription>{what?.text}</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <DialogClose asChild>
            <Button variant="outline">Cancel</Button>
          </DialogClose>
          <Button
            variant="destructive"
            onClick={() => {
              what.action();
              onClose();
            }}
          >
            {what?.confirm}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function CollectionsPanel({ logic }) {
  const viewersOf = (id) => (logic.viewers ?? []).filter((v) => v.collections.includes(id)).length;
  return (
    <section className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <p className="text-sm text-muted-foreground">
          Collections the sorter has shared. Give viewers access under Viewers.
        </p>
        <Button variant="outline" onClick={() => logic.askClearPhotos(null)}>
          <ImageOff /> Clear every photo
        </Button>
      </div>
      {logic.collections.length === 0 && (
        <p className="rounded-xl border border-border bg-card p-5 text-sm text-muted-foreground">
          Nothing shared yet.
        </p>
      )}
      <ul className="flex flex-col gap-2">
        {logic.collections.map((c) => (
          <li
            key={c.id}
            className="flex flex-wrap items-center gap-3 rounded-xl border border-border bg-card px-4 py-3"
          >
            <div className="min-w-0 flex-1">
              <p className="flex items-center gap-2 font-medium">
                <Link to={`/c/${c.id}`} className="truncate hover:underline">
                  {c.name}
                </Link>
                {c.sorting && <Badge variant="running">Sorting now</Badge>}
              </p>
              <p className="text-sm text-muted-foreground">
                {c.pieces.toLocaleString()} pieces · {c.unknown} unknown · {viewersOf(c.id)} viewer
                {viewersOf(c.id) === 1 ? '' : 's'}
              </p>
            </div>
            <Button variant="outline" size="sm" onClick={() => logic.askClearPhotos(c)}>
              <ImageOff /> Clear photos
            </Button>
            <Button
              variant="ghost"
              size="icon-sm"
              aria-label={`Delete ${c.name}`}
              onClick={() => logic.askDeleteCollection(c)}
            >
              <Trash2 />
            </Button>
          </li>
        ))}
      </ul>
    </section>
  );
}

/** Admin (/admin/<tab>): viewers, collections, the sorter's keys and the set data. */
const AdminPage = () => {
  const { tab = 'viewers' } = useParams();
  const logic = useAdminPage();
  const setData = useSetData(tab === 'sets');
  if (!ADMIN_TABS.some((t) => t.id === tab)) return <Navigate to="/admin" replace />;

  return (
    <div className="flex flex-col gap-5">
      <h1 className="font-display text-2xl font-semibold">Admin</h1>
      <nav className="flex gap-1 overflow-x-auto border-b border-border" aria-label="Admin">
        {ADMIN_TABS.map((t) => (
          <NavLink
            key={t.id}
            to={t.id === 'viewers' ? '/admin' : `/admin/${t.id}`}
            end
            className={cn(
              '-mb-px border-b-2 px-3 py-2 text-sm font-medium whitespace-nowrap',
              t.id === tab
                ? 'border-primary text-foreground'
                : 'border-transparent text-muted-foreground hover:text-foreground',
            )}
          >
            {t.label}
          </NavLink>
        ))}
      </nav>
      {tab === 'viewers' && <ViewersPanel logic={logic} />}
      {tab === 'collections' && <CollectionsPanel logic={logic} />}
      {tab === 'keys' && <KeysPanel logic={logic} />}
      {tab === 'sets' && <SetDataPanel setData={setData} />}
      <Confirm what={logic.confirm} onClose={logic.closeConfirm} />
    </div>
  );
};

export default AdminPage;
