import { useCallback, useEffect, useState } from 'react';
import { toast } from 'sonner';

import {
  clearAllPhotos,
  clearCollectionPhotos,
  createKey,
  createViewer,
  deleteCollection,
  deleteViewer,
  listCollections,
  listKeys,
  listViewers,
  revokeKey,
  updateViewer,
} from '@/helpers/api';

export const ADMIN_TABS = [
  { id: 'viewers', label: 'Viewers' },
  { id: 'collections', label: 'Collections' },
  { id: 'keys', label: 'Sorter keys' },
];

/** Admin: viewers and what they see, the sorter's keys, and clearing out collections. */
export function useAdminPage() {
  const [viewers, setViewers] = useState(null);
  const [collections, setCollections] = useState([]);
  const [keys, setKeys] = useState(null);
  const [newKey, setNewKey] = useState(''); // shown once, right after it's made
  const [busy, setBusy] = useState(false);
  const [confirm, setConfirm] = useState(null);

  const load = useCallback(
    () =>
      Promise.all([listViewers(), listCollections(), listKeys()]).then(
        ([v, c, k]) => {
          setViewers(v.viewers ?? []);
          setCollections(c.collections ?? []);
          setKeys(k.keys ?? []);
        },
        (err) => toast.error(err.message),
      ),
    [],
  );
  useEffect(() => {
    load();
  }, [load]);

  const act = async (fn, done) => {
    setBusy(true);
    try {
      const result = await fn();
      if (done) toast.success(done(result));
      await load();
      return true;
    } catch (err) {
      toast.error(err.message);
      return false;
    } finally {
      setBusy(false);
    }
  };

  return {
    viewers,
    collections,
    keys,
    busy,
    newKey,
    clearNewKey: () => setNewKey(''),
    confirm,
    closeConfirm: () => setConfirm(null),
    saveViewer: (viewer) =>
      viewer.id
        ? act(
            () => updateViewer(viewer.id, viewer),
            () => 'Saved',
          )
        : act(
            () => createViewer(viewer),
            () => `Added ${viewer.email}`,
          ),
    askDeleteViewer: (v) =>
      setConfirm({
        title: `Remove ${v.email}?`,
        text: 'They won’t be able to sign in any more. Their collections aren’t touched.',
        confirm: 'Remove',
        action: () =>
          act(
            () => deleteViewer(v.id),
            () => 'Removed',
          ),
      }),
    makeKey: (name) =>
      act(
        async () => setNewKey((await createKey(name)).key),
        () => 'Key made: copy it now',
      ),
    askRevokeKey: (k) =>
      setConfirm({
        title: `Revoke “${k.name}”?`,
        text: 'The sorter using it can’t send updates any more until it gets a new key.',
        confirm: 'Revoke',
        action: () =>
          act(
            () => revokeKey(k.id),
            () => 'Revoked',
          ),
      }),
    askClearPhotos: (c) =>
      setConfirm({
        title: c ? `Clear ${c.name}’s photos?` : 'Clear every photo?',
        text: 'The photos are deleted from this site; the pieces and counts stay. The official part pictures still show.',
        confirm: 'Clear photos',
        action: () =>
          act(
            () => (c ? clearCollectionPhotos(c.id) : clearAllPhotos()),
            (r) => `Cleared ${r.cleared} photo${r.cleared === 1 ? '' : 's'}`,
          ),
      }),
    askDeleteCollection: (c) =>
      setConfirm({
        title: `Delete ${c.name}?`,
        text: 'Its pieces, counts, plan and photos are deleted from this site. If the sorter shares it again, it comes back from then on.',
        confirm: 'Delete',
        action: () =>
          act(
            () => deleteCollection(c.id),
            () => 'Deleted',
          ),
      }),
  };
}
