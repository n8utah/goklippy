import { useCallback, useEffect, useState } from 'react';

import {
  activateSortingProfile,
  deleteSortingProfile,
  importSortingProfile,
  listSortingProfiles,
} from '../../helpers/api';

export const ACTIVATION_MODES = [
  {
    mode: 'keep',
    label: 'Keep its bins',
    hint: 'The bins it had last time it was active (none for a new profile).',
  },
  {
    mode: 'reset',
    label: 'Reset the bins',
    hint: 'Every bin starts unassigned; categories get bins as pieces arrive.',
  },
  {
    mode: 'preassign',
    label: 'Pre-assign from the rules',
    hint: 'The first rule gets layer 1 bin 1, the next rule the next bin, and so on.',
  },
];

export function useProfilesPageLogic() {
  const [profiles, setProfiles] = useState([]);
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
      setNotice(`Added “${saved.name}”. Activate it to sort with it.`);
    });

  const activate = (profile, mode) =>
    run(
      () => activateSortingProfile(profile.id, mode),
      `“${profile.name}” is now the active profile.`,
    );

  const remove = (profile) =>
    run(() => deleteSortingProfile(profile.id), `Deleted “${profile.name}”.`);

  return { profiles, loading, busy, error, notice, upload, activate, remove };
}
