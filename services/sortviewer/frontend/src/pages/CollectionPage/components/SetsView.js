import { RefreshCw } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';

import { coverText, SET_SIZES, setShare } from '../CollectionPage.logic';
import { Pic } from './PieceTile';
import SetCover from './SetCover';
import SetDetail from './SetDetail';

/** A row of buttons where one is on. */
function Choice({ label, options, value, onChange }) {
  return (
    <div role="group" aria-label={label} className="inline-flex rounded-lg bg-muted p-[3px]">
      {options.map((o) => (
        <button
          key={String(o.value)}
          type="button"
          aria-pressed={o.value === value}
          onClick={() => onChange(o.value)}
          className={cn(
            'rounded-md px-2.5 py-1.5 text-sm font-medium whitespace-nowrap',
            o.value === value
              ? 'bg-card text-foreground shadow-xs'
              : 'text-muted-foreground hover:text-foreground',
          )}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}

function SetCard({ set, any, onOpen }) {
  return (
    <button
      type="button"
      onClick={() => onOpen(set.set_num)}
      className="flex w-full flex-col gap-2 rounded-xl border border-border bg-card p-2 text-left transition-colors hover:border-border-strong focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
    >
      <Pic src={set.img_url} alt="" className="aspect-[4/3] bg-white" />
      <span className="flex min-w-0 flex-col gap-1.5 px-0.5 pb-0.5">
        <span className="flex items-baseline justify-between gap-2">
          <span className="line-clamp-2 font-medium" title={set.name}>
            {set.name}
          </span>
          <span className="font-mono text-lg font-semibold text-primary-text">
            {setShare(set, any)}%
          </span>
        </span>
        <span className="truncate text-xs text-muted-foreground">
          <span className="font-mono">{set.set_num}</span> · {set.year}
          {set.theme && ` · ${set.theme}`}
        </span>
        <SetCover set={set} />
        <span className="text-xs text-muted-foreground">{coverText(set, any)}</span>
      </span>
    </button>
  );
}

/** What the set data knows of this collection, and when it's from. */
function CoverageNote({ result }) {
  const cov = result.coverage;
  const unknown = cov.pieces - cov.known;
  const built = result.built_at ? new Date(result.built_at).toLocaleDateString() : '';
  return (
    <p className="text-sm text-muted-foreground">
      Your {cov.pieces.toLocaleString()} identified pieces, matched against every set’s parts from
      Rebrickable{built && ` (updated ${built})`}. Spare parts, stickers and minifigures aren’t
      counted.
      {unknown > 0 && (
        <>
          {' '}
          {unknown.toLocaleString()} {unknown === 1 ? 'piece is a part' : 'pieces are parts'} the
          set data numbers differently:{' '}
          <span className="font-mono">
            {cov.unknown
              .slice(0, 5)
              .map((u) => `${u.part_num} ×${u.pieces}`)
              .join(', ')}
          </span>
          {cov.unknown.length > 5 && '…'}
        </>
      )}
    </p>
  );
}

/** The Sets tab: the sets this collection covers best, and one set's parts. */
export default function SetsView({ sets }) {
  const r = sets.result;
  return (
    <section className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center gap-2">
        <Input
          type="search"
          aria-label="Find a set"
          placeholder="Find a set, number or theme"
          value={sets.q}
          onChange={(e) => sets.setQ(e.target.value)}
          className="sm:w-72"
        />
        <Choice
          label="Set size"
          options={SET_SIZES.map((s) => ({ value: s.min, label: s.label }))}
          value={sets.min}
          onChange={sets.setMin}
        />
        <Choice
          label="Colours"
          options={[
            { value: false, label: 'Right colours' },
            { value: true, label: 'Any colour' },
          ]}
          value={sets.any}
          onChange={sets.setAny}
        />
        <Button
          variant="ghost"
          size="sm"
          onClick={sets.refresh}
          disabled={sets.loading}
          className="ml-auto"
        >
          <RefreshCw className={cn(sets.loading && 'animate-spin')} /> Refresh
        </Button>
      </div>

      {sets.error && (
        <p className="rounded-xl border border-border bg-card p-5 text-sm text-muted-foreground">
          {sets.error}
        </p>
      )}
      {!r && !sets.error && (
        <p className="text-sm text-muted-foreground">Matching against every set…</p>
      )}
      {r && (
        <>
          <CoverageNote result={r} />
          {r.sets.length === 0 ? (
            <p className="rounded-xl border border-border bg-card p-5 text-sm text-muted-foreground">
              {r.coverage.known
                ? 'No sets match. Try a smaller set size or another search.'
                : 'Nothing to match yet: no identified pieces the set data knows.'}
            </p>
          ) : (
            <ul
              className={cn(
                'grid grid-cols-2 gap-3 transition-opacity sm:grid-cols-3 lg:grid-cols-4',
                sets.loading && 'opacity-60',
              )}
            >
              {r.sets.map((s) => (
                <li key={s.set_num}>
                  <SetCard set={s} any={sets.any} onOpen={sets.openSet} />
                </li>
              ))}
            </ul>
          )}
          <div className="flex flex-wrap items-center gap-3 text-sm text-muted-foreground">
            {r.total > 0 && (
              <span>
                {r.sets.length.toLocaleString()} of {r.total.toLocaleString()}{' '}
                {r.total === 1 ? 'set' : 'sets'}
              </span>
            )}
            {sets.canShowMore && (
              <Button variant="outline" size="sm" onClick={sets.showMore} disabled={sets.loading}>
                Show more
              </Button>
            )}
          </div>
        </>
      )}
      <SetDetail
        open={Boolean(sets.open)}
        detail={sets.detail}
        any={sets.any}
        onClose={() => sets.openSet('')}
      />
    </section>
  );
}
