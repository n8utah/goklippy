import { useEffect, useState } from 'react';

import { getSorterStats, listSorterIncidents, listSorterRuns } from '../../helpers/api';

/**
 * Run › Records: runs, incidents and lifetime totals. (Pieces have their own group,
 * Pieces.)
 */
export function useRecordsPageLogic() {
  const [runs, setRuns] = useState([]);
  const [incidents, setIncidents] = useState([]);
  const [stats, setStats] = useState(null);
  const [error, setError] = useState('');

  useEffect(() => {
    let cancelled = false;
    Promise.all([listSorterRuns(), listSorterIncidents(), getSorterStats()]).then(
      ([r, i, s]) => {
        if (cancelled) return;
        setRuns(r.runs ?? []);
        setIncidents(i.incidents ?? []);
        setStats(s);
      },
      (err) => !cancelled && setError(err.message),
    );
    return () => {
      cancelled = true;
    };
  }, []);

  return { runs, incidents, stats, error };
}
