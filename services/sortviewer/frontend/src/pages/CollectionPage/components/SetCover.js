import { cn } from '@/lib/utils';

import { percent } from '../CollectionPage.logic';

/**
 * How much of a set there is: the solid part in the right colours, the faint part
 * counting any colour.
 */
export default function SetCover({ set, className }) {
  return (
    <span
      className={cn('relative block h-2 overflow-hidden rounded-full bg-accent', className)}
      aria-hidden="true"
    >
      <span
        className="absolute inset-y-0 left-0 bg-primary/30"
        style={{ width: `${percent(set.have_any_colour, set.need)}%` }}
      />
      <span
        className="absolute inset-y-0 left-0 bg-primary"
        style={{ width: `${percent(set.have, set.need)}%` }}
      />
    </span>
  );
}
