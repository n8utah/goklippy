import { Navigate, useNavigate, useParams, useSearchParams } from 'react-router';

import { useJourneyModal } from '@/components/JourneyModal/JourneyModal.logic';
import { Badge } from '@/components/ui/badge';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';

import { useRecordsPageLogic } from './RecordsPage.logic';

const PARTS = ['runs', 'incidents'];
const when = (t) => (t ? new Date(t).toLocaleString() : '—');

function Empty({ children }) {
  return <p className="px-5 py-6 text-sm text-muted-foreground">{children}</p>;
}

function RunsTab({ logic, onShowRun }) {
  return (
    <section className="rounded-xl border border-border bg-card">
      {logic.runs.length === 0 ? (
        <Empty>No runs yet. Each sort from Start to Stop is a run.</Empty>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="pl-5">Run</TableHead>
              <TableHead>Profile</TableHead>
              <TableHead className="hidden sm:table-cell">Started</TableHead>
              <TableHead className="hidden md:table-cell">Ended</TableHead>
              <TableHead className="text-right">Pieces</TableHead>
              <TableHead className="pr-5 text-right">Sorted</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {logic.runs.map((r) => (
              <TableRow key={r.id} className="cursor-pointer" onClick={() => onShowRun(r.id)}>
                <TableCell className="pl-5 font-mono">
                  <button type="button" className="hover:underline" onClick={() => onShowRun(r.id)}>
                    {r.id}
                  </button>
                </TableCell>
                <TableCell>{r.profile_name || '—'}</TableCell>
                <TableCell className="hidden sm:table-cell">{when(r.started_at)}</TableCell>
                <TableCell className="hidden md:table-cell">
                  {r.ended_at ? when(r.ended_at) : <Badge variant="running">Running</Badge>}
                </TableCell>
                <TableCell className="text-right font-mono">{r.pieces}</TableCell>
                <TableCell className="pr-5 text-right font-mono">{r.distributed}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </section>
  );
}

function IncidentsTab({ logic, onOpenPiece }) {
  return (
    <section className="rounded-xl border border-border bg-card">
      {logic.incidents.length === 0 ? (
        <Empty>No incidents. Problems during a sort that needed a person show here.</Empty>
      ) : (
        <ul className="divide-y divide-border">
          {logic.incidents.map((i) => (
            <li
              key={i.id}
              className="flex flex-col gap-1 px-5 py-3 sm:flex-row sm:items-start sm:gap-4"
            >
              <span className="w-40 shrink-0 font-mono text-xs text-muted-foreground">
                {when(i.opened_at)}
              </span>
              <span className="min-w-0 flex-1 text-sm">
                <span className="font-medium">{i.title}</span>
                <span className="block text-muted-foreground">{i.message}</span>
                {i.piece_id && (
                  <button
                    type="button"
                    className="text-sm font-medium text-primary-text hover:underline"
                    onClick={() => onOpenPiece(i.piece_id)}
                  >
                    Its piece
                  </button>
                )}
              </span>
              <Badge variant={i.resolved_at ? 'idle' : 'warning'} className="self-start">
                {i.resolved_at ? `${i.resolution || 'Resolved'} · ${when(i.resolved_at)}` : 'Open'}
              </Badge>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

/**
 * Run › Records: Runs and Incidents (/run/records/<tab>). A run opens its pieces under
 * Pieces; the old /run/records/pieces address goes there too.
 */
const RecordsPage = () => {
  const { part = 'runs' } = useParams();
  const [params] = useSearchParams();
  const navigate = useNavigate();
  const journey = useJourneyModal();
  const logic = useRecordsPageLogic();
  const { stats } = logic;

  if (part === 'pieces') return <Navigate to={`/pieces/all?${params}`} replace />;
  if (!PARTS.includes(part)) return <Navigate to="/run/records" replace />;

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
      {part === 'runs' && (
        <RunsTab logic={logic} onShowRun={(id) => navigate(`/pieces/all?run=${id}`)} />
      )}
      {part === 'incidents' && <IncidentsTab logic={logic} onOpenPiece={journey.open} />}
    </div>
  );
};

export default RecordsPage;
