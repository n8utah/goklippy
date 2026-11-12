import { RefreshCw } from 'lucide-react';
import { useEffect } from 'react';
import { Navigate, useNavigate, useParams, useSearchParams } from 'react-router';

import { useJourneyModal } from '@/components/JourneyModal/JourneyModal.logic';
import { PieceThumb, TONE_BADGE } from '@/components/PieceCard/PieceCard';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { PIECE_STATUS, pieceBin, pieceName, pieceStatus } from '@/helpers/sorterPieces';

import { PAGE_SIZE, useRecordsPageLogic } from './RecordsPage.logic';

const PARTS = ['pieces', 'runs', 'incidents'];
const ANY = '__any__';
const when = (t) => (t ? new Date(t).toLocaleString() : '—');

function Empty({ children }) {
  return <p className="px-5 py-6 text-sm text-muted-foreground">{children}</p>;
}

function PiecesTab({ logic, onOpen }) {
  const { filters, pieces, offset } = logic;
  return (
    <section className="rounded-xl border border-border bg-card">
      <div className="flex flex-wrap items-end gap-3 border-b border-border px-5 py-4">
        <label htmlFor="records-q" className="flex flex-col gap-1.5 text-sm">
          <span className="font-medium">Find</span>
          <Input
            id="records-q"
            className="w-60"
            placeholder="Part, category or piece id"
            value={filters.q}
            onChange={(e) => logic.setFilter('q', e.target.value)}
          />
        </label>
        <label className="flex flex-col gap-1.5 text-sm">
          <span className="font-medium">Status</span>
          <Select
            value={filters.status || ANY}
            onValueChange={(v) => logic.setFilter('status', v === ANY ? '' : v)}
          >
            <SelectTrigger className="w-40">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={ANY}>Any status</SelectItem>
              {Object.entries(PIECE_STATUS).map(([id, s]) => (
                <SelectItem key={id} value={id}>
                  {s.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </label>
        <label className="flex flex-col gap-1.5 text-sm">
          <span className="font-medium">Run</span>
          <Select
            value={filters.run || ANY}
            onValueChange={(v) => logic.setFilter('run', v === ANY ? '' : v)}
          >
            <SelectTrigger className="w-48">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={ANY}>Every run</SelectItem>
              {logic.runs.map((r) => (
                <SelectItem key={r.id} value={String(r.id)}>
                  Run {r.id} · {new Date(r.started_at).toLocaleDateString()}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </label>
        <Button variant="ghost" onClick={logic.refresh} className="ml-auto">
          <RefreshCw /> Refresh
        </Button>
      </div>

      {pieces.loading ? (
        <Empty>Loading…</Empty>
      ) : pieces.list.length === 0 ? (
        <Empty>
          {filters.q || filters.status || filters.run
            ? 'No pieces match.'
            : 'No pieces yet. Each piece sorted is recorded here.'}
        </Empty>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-16 pl-5">
                <span className="sr-only">Photo</span>
              </TableHead>
              <TableHead>Piece</TableHead>
              <TableHead className="hidden md:table-cell">Color</TableHead>
              <TableHead className="hidden lg:table-cell">Category → bin</TableHead>
              <TableHead>Status</TableHead>
              <TableHead className="hidden pr-5 sm:table-cell">Seen</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {pieces.list.map((p) => {
              const status = pieceStatus(p.status);
              return (
                <TableRow key={p.id} className="cursor-pointer" onClick={() => onOpen(p.id)}>
                  <TableCell className="pl-5">
                    <PieceThumb piece={p} className="size-10" />
                  </TableCell>
                  <TableCell>
                    <button
                      type="button"
                      className="text-left font-medium hover:underline"
                      onClick={(e) => {
                        e.stopPropagation();
                        onOpen(p.id);
                      }}
                    >
                      {pieceName(p)}
                    </button>
                    {p.part_id && (
                      <span className="ml-2 font-mono text-xs text-muted-foreground">
                        {p.part_id}
                      </span>
                    )}
                  </TableCell>
                  <TableCell className="hidden md:table-cell">{p.color_name || '—'}</TableCell>
                  <TableCell className="hidden lg:table-cell">
                    {p.category_name ? `${p.category_name} → ` : ''}
                    {pieceBin(p)}
                  </TableCell>
                  <TableCell>
                    <Badge variant={TONE_BADGE[status.tone]}>{status.label}</Badge>
                    {p.problems?.length > 0 && (
                      <span className="ml-2 text-xs text-warning">
                        {p.problems.length} problem{p.problems.length === 1 ? '' : 's'}
                      </span>
                    )}
                  </TableCell>
                  <TableCell className="hidden pr-5 text-sm text-muted-foreground sm:table-cell">
                    {when(p.seen_at)}
                  </TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      )}

      {pieces.total > 0 && (
        <div className="flex items-center justify-end gap-2 border-t border-border px-5 py-3">
          <span className="mr-auto font-mono text-sm text-muted-foreground">
            {offset + 1}–{Math.min(offset + PAGE_SIZE, pieces.total)} of {pieces.total}
          </span>
          <Button
            variant="outline"
            size="sm"
            disabled={offset === 0}
            onClick={() => logic.setOffset(Math.max(0, offset - PAGE_SIZE))}
          >
            Newer
          </Button>
          <Button
            variant="outline"
            size="sm"
            disabled={offset + PAGE_SIZE >= pieces.total}
            onClick={() => logic.setOffset(offset + PAGE_SIZE)}
          >
            Older
          </Button>
        </div>
      )}
    </section>
  );
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
 * Run › Records: Pieces, Runs and Incidents (/run/records/<tab>). A piece opens its
 * journey. ?run= shows one run's pieces; ?piece= opens a piece.
 */
const RecordsPage = () => {
  const { part = 'pieces' } = useParams();
  const [params, setParams] = useSearchParams();
  const navigate = useNavigate();
  const journey = useJourneyModal();
  const logic = useRecordsPageLogic(params.get('run'));
  const { stats } = logic;

  // A link to one piece opens its journey once, then leaves the address clean.
  const linkedPiece = params.get('piece');
  useEffect(() => {
    if (!linkedPiece) return;
    journey.open(linkedPiece);
    setParams(
      (prev) => {
        const next = new URLSearchParams(prev);
        next.delete('piece');
        return next;
      },
      { replace: true },
    );
  }, [linkedPiece, journey, setParams]);

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
      {part === 'pieces' && <PiecesTab logic={logic} onOpen={journey.open} />}
      {part === 'runs' && (
        <RunsTab
          logic={logic}
          onShowRun={(id) => {
            logic.showRun(id);
            navigate('/run/records/pieces');
          }}
        />
      )}
      {part === 'incidents' && <IncidentsTab logic={logic} onOpenPiece={journey.open} />}
    </div>
  );
};

export default RecordsPage;
