import { useCallback } from 'react';

import { findSetting, getPath, setPath } from './settingsRegistry';
import { useMachineSettings } from './sorterSettings';
import { useTheme } from './theme';

/**
 * Reads and writes registry settings. `valueOf(setting)` is its current value (its
 * default when unset) and `set(setting, value)` changes it; machine settings save to the
 * server on their own, local ones (the theme) stay in this browser.
 */
export function useSettingValues() {
  const [doc, setDoc, syncError] = useMachineSettings();
  const { theme, setTheme } = useTheme();

  const valueOf = useCallback(
    (setting) => {
      if (setting.local) return theme;
      return getPath(doc, setting.key) ?? setting.default;
    },
    [doc, theme],
  );

  const set = useCallback(
    (setting, value) => {
      if (setting.local) setTheme(value);
      else setDoc((prev) => setPath(prev, setting.key, value));
    },
    [setDoc, setTheme],
  );

  return { valueOf, set, syncError };
}

/** One registry setting: { setting, value, set, reset, isDefault }. */
export function useSetting(key) {
  const setting = findSetting(key);
  if (!setting) throw new Error(`no setting ${key} in the registry`);
  const { valueOf, set } = useSettingValues();
  const value = valueOf(setting);
  return {
    setting,
    value,
    set: (v) => set(setting, v),
    reset: () => set(setting, setting.default),
    isDefault: value === setting.default,
  };
}
