import { Download, RefreshCw, Trash2 } from 'lucide-react';
import { useState } from 'react';

import ConfirmClear from '@/components/ConfirmClear/ConfirmClear';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { cn } from '@/lib/utils';

import { LEVELS, RANGES, useLogsPageLogic } from './LogsPage.logic';

const LEVEL_BADGE = { ERROR: 'destructive', WARN: 'warning', INFO: 'idle', DEBUG: 'idle' };

/** The time of day, with the date when it isn't today. */
function stamp(t) {
  const d = new Date(t);
  const time = d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
  const ms = String(d.getMilliseconds()).padStart(3, '0');
  const today = d.toDateString() === new Date().toDateString();
  return `${today ? '' : `${d.toLocaleDateString([], { month: 'short', day: 'numeric' })} `}${time}.${ms}`;
}

// Shown last and dimmer: they're for matching lines up, not reading.
const QUIET = new Set(['request_id', 'client_ip', 'duration_ms']);

function Attrs({ attrs }) {
  if (!attrs) return null;
  const keys = Object.keys(attrs).sort((a, b) => QUIET.has(a) - QUIET.has(b) || a.localeCompare(b));
  return (
    <span className="flex flex-wrap gap-x-3 gap-y-0.5">
      {keys.map((k) => (
        <span
          key={k}
          className={cn(
            'break-all',
            QUIET.has(k) ? 'text-muted-foreground/70' : 'text-muted-foreground',
          )}
        >
          {k}=<span className={QUIET.has(k) ? '' : 'text-foreground'}>{attrs[k]}</span>
        </span>
      ))}
    </span>
  );
}

function Pick({ label, value, onChange, options, className }) {
  return (
    <label className="flex flex-col gap-1.5 text-sm">
      <span className="font-medium">{label}</span>
      <Select value={value} onValueChange={onChange}>
        <SelectTrigger className={className}>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {options.map((o) => (
            <SelectItem key={o.value} value={o.value}>
              {o.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </label>
  );
}

/**
 * Diagnose › Logs: what the backend did over the last 24 hours (runs, pieces, jobs,
 * incidents, the board, cameras, actions from the pages and anything that went wrong),
 * newest first, following live.
 */
const LogsPage = () => {
  const logic = useLogsPageLogic();
  const { filters } = logic;
  const [confirm, setConfirm] = useState(null);

  return (
    <div className="ds flex flex-col gap-4">
      {logic.error && <p className="text-sm text-destructive">{logic.error}</p>}
      <section className="rounded-xl border border-border bg-card">
        <div className="grid grid-cols-2 items-end gap-3 border-b border-border px-4 py-4 sm:flex sm:flex-wrap sm:px-5">
          <label htmlFor="logs-q" className="col-span-2 flex flex-col gap-1.5 text-sm">
            <span className="font-medium">Find</span>
            <Input
              id="logs-q"
              className="w-full sm:w-64"
              placeholder="Text, piece id, channel, c3…"
              value={filters.q}
              onChange={(e) => logic.setFilter('q', e.target.value)}
            />
          </label>
          <Pick
            label="Show"
            value={filters.level}
            onChange={(v) => logic.setFilter('level', v)}
            options={LEVELS}
            className="w-full sm:w-52"
          />
          <Pick
            label="From"
            value={filters.range}
            onChange={(v) => logic.setFilter('range', v)}
            options={RANGES}
            className="w-full sm:w-40"
          />
          <label className="col-span-2 flex h-10 items-center gap-2 text-sm sm:ml-auto">
            <Switch checked={logic.live} onCheckedChange={logic.setLive} />
            Follow live
          </label>
        </div>

        <div className="flex flex-wrap items-center gap-1 border-b border-border px-4 py-2.5 sm:px-5">
          <span className="mr-auto text-sm text-muted-foreground">
            {logic.loading
              ? 'Loading…'
              : `${logic.entries.length.toLocaleString()} line${logic.entries.length === 1 ? '' : 's'}${logic.more ? ' (more further back)' : ''}`}
          </span>
          <Button variant="ghost" size="sm" onClick={logic.refresh}>
            <RefreshCw /> Refresh
          </Button>
          <Button variant="ghost" size="sm" asChild>
            <a href={logic.downloadURL} download>
              <Download /> Download
            </a>
          </Button>
          <Button
            variant="ghost"
            size="sm"
            className="text-destructive"
            onClick={() =>
              setConfirm({
                title: 'Clear the logs?',
                text: 'This removes every kept log line. New ones are kept as usual.',
                confirm: 'Clear logs',
                action: logic.clear,
              })
            }
          >
            <Trash2 /> Clear
          </Button>
        </div>

        {!logic.loading && logic.entries.length === 0 ? (
          <p className="px-5 py-6 text-sm text-muted-foreground">
            Nothing logged {filters.q || filters.level !== 'info' ? 'that matches ' : ''}in this
            time. Logs are kept for 24 hours.
          </p>
        ) : (
          <ol className="divide-y divide-border font-mono text-xs">
            {logic.entries.map((e, i) => (
              <li
                key={`${e.time}-${i}`}
                className={cn(
                  'flex flex-col gap-1 px-4 py-2 sm:flex-row sm:gap-3 sm:px-5',
                  e.level === 'ERROR' && 'bg-destructive/5',
                  e.level === 'WARN' && 'bg-warning-bg/40',
                )}
              >
                <span className="flex shrink-0 items-start gap-2 sm:w-52">
                  <span className="text-muted-foreground">{stamp(e.time)}</span>
                  <Badge
                    variant={LEVEL_BADGE[e.level] ?? 'idle'}
                    className="px-1.5 py-0 text-[10px]"
                  >
                    {e.level}
                  </Badge>
                </span>
                <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                  <span className="font-sans text-sm text-foreground">{e.msg}</span>
                  <Attrs attrs={e.attrs} />
                </span>
              </li>
            ))}
          </ol>
        )}

        {logic.more && (
          <div className="border-t border-border px-5 py-3">
            <Button variant="outline" size="sm" onClick={logic.loadOlder}>
              Older
            </Button>
          </div>
        )}
      </section>
      <ConfirmClear what={confirm} onClose={() => setConfirm(null)} />
    </div>
  );
};

export default LogsPage;
