import { Link, Navigate, useNavigate, useParams, useSearchParams } from 'react-router';

import { Badge } from '@/components/ui/badge';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';

import {
  formatDuration,
  perMinuteText,
  SHORT_RUN_MS,
  SHORT_RUN_NOTE,
  sortingMs,
} from '@/helpers/sorterRuns';

import { useRecordsPageLogic } from './RecordsPage.logic';

const when = (t) => (t ? new Date(t).toLocaleString() : '—');

/**
 * Run › Records (/run/records): every run; a run opens its pieces under Pieces. The old
 * /run/records/pieces and /run/records/incidents addresses go to Pieces and Run ›
 * Incidents.
 */
const RecordsPage = () => {
  const { part } = useParams();
  const [params] = useSearchParams();
  const navigate = useNavigate();
  const logic = useRecordsPageLogic();
  const { stats } = logic;

  if (part === 'pieces') return <Navigate to={`/pieces/all?${params}`} replace />;
  if (part === 'incidents') return <Navigate to={`/run/incidents?${params}`} replace />;
  if (part) return <Navigate to="/run/records" replace />;

  const showRun = (id) => navigate(`/pieces/all?run=${id}`);

  return (
    <div className="ds flex flex-col gap-4">
      {stats && (
        <p className="text-sm text-muted-foreground">
          <span className="font-mono text-foreground">{stats.pieces.toLocaleString()}</span> pieces
          seen,{' '}
          <span className="font-mono text-foreground">{stats.distributed.toLocaleString()}</span>{' '}
          sorted, over <span className="font-mono text-foreground">{stats.runs}</span> run
          {stats.runs === 1 ? '' : 's'}.
        </p>
      )}
      {logic.error && <p className="text-sm text-destructive">{logic.error}</p>}
      <section className="rounded-xl border border-border bg-card">
        {logic.runs.length === 0 ? (
          <p className="px-5 py-6 text-sm text-muted-foreground">
            No runs yet. Each sort from Start to Stop is a run.
          </p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="pl-5">Run</TableHead>
                <TableHead>Collection</TableHead>
                <TableHead className="hidden sm:table-cell">Profile</TableHead>
                <TableHead className="hidden sm:table-cell">Started</TableHead>
                <TableHead className="hidden md:table-cell">Ended</TableHead>
                <TableHead className="text-right">Pieces</TableHead>
                <TableHead className="text-right">Sorted</TableHead>
                <TableHead className="hidden text-right lg:table-cell">Sorting time</TableHead>
                <TableHead className="pr-5 text-right">Per min</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {logic.runs.map((r) => (
                <TableRow key={r.id} className="cursor-pointer" onClick={() => showRun(r.id)}>
                  <TableCell className="pl-5 font-mono">
                    <button type="button" className="hover:underline" onClick={() => showRun(r.id)}>
                      {r.id}
                    </button>
                  </TableCell>
                  <TableCell>
                    <Link
                      to={`/run/collections/${r.collection_id}`}
                      className="hover:underline"
                      onClick={(e) => e.stopPropagation()}
                    >
                      {r.collection_name}
                    </Link>
                  </TableCell>
                  <TableCell className="hidden sm:table-cell">{r.profile_name || '—'}</TableCell>
                  <TableCell className="hidden sm:table-cell">{when(r.started_at)}</TableCell>
                  <TableCell className="hidden md:table-cell">
                    {r.ended_at ? when(r.ended_at) : <Badge variant="running">Running</Badge>}
                  </TableCell>
                  <TableCell className="text-right font-mono">{r.pieces}</TableCell>
                  <TableCell className="text-right font-mono">{r.distributed}</TableCell>
                  <TableCell
                    className="hidden text-right font-mono lg:table-cell"
                    title="From the start to its last piece sorted"
                  >
                    {r.last_piece_at ? formatDuration(sortingMs(r)) : '—'}
                  </TableCell>
                  <TableCell
                    className="pr-5 text-right font-mono"
                    title={perMinuteText(r).endsWith('*') ? SHORT_RUN_NOTE : undefined}
                  >
                    {perMinuteText(r)}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
        {logic.runs.some((r) => r.distributed && sortingMs(r) < SHORT_RUN_MS) && (
          <p className="border-t border-border px-5 py-3 text-xs text-muted-foreground">
            {SHORT_RUN_NOTE}
          </p>
        )}
      </section>
    </div>
  );
};

export default RecordsPage;
