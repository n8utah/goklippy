import { ArrowLeft, ArrowRight, ChevronDown, Waves } from 'lucide-react';
import { useState } from 'react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { cn } from '@/lib/utils';

const CHANNELS = ['c1', 'c2', 'c3', 'c4'];

/**
 * Turn a channel by hand or shake it to free a stuck piece. Folded away while sorting;
 * the controls say why when they can't be used.
 */
export default function ChannelControls({ shakes, disabled, why, sorting, onTurn, onShake }) {
  const [open, setOpen] = useState(!sorting);
  const [amount, setAmount] = useState(10);
  const [shake, setShake] = useState('heavy');
  // Folding away when a sort starts; the person can still open it.
  const [wasSorting, setWasSorting] = useState(sorting);
  if (sorting !== wasSorting) {
    setWasSorting(sorting);
    if (sorting) setOpen(false);
  }

  return (
    <section className="rounded-xl border border-border bg-card">
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        aria-expanded={open}
        className="flex w-full items-center justify-between gap-3 px-4 py-3 text-left"
      >
        <span>
          <span className="block font-display font-semibold">Turn or shake a channel</span>
          <span className="block text-sm text-muted-foreground">
            Moves the real channel at once{why ? ` · ${why}` : ''}
          </span>
        </span>
        <ChevronDown
          className={cn('size-5 shrink-0 transition-transform', open && 'rotate-180')}
          aria-hidden
        />
      </button>
      {open && (
        <div className="flex flex-col gap-4 border-t border-border px-4 py-4">
          <div className="flex flex-wrap items-center gap-4">
            <label className="flex items-center gap-2 text-sm">
              Turn by
              <Input
                type="number"
                min={0.1}
                max={360}
                step={1}
                value={amount}
                className="w-20 text-right font-mono"
                onChange={(e) => Number(e.target.value) > 0 && setAmount(Number(e.target.value))}
              />
              °
            </label>
            <label className="flex items-center gap-2 text-sm">
              Shake
              <Select value={shake} onValueChange={setShake}>
                <SelectTrigger className="w-44">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {shakes.map((s) => (
                    <SelectItem key={s.key} value={s.key}>
                      {s.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </label>
          </div>
          <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-4">
            {CHANNELS.map((ch) => (
              <div
                key={ch}
                className="flex items-center gap-2 rounded-lg border border-border px-3 py-2"
              >
                <span className="w-7 font-medium">{ch.toUpperCase()}</span>
                <Button
                  variant="outline"
                  size="icon-sm"
                  disabled={disabled}
                  onClick={() => onTurn(ch, -amount)}
                  aria-label={`Turn ${ch.toUpperCase()} back ${amount}°`}
                >
                  <ArrowLeft />
                </Button>
                <Button
                  variant="outline"
                  size="icon-sm"
                  disabled={disabled}
                  onClick={() => onTurn(ch, amount)}
                  aria-label={`Turn ${ch.toUpperCase()} forward ${amount}°`}
                >
                  <ArrowRight />
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  disabled={disabled}
                  onClick={() => onShake(ch, shake)}
                  className="ml-auto"
                >
                  <Waves />
                  Shake
                </Button>
              </div>
            ))}
          </div>
        </div>
      )}
    </section>
  );
}
