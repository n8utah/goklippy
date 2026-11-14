import { ChevronLeft } from 'lucide-react';
import { Link, Navigate, NavLink, useParams } from 'react-router';

import { Badge } from '@/components/ui/badge';
import { useSession } from '@/helpers/session';
import { cn } from '@/lib/utils';

import { TABS, useCollectionPage, useSets } from './CollectionPage.logic';
import PartsView from './components/PartsView';
import PieceTile from './components/PieceTile';
import PlanView from './components/PlanView';
import SetsView from './components/SetsView';

function Stat({ label, value }) {
  return (
    <div className="rounded-xl border border-border bg-card px-4 py-3">
      <p className="text-sm text-muted-foreground">{label}</p>
      <p className="font-mono text-2xl font-medium">{value}</p>
    </div>
  );
}

function PieceGrid({ pieces, freshId, empty }) {
  if (!pieces) return <p className="text-sm text-muted-foreground">Loading…</p>;
  if (pieces.length === 0) return <p className="text-sm text-muted-foreground">{empty}</p>;
  return (
    <ul className="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
      {pieces.map((p) => (
        <li key={p.id}>
          <PieceTile piece={p} fresh={p.id === freshId} />
        </li>
      ))}
    </ul>
  );
}

/** One collection (/c/<id>/<tab>): live as it sorts, its plan, its parts and unknowns. */
const CollectionPage = () => {
  const { id, tab = 'live' } = useParams();
  const { me } = useSession();
  const logic = useCollectionPage(id, tab);
  const sets = useSets(id, tab === 'sets');
  const col = logic.collection;

  if (!TABS.some((t) => t.id === tab)) return <Navigate to={`/c/${id}`} replace />;
  if (logic.error === 'notfound') {
    return (
      <p className="rounded-xl border border-border bg-card p-5 text-sm text-muted-foreground">
        That collection isn’t here, or isn’t shared with you.{' '}
        <Link to="/" className="font-medium text-primary-text underline">
          Your collections
        </Link>
      </p>
    );
  }

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-col gap-1">
        {me?.admin && (
          <Link
            to="/"
            className="flex items-center gap-1 text-sm font-medium text-primary-text hover:underline"
          >
            <ChevronLeft className="size-4" /> Collections
          </Link>
        )}
        <h1 className="flex flex-wrap items-center gap-2 font-display text-2xl font-semibold">
          {col?.name ?? '…'}
          {col?.sorting && <Badge variant="running">Sorting now</Badge>}
        </h1>
        {logic.error && <p className="text-sm text-destructive">{logic.error}</p>}
      </div>

      {col && (
        <section aria-label="Totals" className="grid grid-cols-3 gap-3">
          <Stat label="Pieces" value={col.pieces.toLocaleString()} />
          <Stat label="Identified" value={col.identified.toLocaleString()} />
          <Stat label="Unknown" value={col.unknown.toLocaleString()} />
        </section>
      )}

      <nav className="flex gap-1 overflow-x-auto border-b border-border" aria-label="Views">
        {TABS.map((t) => (
          <NavLink
            key={t.id}
            to={t.id === 'live' ? `/c/${id}` : `/c/${id}/${t.id}`}
            end
            className={({ isActive }) =>
              cn(
                '-mb-px border-b-2 px-3 py-2 text-sm font-medium whitespace-nowrap',
                isActive || t.id === tab
                  ? 'border-primary text-foreground'
                  : 'border-transparent text-muted-foreground hover:text-foreground',
              )
            }
          >
            {t.label}
            {t.id === 'unknowns' && col?.unknown ? ` (${col.unknown})` : ''}
          </NavLink>
        ))}
      </nav>

      {tab === 'live' && (
        <section className="flex flex-col gap-3">
          <p className="text-sm text-muted-foreground">
            {col?.sorting
              ? 'The newest pieces, as they’re sorted. Our photo is on the left, the official picture on the right.'
              : 'The newest pieces. This updates by itself while sorting is on.'}
          </p>
          <PieceGrid
            pieces={logic.live}
            freshId={logic.connected ? logic.live[0]?.id : undefined}
            empty="No pieces yet."
          />
        </section>
      )}
      {tab === 'plan' && <PlanView view={logic.plan} />}
      {tab === 'parts' && <PartsView parts={logic.parts} />}
      {tab === 'sets' && <SetsView sets={sets} />}
      {tab === 'unknowns' && (
        <section className="flex flex-col gap-3">
          <p className="text-sm text-muted-foreground">
            Pieces the sorter couldn’t name. They usually go through again later for another try.
          </p>
          <PieceGrid pieces={logic.unknowns?.pieces} empty="No unknowns." />
        </section>
      )}
    </div>
  );
};

export default CollectionPage;
