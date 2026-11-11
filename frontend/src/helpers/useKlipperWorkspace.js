import { useCallback, useEffect, useMemo, useState } from 'react';

import { useMCUSession } from '../context/MCUSessionContext';
import {
  deleteKlipperSection,
  getBoardDiagram,
  getBoardProfile,
  getKlipperWorkspace,
  listKlipperConfigFiles,
  loadKlipperConfig,
  saveKlipperConfig,
  updateKlipperRaw,
  updateKlipperSection,
} from './api';
import {
  configOptionsForSlot,
  configSectionName,
  findSectionForSlot,
  isOnOffSlot,
  slotStatesFromWorkspace,
} from './boardSlotConfig';
import { pca9685BoardsInConfig, pca9685Sections } from './pca9685Config';

export const BOARD_PROFILE_ID = 'ramps14';
// Bundled configs to start from when the backend has no workspace loaded yet.
const DEFAULT_CONFIGS = [
  'stepper-x-only.cfg',
  'ramps-all-steppers.cfg',
  'sample-ramps-minimal.cfg',
];

/**
 * The board's Klipper config being edited (the backend's workspace): the board profile and
 * diagram, each slot's wiring, the raw text, and saving. Machine › Board's Wiring and Config
 * file tabs and Diagnose › Motor bench (PCA9685 boards) work on it.
 */
export function useKlipperWorkspace() {
  const [profile, setProfile] = useState(null);
  const [diagram, setDiagram] = useState(null);
  const [slotStates, setSlotStates] = useState({});
  const [workspace, setWorkspace] = useState(null);
  const [rawText, setRawText] = useState('');
  const [configFiles, setConfigFiles] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const { connected, objectsBySlot } = useMCUSession();

  useEffect(() => {
    Promise.all([getBoardProfile(BOARD_PROFILE_ID), getBoardDiagram(BOARD_PROFILE_ID)]).then(
      ([p, d]) => {
        setProfile(p);
        setDiagram({ ...d, hotspots: d.hotspots || [] });
      },
      (err) => setError(err.message),
    );
  }, []);

  const sync = useCallback(
    (ws) => {
      setWorkspace(ws);
      setRawText(ws.raw || '');
      if (profile) setSlotStates(slotStatesFromWorkspace(profile, ws.merged_sections || []));
    },
    [profile],
  );

  // The workspace the backend has, else the first bundled default that loads.
  useEffect(() => {
    if (!profile) return;
    (async () => {
      try {
        sync(await getKlipperWorkspace());
        return;
      } catch {
        // none loaded yet
      }
      for (const path of DEFAULT_CONFIGS) {
        try {
          sync(await loadKlipperConfig(path));
          return;
        } catch {
          // try the next
        }
      }
    })();
  }, [profile, sync]);

  // Live MCU details on connected slots, without bringing back cleared ones.
  useEffect(() => {
    if (!connected || !profile?.slots) return;
    setSlotStates((prev) => {
      const next = { ...prev };
      for (const [slotId, obj] of Object.entries(objectsBySlot)) {
        if (!next[slotId]?.connected) continue;
        const slot = profile.slots[slotId] || {};
        next[slotId] = {
          ...next[slotId],
          connected: true,
          label: next[slotId].label || obj.label || slot.label || slotId,
          oid: obj.oid,
          kind: obj.kind,
          section: obj.section,
          type: slot.type,
        };
      }
      return next;
    });
  }, [connected, objectsBySlot, profile]);

  /** Runs a workspace change, keeping loading and error. Returns the new workspace or null. */
  const change = useCallback(
    async (fn) => {
      setLoading(true);
      setError('');
      try {
        const ws = await fn();
        if (ws) sync(ws);
        return ws ?? null;
      } catch (err) {
        setError(err.message);
        return null;
      } finally {
        setLoading(false);
      }
    },
    [sync],
  );

  const updateSlot = useCallback(
    async (slotId, patch) => {
      const slot = profile?.slots?.[slotId];
      if (!slot) return;
      if (patch.localOnly) {
        setSlotStates((prev) => ({
          ...prev,
          [slotId]: {
            connected: prev[slotId]?.connected ?? false,
            label: slot.label || slotId,
            type: slot.type,
            ...prev[slotId],
            ...patch,
          },
        }));
        return;
      }
      const existing = findSectionForSlot(slotId, slot, workspace?.merged_sections || []);
      let sectionName = configSectionName(slotId, slot);
      let options = configOptionsForSlot(slotId, slot, patch);
      if (isOnOffSlot(slot) && existing?.name.startsWith('output_pin ')) {
        // Keep its name (e.g. [output_pin leds], which the sorter looks for) and its
        // settings, apart from the safety timeout just chosen.
        sectionName = existing.name;
        // eslint-disable-next-line no-unused-vars
        const { maximum_mcu_duration: _old, ...kept } = existing.options;
        options = {
          ...kept,
          ...(options.maximum_mcu_duration
            ? { maximum_mcu_duration: options.maximum_mcu_duration }
            : {}),
        };
      }
      const ws = await change(async () => {
        let next = await updateKlipperSection(sectionName, options);
        if (existing && existing.name !== sectionName) {
          // An old [extruder]/[heater_bed]/[fan] on the same pin would clash with it.
          next = await deleteKlipperSection(existing.name);
        }
        return next;
      });
      if (ws) {
        setSlotStates((prev) => ({
          ...prev,
          [slotId]: {
            connected: true,
            label: patch.label || prev[slotId]?.label || slot.label || slotId,
            type: slot.type,
            section: sectionName,
          },
        }));
      }
    },
    [profile, workspace, change],
  );

  const clearSlot = useCallback(
    async (slotId) => {
      const slot = profile?.slots?.[slotId];
      if (!slot) return;
      const section = findSectionForSlot(slotId, slot, workspace?.merged_sections || []);
      if (section?.name) {
        await change(() => deleteKlipperSection(section.name));
      } else {
        setSlotStates((prev) => {
          const next = { ...prev };
          delete next[slotId];
          return next;
        });
      }
    },
    [profile, workspace, change],
  );

  /** Add (or replace) a PCA9685 board and its servo sections; used on the next connect. */
  const addPCA9685Board = async (board) => {
    const ws = await change(async () => {
      let next = null;
      for (const section of pca9685Sections(board)) {
        next = await updateKlipperSection(section.name, section.options);
      }
      return next;
    });
    return Boolean(ws);
  };

  const loadConfigFiles = useCallback(async () => {
    try {
      setConfigFiles((await listKlipperConfigFiles()).files || []);
    } catch (err) {
      setError(err.message);
    }
  }, []);

  return {
    profile,
    diagram,
    slotStates,
    workspace,
    dirty: Boolean(workspace?.dirty),
    rawText,
    setRawText,
    configFiles,
    loading,
    error,
    setError,
    updateSlot,
    clearSlot,
    addPCA9685Board,
    pcaBoards: useMemo(() => pca9685BoardsInConfig(workspace?.merged_sections), [workspace]),
    loadConfigFiles,
    loadConfigFile: (path) => change(() => loadKlipperConfig(path)),
    /** Saves to disk; returns the workspace, or null when it failed (error says why). */
    save: () => change(() => saveKlipperConfig()),
    applyRaw: () => change(() => updateKlipperRaw(rawText)),
  };
}
