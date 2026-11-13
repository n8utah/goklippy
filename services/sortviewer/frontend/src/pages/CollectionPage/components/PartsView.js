import { useState } from 'react';

import { Input } from '@/components/ui/input';
import { photoURL } from '@/helpers/api';

import { Pic } from './PieceTile';

/** Every part and colour found, most first, with a search. */
export default function PartsView({ parts }) {
  const [q, setQ] = useState('');
  if (!parts) return <p className="text-sm text-muted-foreground">Loading…</p>;
  const needle = q.trim().toLowerCase();
  const shown = needle
    ? parts.filter((p) =>
        [p.part_id, p.part_name, p.color_name, p.part_category].some((v) =>
          v?.toLowerCase().includes(needle),
        ),
      )
    : parts;
  return (
    <div className="flex flex-col gap-3">
      <Input
        type="search"
        aria-label="Find a part"
        placeholder="Find a part, colour or number"
        value={q}
        onChange={(e) => setQ(e.target.value)}
        className="sm:w-80"
      />
      {shown.length === 0 ? (
        <p className="text-sm text-muted-foreground">
          {parts.length ? 'No parts match.' : 'No parts found yet.'}
        </p>
      ) : (
        <ul className="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6">
          {shown.map((p) => (
            <li
              key={`${p.part_id}/${p.color_id}`}
              className="flex flex-col gap-2 rounded-xl border border-border bg-card p-2"
            >
              <span className="relative">
                <Pic
                  src={p.img_url || (p.photo_piece_id ? photoURL(p.photo_piece_id) : '')}
                  alt={p.part_name}
                  className="aspect-square bg-white"
                />
                <span className="absolute top-1.5 right-1.5 rounded-full bg-primary px-2 py-0.5 font-mono text-xs font-semibold text-primary-foreground">
                  ×{p.count.toLocaleString()}
                </span>
              </span>
              <span className="min-w-0 px-0.5 text-sm">
                <span className="line-clamp-2 font-medium" title={p.part_name}>
                  {p.part_name || p.part_id}
                </span>
                <span className="block truncate text-xs text-muted-foreground">
                  <span className="font-mono">{p.part_id}</span>
                  {p.color_name && ` · ${p.color_name}`}
                </span>
              </span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
