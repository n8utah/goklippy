import { useCallback, useEffect, useState } from 'react';
import { useNavigate } from 'react-router';

import {
  createEditorProfile,
  deleteSortingProfile,
  getProfileEditorOptions,
  importSortingProfile,
  listSortingProfiles,
} from '../../helpers/api';

export function useProfilesPageLogic() {
  const navigate = useNavigate();
  const [profiles, setProfiles] = useState([]);
  const [starters, setStarters] = useState([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [reloadCount, setReloadCount] = useState(0);

  useEffect(() => {
    let cancelled = false;
    listSortingProfiles().then(
      ({ profiles: list }) => {
        if (cancelled) return;
        setProfiles(list);
        setLoading(false);
      },
      (err) => {
        if (cancelled) return;
        setError(err.message);
        setLoading(false);
      },
    );
    return () => {
      cancelled = true;
    };
  }, [reloadCount]);

  useEffect(() => {
    getProfileEditorOptions().then(
      (o) => setStarters(o.starters ?? []),
      () => {},
    );
  }, []);

  const run = useCallback(async (fn, done) => {
    setError('');
    setNotice('');
    setBusy(true);
    try {
      await fn();
      if (done) setNotice(done);
      setReloadCount((n) => n + 1);
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  }, []);

  const upload = (file) =>
    run(async () => {
      const text = await file.text();
      const saved = await importSortingProfile(text);
      setNotice(`Added “${saved.name}”. Pick it for a collection on Run › Collections.`);
    });

  const remove = (profile) =>
    run(() => deleteSortingProfile(profile.id), `Deleted “${profile.name}”.`);

  /** body: { name, starter? } or { copy_of }; opens the new profile in the editor. */
  const create = (body) =>
    run(async () => {
      const saved = await createEditorProfile(body);
      navigate(`/rules/profiles/${saved.id}`);
    });

  return {
    builtins: profiles.filter((p) => p.source === 'builtin'),
    made: profiles.filter((p) => p.source === 'editor'),
    uploads: profiles.filter((p) => p.source === 'upload'),
    starters,
    loading,
    busy,
    error,
    notice,
    upload,
    remove,
    create,
  };
}
