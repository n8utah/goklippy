import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';

/** Joins class names, letting later Tailwind classes win over earlier ones (shadcn's helper). */
export function cn(...inputs) {
  return twMerge(clsx(inputs));
}
