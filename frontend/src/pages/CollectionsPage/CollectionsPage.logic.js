import { useCallback, useEffect, useState } from 'react';
import { toast } from 'sonner';

import {
  clearSorterCollectionUnknowns,
  getSorterCollectionPlan,
  getSorterSharing,
  listSorterCollections,
  listSorterPieces,
  listSorterRuns,
  putSorterSharing,
  testSorterSharing,
  updateSorterCollection,
} from '../../helpers/api';
import { useSorterState } from '../../helpers/sorterState';

const UNKNOWNS_SHOWN = 60;
const SHARING_POLL_MS = 5000;

/**
 * Sharing to the sort viewer site: where it sends, and how it's going (polled, since
 * sending runs on its own). `siteHost` is the site's name for labels ("lego.n8p.xyz").
 */
export function useSharing() {
  const [status, setStatus] = useState(null);
  const [busy, setBusy] = useState(false);
  const load = useCallback(
    () =>
      getSorterSharing().then(
        (s) => setStatus(s),
        () => {},
      ),
    [],
  );
  useEffect(() => {
    load();
    const timer = setInterval(load, SHARING_POLL_MS);
    return () => clearInterval(timer);
  }, [load]);

  let siteHost = '';
  try {
    siteHost = status?.url ? new URL(status.url).host : '';
  } catch {
    siteHost = '';
  }

  return {
    status,
    busy,
    siteHost,
    ready: Boolean(status?.url && status?.key_set),
    save: async (url, apiKey) => {
      setBusy(true);
      try {
        setStatus(await putSorterSharing(url, apiKey));
        toast.success('Saved');
        return true;
      } catch (err) {
        toast.error(err.message);
        return false;
      } finally {
        setBusy(false);
      }
    },
    test: async () => {
      setBusy(true);
      try {
        await testSorterSharing();
        toast.success(`${siteHost || 'The site'} answers and takes the key`);
      } catch (err) {
        toast.error(err.message);
      } finally {
        setBusy(false);
        load();
      }
    },
  };
}

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
    /** Shares it to the sort viewer site (sending everything in it so far), or stops. */
    setShared: (shared) =>
      act(
        () => updateSorterCollection(id, { shared }),
        () => (shared ? 'Shared: sending what’s sorted so far' : 'No longer shared'),
      ),
    /** bins: "emptied" or "continue" for the current collection; undefined otherwise. */
    changeProfile: (profileId, bins) =>
      act(
        () => updateSorterCollection(id, { profile_id: profileId, bins }),
        (col) => `Sorting by ${col.profile_name}`,
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
