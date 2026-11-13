import { useEffect, useState } from 'react';

import { getSorterStats, listSorterRuns } from '../../helpers/api';

/**
 * Run › Records: every run and lifetime totals. (Pieces have their own group, Pieces;
 * incidents their own page, Run › Incidents.)
 */
export function useRecordsPageLogic() {
  const [runs, setRuns] = useState([]);
  const [stats, setStats] = useState(null);
  const [error, setError] = useState('');

  useEffect(() => {
    let cancelled = false;
    Promise.all([listSorterRuns(), getSorterStats()]).then(
      ([r, s]) => {
        if (cancelled) return;
        setRuns(r.runs ?? []);
        setStats(s);
      },
      (err) => !cancelled && setError(err.message),
    );
    return () => {
      cancelled = true;
    };
  }, []);

  return { runs, stats, error };
}
