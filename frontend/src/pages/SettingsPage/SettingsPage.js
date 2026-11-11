import { useParams } from 'react-router';

import SettingField from '@/components/SettingField/SettingField';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';

import { useSettingsPageLogic } from './SettingsPage.logic';

/**
 * System › Settings: every registry setting, a section per part tab. Changes save on their
 * own and reach a running sort straight away.
 */
const SettingsPage = () => {
  const { section: sectionId } = useParams();
  const logic = useSettingsPageLogic(sectionId);
  const { section } = logic;

  return (
    <div className="ds flex flex-col gap-4">
      {logic.syncError && (
        <p
          role="alert"
          className="rounded-lg border border-warning/40 bg-warning-bg px-4 py-3 text-sm text-warning"
        >
          {logic.syncError}
        </p>
      )}
      <Card className="gap-0 py-0">
        <CardContent className="px-5 sm:px-6">
          <div className="flex flex-col gap-3 border-b border-border py-5 sm:flex-row sm:items-start sm:justify-between">
            <div className="max-w-[70ch]">
              <h2 className="font-display text-xl font-semibold">{section.label}</h2>
              <p className="mt-1 text-sm text-muted-foreground">{section.about}</p>
              {section.id === 'power' && <SupplyNow supply={logic.supply} />}
            </div>
            {logic.changedCount > 0 && (
              <Button variant="outline" onClick={logic.resetSection} className="shrink-0">
                Reset {logic.changedCount} {logic.changedCount === 1 ? 'change' : 'changes'}
              </Button>
            )}
          </div>
          <div className="divide-y divide-border">
            {logic.settings.map((s) => (
              <SettingField key={s.key} settingKey={s.key} />
            ))}
          </div>
        </CardContent>
      </Card>
    </div>
  );
};

/** The supply's reading now, to check the divider settings against a meter. */
function SupplyNow({ supply }) {
  return (
    <p className="mt-2 text-sm">
      Now:{' '}
      {supply?.known ? (
        <span className="font-mono font-medium">{supply.volts.toFixed(2)} V</span>
      ) : (
        <span className="text-muted-foreground">not measured</span>
      )}
    </p>
  );
}

export default SettingsPage;
