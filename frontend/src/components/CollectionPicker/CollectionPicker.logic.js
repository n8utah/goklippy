import { useCallback, useEffect, useState } from 'react';
import { toast } from 'sonner';

import {
  createSorterCollection,
  listSorterCollections,
  selectSorterCollection,
} from '@/helpers/api';

/**
 * The collections a run can sort into, and picking one. Picking another than the current
 * collection means its bins were emptied, so it asks first (`confirm`); a new collection
 * is made and picked in one go.
 */
export function useCollectionPicker(current, onPicked) {
  const [collections, setCollections] = useState([]);
  const [confirm, setConfirm] = useState(null); // the collection waiting to be picked
  const [creating, setCreating] = useState(false);
  const [busy, setBusy] = useState(false);

  const load = useCallback(
    () =>
      listSorterCollections().then(
        (data) => setCollections(data.collections ?? []),
        () => {},
      ),
    [],
  );
  useEffect(() => {
    load();
  }, [load]);

  const pick = async (col) => {
    setBusy(true);
    try {
      await selectSorterCollection(col.id);
      toast.success(`Sorting into ${col.name}`);
      onPicked?.(col);
      await load();
    } catch (err) {
      toast.error(err.message);
    } finally {
      setBusy(false);
      setConfirm(null);
    }
  };

  return {
    /** Unarchived ones, plus the current one even if archived. */
    collections: collections.filter((c) => !c.archived_at || c.id === current?.id),
    busy,
    confirm,
    creating,
    choose: (id) => {
      const col = collections.find((c) => String(c.id) === String(id));
      if (!col || col.id === current?.id) return;
      // Nothing current yet: no bins to empty, so no need to ask.
      if (!current) pick(col);
      else setConfirm(col);
    },
    confirmPick: () => confirm && pick(confirm),
    cancelPick: () => setConfirm(null),
    startCreating: () => setCreating(true),
    cancelCreating: () => setCreating(false),
    create: async (name, profileId) => {
      setBusy(true);
      try {
        const col = await createSorterCollection(name, profileId);
        setCreating(false);
        await pick(col);
      } catch (err) {
        toast.error(err.message);
        setBusy(false);
      }
    },
  };
}
