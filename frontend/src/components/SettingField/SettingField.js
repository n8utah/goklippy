import { Badge } from '@/components/ui/badge';
import { formatSettingValue } from '@/helpers/settingsRegistry';
import { useSetting } from '@/helpers/useSetting';
import { cn } from '@/lib/utils';

import SettingInput from './SettingInput';

/**
 * One registry setting as a row: label, help, its control, and "Reset to …" once it's
 * changed. The same row on System › Settings and inline on any page; changes save on their
 * own.
 */
export default function SettingField({ settingKey, className }) {
  const { setting, value, set, reset, isDefault } = useSetting(settingKey);
  const id = `setting-${setting.key}`;

  return (
    <div
      className={cn(
        'flex flex-col gap-3 py-4 sm:flex-row sm:items-start sm:justify-between sm:gap-8',
        className,
      )}
    >
      <div className="min-w-0 sm:max-w-[60ch]">
        <div className="flex flex-wrap items-center gap-2">
          <label htmlFor={id} className="text-[15px] font-medium">
            {setting.label}
          </label>
          {!isDefault && <Badge variant="outline">Changed</Badge>}
          {setting.stopped && <Badge variant="idle">Change while stopped</Badge>}
        </div>
        {setting.help && <p className="mt-0.5 text-sm text-muted-foreground">{setting.help}</p>}
      </div>
      <div className="flex shrink-0 flex-col items-start gap-1 sm:items-end">
        <SettingInput id={id} setting={setting} value={value} onChange={set} />
        {!isDefault && (
          <button
            type="button"
            onClick={reset}
            className="text-sm font-medium text-primary-text hover:underline"
          >
            Reset to {formatSettingValue(setting, setting.default) || 'empty'}
          </button>
        )}
      </div>
    </div>
  );
}
