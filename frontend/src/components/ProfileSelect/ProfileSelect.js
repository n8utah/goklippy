import { useEffect, useState } from 'react';

import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { listSortingProfiles } from '@/helpers/api';

/** The saved sorting profiles: built-ins first, then uploads. */
export function useSortingProfiles() {
  const [profiles, setProfiles] = useState(null);
  const [error, setError] = useState('');
  useEffect(() => {
    let cancelled = false;
    listSortingProfiles().then(
      (data) => !cancelled && setProfiles(data.profiles ?? []),
      (err) => !cancelled && setError(err.message),
    );
    return () => {
      cancelled = true;
    };
  }, []);
  return { profiles, error };
}

/**
 * Picks a sorting profile: `value` is its id (a number, or null). `profiles` comes from
 * useSortingProfiles.
 */
export default function ProfileSelect({ id, profiles, value, onChange, disabled, className }) {
  const builtins = (profiles ?? []).filter((p) => p.source === 'builtin');
  const uploads = (profiles ?? []).filter((p) => p.source !== 'builtin');
  return (
    <Select
      value={value ? String(value) : ''}
      onValueChange={(v) => onChange(Number(v))}
      disabled={disabled || !profiles}
    >
      <SelectTrigger id={id} className={className ?? 'w-full'}>
        <SelectValue placeholder={profiles ? 'Pick a profile' : 'Loading…'} />
      </SelectTrigger>
      <SelectContent>
        <SelectGroup>
          <SelectLabel>Built in</SelectLabel>
          {builtins.map((p) => (
            <SelectItem key={p.id} value={String(p.id)}>
              {p.name}
            </SelectItem>
          ))}
        </SelectGroup>
        {uploads.length > 0 && (
          <SelectGroup>
            <SelectLabel>Uploaded from Hive</SelectLabel>
            {uploads.map((p) => (
              <SelectItem key={p.id} value={String(p.id)}>
                {p.name}
              </SelectItem>
            ))}
          </SelectGroup>
        )}
      </SelectContent>
    </Select>
  );
}
