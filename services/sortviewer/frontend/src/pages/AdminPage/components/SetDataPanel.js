import { RefreshCw } from 'lucide-react';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';

/** The set data behind each collection's Sets tab: when it was built, and rebuilding it. */
export default function SetDataPanel({ setData }) {
  const st = setData.status;
  return (
    <section className="flex flex-col gap-3">
      <p className="text-sm text-muted-foreground">
        Every LEGO set’s parts, from Rebrickable’s downloads, for the Sets tab. It’s fetched when
        the site first starts and rebuilt by itself every month.
      </p>
      <div className="flex flex-wrap items-center gap-3 rounded-xl border border-border bg-card px-4 py-3">
        <div className="min-w-0 flex-1">
          <p className="flex items-center gap-2 font-medium">
            {!st ? 'Loading…' : st.ready ? `${st.sets.toLocaleString()} sets` : 'Not built yet'}
            {st?.building && <Badge variant="running">Building</Badge>}
          </p>
          {st?.built_at && (
            <p className="text-sm text-muted-foreground">
              Built {new Date(st.built_at).toLocaleString()}
            </p>
          )}
          {st?.error && <p className="text-sm text-destructive">Last rebuild failed: {st.error}</p>}
        </div>
        <Button variant="outline" onClick={setData.refresh} disabled={!st || st.building}>
          <RefreshCw /> Rebuild now
        </Button>
      </div>
    </section>
  );
}
