import { useState } from 'react';
import { Link } from 'react-router';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { findSetting, formatSettingValue } from '@/helpers/settingsRegistry';
import { useSettingValues } from '@/helpers/useSetting';

import SettingInput from '../SettingField/SettingInput';

/**
 * Edits one setting in a dialog, from search or a page: change it, then Save or Cancel.
 * `settingKey` null keeps it closed.
 */
export default function SettingQuickEditor({ settingKey, onClose }) {
  const setting = settingKey ? findSetting(settingKey) : null;
  return (
    <Dialog open={Boolean(setting)} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-md">
        {/* Keyed, so each opening starts from the saved value. */}
        {setting && <Editor key={setting.key} setting={setting} onClose={onClose} />}
      </DialogContent>
    </Dialog>
  );
}

function Editor({ setting, onClose }) {
  const { valueOf, set } = useSettingValues();
  const saved = valueOf(setting);
  const [draft, setDraft] = useState(saved);
  const [problem, setProblem] = useState('');
  const id = `quick-${setting.key}`;

  const save = () => {
    set(setting, draft);
    onClose();
  };

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        if (!problem) save();
      }}
      className="flex flex-col gap-4"
    >
      <DialogHeader>
        <DialogTitle className="font-display">{setting.label}</DialogTitle>
        <DialogDescription>
          {setting.help || `Settings › ${setting.section.label}`}
        </DialogDescription>
      </DialogHeader>

      <div className="flex flex-wrap items-center justify-between gap-3">
        <SettingInput
          id={id}
          setting={setting}
          value={draft}
          onChange={setDraft}
          onProblem={setProblem}
          autoFocus
        />
        {setting.local ? null : setting.stopped ? (
          <Badge variant="warning">Change while stopped</Badge>
        ) : (
          <Badge variant="running">Applies while sorting</Badge>
        )}
      </div>

      <div className="flex flex-wrap items-center justify-between gap-2 text-sm text-muted-foreground">
        <span>
          Default {formatSettingValue(setting, setting.default) || 'empty'}
          {draft !== setting.default && (
            <>
              {' · '}
              <button
                type="button"
                className="font-medium text-primary-text hover:underline"
                onClick={() => setDraft(setting.default)}
              >
                Use the default
              </button>
            </>
          )}
        </span>
        <Link
          to={`/system/settings/${setting.section.id}`}
          onClick={onClose}
          className="font-medium text-primary-text hover:underline"
        >
          All {setting.section.label} settings
        </Link>
      </div>

      <DialogFooter>
        <Button type="button" variant="outline" onClick={onClose}>
          Cancel
        </Button>
        <Button type="submit" disabled={Boolean(problem) || draft === saved}>
          Save
        </Button>
      </DialogFooter>
    </form>
  );
}
