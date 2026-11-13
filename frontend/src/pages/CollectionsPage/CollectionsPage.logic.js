import { useCallback, useEffect, useState } from 'react';
import { toast } from 'sonner';

import {
  clearSorterCollectionUnknowns,
  getSorterCollectionPlan,
  listSorterCollections,
  listSorterPieces,
  listSorterRuns,
  updateSorterCollection,
} from '../../helpers/api';
import { useSorterState } from '../../helpers/sorterState';

const UNKNOWNS_SHOWN = 60;

/** Run › Collections: every collection with its totals. */
export function useCollectionsList() {
  const { state } = useSorterState();
  const [collections, setCollections] = useState([]);
  const [showArchived, setShowArchived] = useState(false);
  const [error, setError] = useState('');

  const load = useCallback(
    () =>
      listSorterCollections().then(
        (data) => {
          setCollections(data.collections ?? []);
          setError('');
        },
        (err) => setError(err.message),
      ),
    [],
  );
  // Reload when the current collection changes (picked here or on Run › Sort).
  const currentId = state?.collection?.id;
  useEffect(() => {
    load();
  }, [load, currentId]);

  const archived = collections.filter((c) => c.archived_at).length;
  return {
    state,
    error,
    archived,
    showArchived,
    toggleArchived: () => setShowArchived((v) => !v),
    collections: showArchived ? collections : collections.filter((c) => !c.archived_at),
    reload: load,
  };
}

/** One collection: its totals, plan, unknowns and runs, and renaming, archiving and
 * clearing its unknowns. */
export function useCollectionDetail(id) {
  const { state } = useSorterState();
  const [plan, setPlan] = useState(null);
  const [unknowns, setUnknowns] = useState({ pieces: [], total: 0 });
  const [runs, setRuns] = useState([]);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const [confirm, setConfirm] = useState(null);

  const sortingHere = Boolean(state?.run_id && state?.collection?.id === Number(id));
  // While it's being sorted, its numbers move: refresh as pieces land.
  const tick = sortingHere ? (state?.recent?.[0]?.id ?? '') : '';

  const load = useCallback(() => {
    return Promise.all([
      getSorterCollectionPlan(id),
      listSorterPieces({ collection: id, unknown: true, limit: UNKNOWNS_SHOWN }),
      listSorterRuns(id),
    ]).then(
      ([p, u, r]) => {
        setPlan(p);
        setUnknowns({ pieces: u.pieces ?? [], total: u.total ?? 0 });
        setRuns(r.runs ?? []);
        setError('');
      },
      (err) => setError(err.message),
    );
  }, [id]);
  useEffect(() => {
    load();
  }, [load, tick]);

  const act = async (fn, done) => {
    setBusy(true);
    try {
      const result = await fn();
      if (done) toast.success(done(result));
      await load();
    } catch (err) {
      toast.error(err.message);
    } finally {
      setBusy(false);
    }
  };

  const collection = plan?.collection ?? null;
  return {
    collection,
    plan,
    unknowns,
    unknownsShown: UNKNOWNS_SHOWN,
    runs,
    error,
    busy,
    sortingHere,
    rename: (name) =>
      act(
        () => updateSorterCollection(id, { name }),
        () => 'Renamed',
      ),
    setArchived: (archived) =>
      act(
        () => updateSorterCollection(id, { archived }),
        () => (archived ? 'Archived' : 'Unarchived'),
      ),
    confirm,
    closeConfirm: () => setConfirm(null),
    askClearUnknowns: () =>
      setConfirm({
        title: `Clear ${collection?.name}’s unknowns?`,
        text: `Deletes its ${unknowns.total} unknown piece${unknowns.total === 1 ? '' : 's'} and their photos, ready to feed the unknowns bin through again so they aren’t counted twice. Identified pieces stay.`,
        confirm: 'Clear unknowns',
        action: () =>
          act(
            () => clearSorterCollectionUnknowns(id),
            (r) => `Cleared ${r.cleared} unknown piece${r.cleared === 1 ? '' : 's'}`,
          ),
      }),
  };
}
