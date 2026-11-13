import { useNavigate, useParams } from 'react-router';

import CollectionPicker from '@/components/CollectionPicker/CollectionPicker';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';

import { useCollectionsList } from './CollectionsPage.logic';
import CollectionDetail from './components/CollectionDetail';

const when = (t) => (t ? new Date(t).toLocaleString() : '—');

/**
 * Run › Collections (/run/collections, /run/collections/<id>): each person's LEGO being
 * sorted, over as many runs as it takes. A collection opens its plan (the bins it's
 * sorted into), its unknowns and its runs.
 */
const CollectionsPage = () => {
  const { part } = useParams();
  if (part) return <CollectionDetail key={part} id={part} />;
  return <CollectionsList />;
};

function CollectionsList() {
  const logic = useCollectionsList();
  const navigate = useNavigate();
  const open = (id) => navigate(`/run/collections/${id}`);

  return (
    <div className="ds flex flex-col gap-4">
      <section className="flex flex-col gap-3 rounded-xl border border-border bg-card p-5 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h2 className="font-display text-lg font-semibold">Sorting into</h2>
          <p className="text-sm text-muted-foreground">
            The collection the next run sorts into. Switching means the bins were emptied.
          </p>
        </div>
        <CollectionPicker
          id="collections-current"
          current={logic.state?.collection}
          locked={Boolean(logic.state?.run_id)}
          onPicked={logic.reload}
        />
      </section>

      {logic.error && <p className="text-sm text-destructive">{logic.error}</p>}

      <section className="rounded-xl border border-border bg-card">
        {logic.collections.length === 0 ? (
          <p className="px-5 py-6 text-sm text-muted-foreground">
            No collections yet. Make one with the picker above (New collection…), then start a sort
            on Run › Sort.
          </p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="pl-5">Collection</TableHead>
                <TableHead className="text-right">Runs</TableHead>
                <TableHead className="text-right">Pieces</TableHead>
                <TableHead className="hidden text-right sm:table-cell">Identified</TableHead>
                <TableHead className="text-right">Unknown</TableHead>
                <TableHead className="hidden pr-5 md:table-cell">Last sorted</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {logic.collections.map((c) => (
                <TableRow key={c.id} className="cursor-pointer" onClick={() => open(c.id)}>
                  <TableCell className="pl-5">
                    <button
                      type="button"
                      className="font-medium hover:underline"
                      onClick={() => open(c.id)}
                    >
                      {c.name}
                    </button>{' '}
                    {c.current && <Badge variant="running">Current</Badge>}
                    {c.archived_at && <Badge variant="idle">Archived</Badge>}
                  </TableCell>
                  <TableCell className="text-right font-mono">{c.runs}</TableCell>
                  <TableCell className="text-right font-mono">
                    {c.pieces.toLocaleString()}
                  </TableCell>
                  <TableCell className="hidden text-right font-mono sm:table-cell">
                    {c.identified.toLocaleString()}
                  </TableCell>
                  <TableCell className="text-right font-mono">
                    {c.unknown.toLocaleString()}
                  </TableCell>
                  <TableCell className="hidden pr-5 md:table-cell">
                    {when(c.last_sorted_at)}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
        {logic.archived > 0 && (
          <div className="border-t border-border px-5 py-3">
            <Button variant="ghost" size="sm" onClick={logic.toggleArchived}>
              {logic.showArchived ? 'Hide archived' : `Show archived (${logic.archived})`}
            </Button>
          </div>
        )}
      </section>
    </div>
  );
}

export default CollectionsPage;
