import { ArrowRight, CircleCheck, CircleDashed, CircleHelp } from 'lucide-react';
import { Link } from 'react-router';

import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';

const ICONS = {
  done: { Icon: CircleCheck, className: 'text-running', label: 'Done' },
  todo: { Icon: CircleDashed, className: 'text-warning', label: 'To do' },
  unknown: { Icon: CircleHelp, className: 'text-idle', label: 'Not checked yet' },
};

/** The machine's setup items with live ticks; each links to where it's done. */
export default function SetupChecklist({ items, compact }) {
  return (
    <ol className="divide-y divide-border">
      {items.map((item) => {
        const { Icon, className, label } = ICONS[item.state];
        return (
          <li key={item.id}>
            <Link
              to={item.to}
              className={cn(
                'group flex items-start gap-3 py-3 hover:bg-accent/60',
                compact ? 'px-1' : 'px-5',
              )}
            >
              <Icon className={cn('mt-0.5 size-5 shrink-0', className)} aria-label={label} />
              <span className="min-w-0 flex-1">
                <span className="flex flex-wrap items-center gap-2">
                  <span className="font-medium">{item.label}</span>
                  {item.optional && <Badge variant="outline">Optional</Badge>}
                </span>
                <span className="block text-sm text-muted-foreground">{item.detail}</span>
              </span>
              <ArrowRight
                className="mt-0.5 size-4 shrink-0 text-muted-foreground transition-transform group-hover:translate-x-0.5"
                aria-hidden
              />
            </Link>
          </li>
        );
      })}
    </ol>
  );
}
