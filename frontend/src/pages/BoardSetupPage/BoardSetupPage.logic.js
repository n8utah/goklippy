import { useCallback, useEffect, useMemo, useState } from 'react';

import {
  deleteKlipperSection,
  getBoardDiagram,
  getBoardProfile,
  getKlipperWorkspace,
  getMCUPorts,
  listKlipperConfigFiles,
  loadKlipperConfig,
  saveKlipperConfig,
  scanMCUPorts,
  updateKlipperRaw,
  updateKlipperSection,
} from '../../helpers/api';
import {
  configOptionsForSlot,
  configSectionName,
  findSectionForSlot,
  slotStatesFromWorkspace,
} from '../../helpers/boardSlotConfig';
import { pca9685BoardsInConfig, pca9685Sections } from '../../helpers/pca9685Config';
import { useMCUSession } from '../../context/MCUSessionContext';

const PROFILE_ID = 'ramps14';
const LAST_PORT_STORAGE_KEY = 'goklippy_last_mcu_port';

/** Keep `preferred` if it is a sensible choice; otherwise pick the likeliest board. */
function pickPort(ports, preferred) {
  const klipperPorts = ports.filter((p) => p.is_klipper);
  const candidates = klipperPorts.length > 0 ? klipperPorts : ports;
  if (candidates.some((p) => p.name === preferred)) {
    return preferred;
  }
  return (candidates.find((p) => p.is_usb) || candidates[0])?.name ?? '';
}

export function useBoardSetupPageLogic() {
  const [tab, setTab] = useState('visual');
  const [profile, setProfile] = useState(null);
  const [diagram, setDiagram] = useState(null);
  const [slotStates, setSlotStates] = useState({});
  const [selectedHotspot, setSelectedHotspot] = useState(null);

  const [configFiles, setConfigFiles] = useState([]);
  const [workspace, setWorkspace] = useState(null);
  const [rawText, setRawText] = useState('');
  const [configLoading, setConfigLoading] = useState(false);
  const [configError, setConfigError] = useState('');

  const [ports, setPorts] = useState([]);
  const [selectedPort, setSelectedPort] = useState('');
  const [scanningPorts, setScanningPorts] = useState(false);
  const [scanMessage, setScanMessage] = useState('');
  const [resettingBoard, setResettingBoard] = useState(false);

  const {
    session,
    connected,
    connecting,
    connectStatus,
    wsConnected,
    error: sessionError,
    connect,
    disconnect,
    resetBoard,
    objectsBySlot,
    readingForSlot,
    setDigitalOut,
    setServo,
    jogStepper,
    stopStepper,
  } = useMCUSession();

  useEffect(() => {
    async function loadProfile() {
      try {
        const [p, d] = await Promise.all([getBoardProfile(PROFILE_ID), getBoardDiagram(PROFILE_ID)]);
        setProfile(p);
        setDiagram({
          ...d,
          hotspots: d.hotspots || [],
        });
      } catch (err) {
        setConfigError(err.message);
      }
    }
    loadProfile();
  }, []);

  const applyPorts = useCallback((nextPorts) => {
    setPorts(nextPorts);
    setSelectedPort((prev) => pickPort(nextPorts, prev || localStorage.getItem(LAST_PORT_STORAGE_KEY)));
  }, []);

  // Listing does not open the ports, so it is safe to run on load.
  useEffect(() => {
    if (connected) {
      return;
    }
    getMCUPorts()
      .then((data) => applyPorts(data.ports || []))
      .catch((err) => setConfigError(err.message));
  }, [connected, applyPorts]);

  const scanPorts = async () => {
    setScanningPorts(true);
    setScanMessage('');
    setConfigError('');
    try {
      const data = await scanMCUPorts();
      const scanned = data.ports || [];
      applyPorts(scanned);
      if (!scanned.some((p) => p.is_klipper)) {
        setScanMessage('No Klipper board responded. Check the USB cable and that nothing else is using the port.');
      }
    } catch (err) {
      setConfigError(err.message);
    } finally {
      setScanningPorts(false);
    }
  };

  const syncWorkspaceState = useCallback(
    (ws) => {
      setWorkspace(ws);
      setRawText(ws.raw || '');
      if (profile) {
        setSlotStates(slotStatesFromWorkspace(profile, ws.merged_sections || []));
      }
    },
    [profile],
  );

  useEffect(() => {
    if (!profile) {
      return;
    }
    async function loadWorkspace() {
      try {
        const ws = await getKlipperWorkspace();
        syncWorkspaceState(ws);
        return;
      } catch {
        // fall through to load a bundled default
      }
      for (const path of ['stepper-x-only.cfg', 'ramps-all-steppers.cfg', 'sample-ramps-minimal.cfg']) {
        try {
          const ws = await loadKlipperConfig(path);
          syncWorkspaceState(ws);
          return;
        } catch {
          // try next default
        }
      }
    }
    loadWorkspace();
  }, [profile, syncWorkspaceState]);

  // Enrich connected slots with live MCU metadata without overriding cleared slots.
  useEffect(() => {
    if (!connected || !profile?.slots) {
      return;
    }
    setSlotStates((prev) => {
      const next = { ...prev };
      for (const [slotId, obj] of Object.entries(objectsBySlot)) {
        if (!next[slotId]?.connected) {
          continue;
        }
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

  const loadConfigMeta = useCallback(async () => {
    try {
      const files = await listKlipperConfigFiles();
      setConfigFiles(files.files || []);
      try {
        const ws = await getKlipperWorkspace();
        syncWorkspaceState(ws);
      } catch {
        // no workspace loaded yet
      }
    } catch (err) {
      setConfigError(err.message);
    }
  }, [syncWorkspaceState]);

  useEffect(() => {
    if (tab === 'config') {
      loadConfigMeta();
    }
  }, [tab, loadConfigMeta]);

  const selectHotspot = useCallback((hotspot) => {
    setSelectedHotspot(hotspot);
  }, []);

  const updateSlot = useCallback(
    async (slotId, patch) => {
      const slot = profile?.slots?.[slotId];
      if (!slot) {
        return;
      }

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

      setConfigError('');
      try {
        const sectionName = configSectionName(slotId, slot);
        const options = configOptionsForSlot(slotId, slot, patch);
        const ws = await updateKlipperSection(sectionName, options);
        syncWorkspaceState(ws);
        setSlotStates((prev) => ({
          ...prev,
          [slotId]: {
            connected: true,
            label: patch.label || prev[slotId]?.label || slot.label || slotId,
            type: slot.type,
            section: sectionName,
          },
        }));
      } catch (err) {
        setConfigError(err.message);
      }
    },
    [profile, syncWorkspaceState],
  );

  const clearSlot = useCallback(
    async (slotId) => {
      const slot = profile?.slots?.[slotId];
      if (!slot) {
        return;
      }

      setConfigError('');
      try {
        const merged = workspace?.merged_sections || [];
        const section = findSectionForSlot(slotId, slot, merged);
        if (section?.name) {
          const ws = await deleteKlipperSection(section.name);
          syncWorkspaceState(ws);
        } else {
          setSlotStates((prev) => {
            const next = { ...prev };
            delete next[slotId];
            return next;
          });
        }
      } catch (err) {
        setConfigError(err.message);
      }
    },
    [profile, workspace, syncWorkspaceState],
  );

  /** Add (or replace) a PCA9685 board and its servo sections; used on the next connect. */
  const addPCA9685Board = async (board) => {
    setConfigLoading(true);
    setConfigError('');
    try {
      let ws = null;
      for (const section of pca9685Sections(board)) {
        ws = await updateKlipperSection(section.name, section.options);
      }
      if (ws) syncWorkspaceState(ws);
      return true;
    } catch (err) {
      setConfigError(err.message);
      return false;
    } finally {
      setConfigLoading(false);
    }
  };

  const pcaBoards = useMemo(() => pca9685BoardsInConfig(workspace?.merged_sections), [workspace]);

  const loadConfigFile = async (path) => {
    setConfigLoading(true);
    setConfigError('');
    try {
      const ws = await loadKlipperConfig(path);
      syncWorkspaceState(ws);
    } catch (err) {
      setConfigError(err.message);
    } finally {
      setConfigLoading(false);
    }
  };

  const saveConfig = async () => {
    setConfigLoading(true);
    setConfigError('');
    try {
      const ws = await saveKlipperConfig();
      syncWorkspaceState(ws);
      return ws;
    } catch (err) {
      setConfigError(err.message);
      throw err;
    } finally {
      setConfigLoading(false);
    }
  };

  const applyRawConfig = async () => {
    setConfigLoading(true);
    setConfigError('');
    try {
      const ws = await updateKlipperRaw(rawText);
      syncWorkspaceState(ws);
    } catch (err) {
      setConfigError(err.message);
    } finally {
      setConfigLoading(false);
    }
  };

  const handleConnect = async (port) => {
    setConfigError('');
    if (workspace?.dirty) {
      const shouldSave = window.confirm(
        'You have unsaved wiring changes.\n\nOK — save to disk, then connect.\nCancel — connect using current in-memory wiring (without saving).',
      );
      if (shouldSave) {
        try {
          await saveConfig();
        } catch {
          return;
        }
      }
    }
    try {
      await connect(port, { useWorkspace: true, profileId: PROFILE_ID });
      localStorage.setItem(LAST_PORT_STORAGE_KEY, port);
    } catch (err) {
      setConfigError(err.message);
    }
  };

  const handleResetBoard = async () => {
    setResettingBoard(true);
    setConfigError('');
    try {
      await resetBoard();
    } catch (err) {
      setConfigError(err.message);
    } finally {
      setResettingBoard(false);
    }
  };

  const selectedHotspotMemo = useMemo(() => selectedHotspot, [selectedHotspot]);

  return {
    tab,
    setTab,
    profile,
    diagram,
    slotStates,
    selectedHotspot: selectedHotspotMemo,
    selectHotspot,
    updateSlot,
    clearSlot,
    configFiles,
    workspace,
    configError,
    configLoading,
    rawText,
    setRawText,
    loadConfigFile,
    saveConfig,
    applyRawConfig,
    addPCA9685Board,
    pcaBoards,
    session,
    connected,
    connecting,
    connectStatus,
    wsConnected,
    sessionError,
    ports,
    selectedPort,
    setSelectedPort,
    scanningPorts,
    scanMessage,
    scanPorts,
    handleConnect,
    disconnect,
    resettingBoard,
    handleResetBoard,
    objectsBySlot,
    readingForSlot,
    setDigitalOut,
    setServo,
    jogStepper,
    stopStepper,
  };
}
