import {
  ArrowDownWideNarrow,
  ArrowUpNarrowWide,
  Camera,
  Download,
  ImageIcon,
  LayoutGrid,
  List,
  PackagePlus,
  RefreshCw,
  Trash2,
} from 'lucide-react';
import { useEffect, useState } from 'react';
import { Navigate, useNavigate, useParams, useSearchParams } from 'react-router';

import { useJourneyModal } from '@/components/JourneyModal/JourneyModal.logic';
import Segmented from '@/components/Segmented/Segmented';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { PIECE_STATUS } from '@/helpers/sorterPieces';
import { cn } from '@/lib/utils';

import ConfirmClear from './components/ConfirmClear';
import PartModal from './components/PartModal';
import { PartGrid, PartList } from './components/PartViews';
import { PieceGrid, PieceList } from './components/PieceViews';
import { PAGE_SIZE, SORTS, formatBytes, usePiecesPageLogic } from './PiecesPage.logic';

const TABS = ['parts', 'all'];
const ANY = '__any__';

const withIcon = (Icon, text) => (
  <span className="flex items-center gap-1.5">
    <Icon className="size-4" aria-hidden /> {text}
  </span>
);

function FilterSelect({ label, value, onChange, anyLabel, options, className }) {
  return (
    <label className="flex flex-col gap-1.5 text-sm">
      <span className="font-medium">{label}</span>
      <Select value={value || ANY} onValueChange={(v) => onChange(v === ANY ? '' : v)}>
        <SelectTrigger className={cn('w-full', className)}>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={ANY}>{anyLabel}</SelectItem>
          {options.map((o) => (
            <SelectItem key={o.value} value={o.value}>
              {o.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </label>
  );
}

function Filters({ tab, logic }) {
  const { filters, facets } = logic;
  const sort = SORTS.find((s) => s.value === logic.sort.sort) ?? SORTS[0];
  return (
    // Two to a row on a phone; one row, wrapping, wider.
    <div className="grid grid-cols-2 items-end gap-3 border-b border-border px-4 py-4 sm:flex sm:flex-wrap sm:px-5">
      <label htmlFor="pieces-q" className="col-span-2 flex flex-col gap-1.5 text-sm">
        <span className="font-medium">Find</span>
        <Input
          id="pieces-q"
          className="w-full sm:w-56"
          placeholder={tab === 'parts' ? 'Name, part number, color' : 'Part, name, color, piece id'}
          value={filters.q}
          onChange={(e) => logic.setFilter('q', e.target.value)}
        />
      </label>
      <FilterSelect
        label="Color"
        value={filters.color}
        onChange={(v) => logic.setFilter('color', v)}
        anyLabel="Every color"
        className="sm:w-40"
        options={(facets?.colors ?? []).map((f) => ({
          value: f.id,
          label: `${f.name || 'No color'} (${f.count})`,
        }))}
      />
      <FilterSelect
        label="Category"
        value={filters.category}
        onChange={(v) => logic.setFilter('category', v)}
        anyLabel="Every category"
        className="sm:w-44"
        options={(facets?.categories ?? []).map((f) => ({
          value: f.name,
          label: `${f.name} (${f.count})`,
        }))}
      />
      <FilterSelect
        label="Run"
        value={filters.run}
        onChange={(v) => logic.setFilter('run', v)}
        anyLabel="Every run"
        className="sm:w-48"
        options={logic.runs.map((r) => ({
          value: String(r.id),
          label: `Run ${r.id} · ${new Date(r.started_at).toLocaleDateString()}`,
        }))}
      />
      {tab === 'all' ? (
        <FilterSelect
          label="Status"
          value={filters.status}
          onChange={(v) => logic.setFilter('status', v)}
          anyLabel="Any status"
          className="sm:w-40"
          options={Object.entries(PIECE_STATUS).map(([id, s]) => ({ value: id, label: s.label }))}
        />
      ) : (
        <label className="flex flex-col gap-1.5 text-sm">
          <span className="font-medium">Sort</span>
          <span className="flex gap-1">
            <Select value={sort.value} onValueChange={logic.setSort}>
              <SelectTrigger className="w-full min-w-0 flex-1 sm:w-36 sm:flex-none">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {SORTS.map((s) => (
                  <SelectItem key={s.value} value={s.value}>
                    {s.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Button
              variant="outline"
              size="icon"
              onClick={logic.flipDir}
              aria-label={
                logic.sort.dir === 'asc' ? 'Ascending; reverse it' : 'Descending; reverse it'
              }
              title={logic.sort.dir === 'asc' ? 'Ascending' : 'Descending'}
            >
              {logic.sort.dir === 'asc' ? <ArrowUpNarrowWide /> : <ArrowDownWideNarrow />}
            </Button>
          </span>
        </label>
      )}
      {logic.filtered && (
        <Button variant="ghost" onClick={logic.resetFilters}>
          Show all
        </Button>
      )}
      <div className="col-span-2 flex flex-wrap gap-2 sm:ml-auto">
        <Segmented
          label="Pictures"
          value={logic.picture}
          onChange={logic.setPicture}
          options={[
            { value: 'official', label: withIcon(ImageIcon, 'Official') },
            { value: 'camera', label: withIcon(Camera, 'Camera') },
          ]}
        />
        <Segmented
          label="View"
          value={logic.view}
          onChange={logic.setView}
          options={[
            { value: 'grid', label: withIcon(LayoutGrid, 'Grid') },
            { value: 'list', label: withIcon(List, 'List') },
          ]}
        />
      </div>
    </div>
  );
}

function Actions({ tab, logic, count, onConfirm }) {
  const { facets, photos } = logic;
  return (
    <div className="flex flex-wrap items-center gap-1 border-b border-border px-5 py-2.5">
      <span className="mr-auto text-sm text-muted-foreground">{count}</span>
      <Button variant="ghost" size="sm" onClick={logic.refresh}>
        <RefreshCw /> Refresh
      </Button>
      <Button
        variant="ghost"
        size="sm"
        onClick={logic.exportCsv}
        disabled={logic.busy !== ''}
        title={
          tab === 'parts' ? 'The parts shown, as a CSV file' : 'Every piece matching, as a CSV file'
        }
      >
        <Download /> {logic.busy === 'export' ? 'Exporting…' : 'Export CSV'}
      </Button>
      {tab === 'parts' && (
        <Button
          variant="ghost"
          size="sm"
          onClick={logic.fill}
          disabled={logic.busy !== ''}
          title="Count the identified pieces in the records that aren't in the parts yet"
        >
          <PackagePlus /> {logic.busy === 'fill' ? 'Filling…' : 'Fill from records'}
        </Button>
      )}
      <Button
        variant="ghost"
        size="sm"
        className="text-destructive"
        disabled={logic.busy !== '' || !photos?.files}
        title="Remove the camera photos only"
        onClick={() =>
          onConfirm({
            title: 'Clear the camera photos?',
            text: `This removes all ${(photos?.files ?? 0).toLocaleString()} piece photos (${formatBytes(photos?.bytes)}). Every piece's record and the parts stay; pictures fall back to Brickognize's. Photos of pieces sorted from now on are kept as usual.`,
            confirm: 'Clear photos',
            action: logic.clearPhotos,
          })
        }
      >
        <Trash2 /> Clear photos{photos?.files ? ` (${formatBytes(photos.bytes)})` : ''}
      </Button>
      {tab === 'parts' && (
        <Button
          variant="ghost"
          size="sm"
          className="text-destructive"
          disabled={logic.busy !== '' || !facets?.parts}
          title="Empty the parts list only"
          onClick={() =>
            onConfirm({
              title: 'Clear the parts?',
              text: `This empties the parts: ${facets?.parts ?? 0} parts and colors, ${facets?.pieces ?? 0} pieces counted. Every piece's record and photos stay, so Fill from records can count them again.`,
              confirm: 'Clear parts',
              action: logic.clearCatalog,
            })
          }
        >
          <Trash2 /> Clear parts
        </Button>
      )}
    </div>
  );
}

function Pager({ logic }) {
  const { pieces, offset } = logic;
  if (!pieces?.total) return null;
  return (
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
  );
}

function Empty({ children }) {
  return <p className="px-5 py-6 text-sm text-muted-foreground">{children}</p>;
}

/**
 * Pieces: Parts (every part and colour identified, across runs) and Every piece (each
 * piece seen), at /pieces/<tab>. A part opens in full; a piece opens its journey.
 * ?run= filters to a run; ?piece= opens a piece.
 */
const PiecesPage = () => {
  const { part: tab = 'parts' } = useParams();
  const [params, setParams] = useSearchParams();
  const navigate = useNavigate();
  const journey = useJourneyModal();
  const logic = usePiecesPageLogic(tab, params.get('run'));
  const [confirm, setConfirm] = useState(null);
  const { facets, parts, pieces } = logic;

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

  if (!TABS.includes(tab)) return <Navigate to="/pieces/parts" replace />;

  const count =
    tab === 'parts'
      ? parts
        ? `${parts.length.toLocaleString()} part${parts.length === 1 ? '' : 's'} shown`
        : 'Loading…'
      : pieces
        ? `${pieces.total.toLocaleString()} piece${pieces.total === 1 ? '' : 's'}`
        : 'Loading…';

  let body = null;
  if (tab === 'parts' && parts) {
    const props = {
      parts,
      run: logic.shownRun,
      picture: logic.picture,
      onOpen: logic.setOpenPart,
    };
    body =
      parts.length === 0 ? (
        <Empty>
          {logic.filtered
            ? 'No parts match.'
            : 'No parts yet. Each identified piece is counted as it’s sorted; Fill from records counts the ones sorted before.'}
        </Empty>
      ) : logic.view === 'grid' ? (
        <PartGrid {...props} />
      ) : (
        <PartList {...props} />
      );
  } else if (tab === 'all' && pieces) {
    const props = { pieces: pieces.pieces, picture: logic.picture, onOpen: journey.open };
    body =
      pieces.pieces.length === 0 ? (
        <Empty>
          {logic.filtered
            ? 'No pieces match.'
            : 'No pieces yet. Each piece sorted is recorded here.'}
        </Empty>
      ) : logic.view === 'grid' ? (
        <PieceGrid {...props} />
      ) : (
        <PieceList {...props} />
      );
  }

  return (
    <div className="ds flex flex-col gap-4">
      {facets && (
        <p className="text-sm text-muted-foreground">
          <span className="font-mono text-foreground">{facets.parts.toLocaleString()}</span>{' '}
          different part{facets.parts === 1 ? '' : 's'} and colors found,{' '}
          <span className="font-mono text-foreground">{facets.pieces.toLocaleString()}</span>{' '}
          identified piece{facets.pieces === 1 ? '' : 's'}, over{' '}
          <span className="font-mono text-foreground">{facets.runs.length}</span> run
          {facets.runs.length === 1 ? '' : 's'}.
        </p>
      )}
      {logic.error && <p className="text-sm text-destructive">{logic.error}</p>}
      {logic.message && <p className="text-sm text-muted-foreground">{logic.message}</p>}

      <section className="rounded-xl border border-border bg-card">
        <Filters tab={tab} logic={logic} />
        <Actions tab={tab} logic={logic} count={count} onConfirm={setConfirm} />
        {body}
        {tab === 'all' && <Pager logic={logic} />}
      </section>

      <PartModal
        target={logic.openPart}
        onClose={() => logic.setOpenPart(null)}
        onShowRun={(run) => {
          logic.setOpenPart(null);
          navigate(`/pieces/all?run=${run}`);
        }}
      />
      <ConfirmClear what={confirm} onClose={() => setConfirm(null)} />
    </div>
  );
};

export default PiecesPage;
