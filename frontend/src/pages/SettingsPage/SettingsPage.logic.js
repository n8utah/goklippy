import { ALL_SETTINGS, SETTING_SECTIONS } from '../../helpers/settingsRegistry';
import { useSorterState } from '../../helpers/sorterState';
import { useSettingValues } from '../../helpers/useSetting';

/** System › Settings: one registry section at a time, with resetting it whole. */
export function useSettingsPageLogic(sectionId) {
  const section = SETTING_SECTIONS.find((s) => s.id === sectionId) || SETTING_SECTIONS[0];
  const settings = ALL_SETTINGS.filter((s) => s.section.id === section.id);
  const { valueOf, set, syncError } = useSettingValues();
  const { state } = useSorterState();

  const changed = settings.filter((s) => valueOf(s) !== s.default);
  const resetSection = () => changed.forEach((s) => set(s, s.default));

  return {
    section,
    settings,
    changedCount: changed.length,
    resetSection,
    syncError,
    supply: state?.supply,
  };
}
