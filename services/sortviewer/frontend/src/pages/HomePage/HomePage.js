import { ChevronRight } from 'lucide-react';
import { Link, Navigate } from 'react-router';

import { Badge } from '@/components/ui/badge';
import { useSession } from '@/helpers/session';

import { useHomePage } from './HomePage.logic';

const ago = (t) => {
  if (!t) return 'nothing sorted yet';
  const mins = Math.round((Date.now() - new Date(t).getTime()) / 60000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins} min ago`;
  if (mins < 48 * 60) return `${Math.round(mins / 60)} h ago`;
  return new Date(t).toLocaleDateString();
};

/** Your collections. A viewer with just one goes straight to it. */
const HomePage = () => {
  const { me } = useSession();
  const { collections, error } = useHomePage();

  if (collections?.length === 1 && !me?.admin) {
    return <Navigate to={`/c/${collections[0].id}`} replace />;
  }

  return (
    <div className="flex flex-col gap-4">
      <h1 className="font-display text-2xl font-semibold">
        {me?.admin ? 'Collections' : 'Your collections'}
      </h1>
      {error && <p className="text-sm text-destructive">{error}</p>}
      {collections?.length === 0 && (
        <p className="rounded-xl border border-border bg-card p-5 text-sm text-muted-foreground">
          {me?.admin
            ? 'Nothing shared yet. Turn on sharing for a collection on the sorter and it appears here.'
            : 'Nothing has been shared with you yet. You’ll see your collection here once sorting starts.'}
        </p>
      )}
      <ul className="grid gap-3 sm:grid-cols-2">
        {(collections ?? []).map((c) => (
          <li key={c.id}>
            <Link
              to={`/c/${c.id}`}
              className="flex items-center gap-3 rounded-xl border border-border bg-card p-4 transition-colors hover:border-border-strong"
            >
              <div className="min-w-0 flex-1">
                <p className="flex items-center gap-2">
                  <span className="truncate font-display text-lg font-semibold">{c.name}</span>
                  {c.sorting && <Badge variant="running">Sorting now</Badge>}
                </p>
                <p className="text-sm text-muted-foreground">
                  <span className="font-mono text-foreground">{c.pieces.toLocaleString()}</span>{' '}
                  pieces · {ago(c.last_piece_at)}
                </p>
              </div>
              <ChevronRight className="size-5 text-muted-foreground" />
            </Link>
          </li>
        ))}
      </ul>
    </div>
  );
};

export default HomePage;
