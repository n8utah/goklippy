import { useEffect, useState } from 'react';

import { listCollections } from '@/helpers/api';

const POLL_MS = 15000;

/** The collections you may see, refreshed now and then (a sort may start any time). */
export function useHomePage() {
  const [collections, setCollections] = useState(null);
  const [error, setError] = useState('');
  useEffect(() => {
    let cancelled = false;
    const load = () =>
      listCollections().then(
        (data) => {
          if (cancelled) return;
          setCollections(data.collections ?? []);
          setError('');
        },
        (err) => !cancelled && setError(err.message),
      );
    load();
    const timer = setInterval(load, POLL_MS);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, []);
  return { collections, error };
}
