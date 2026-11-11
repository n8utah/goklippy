import { useCallback, useEffect, useState } from 'react';
import { toast } from 'sonner';

import { useMCUSession } from '../../context/MCUSessionContext';
import { getMCUPorts, scanMCUPorts } from '../../helpers/api';
import { BOARD_PROFILE_ID, useKlipperWorkspace } from '../../helpers/useKlipperWorkspace';

const LAST_PORT_KEY = 'goklippy_last_mcu_port';

/** Machine › Board: the connection, the wiring and the config file, on one workspace. */
export function useBoardPageLogic() {
  const config = useKlipperWorkspace();
  const mcu = useMCUSession();
  const [ports, setPorts] = useState([]);
  const [scannedAt, setScannedAt] = useState(null);
  const [scanning, setScanning] = useState(false);
  const [portsError, setPortsError] = useState('');
  const [resetting, setResetting] = useState(false);
  // A port waiting on the unsaved-wiring question before connecting.
  const [pendingPort, setPendingPort] = useState(null);
  // The pin picked on the Wiring tab's board.
  const [selectedHotspot, selectHotspot] = useState(null);

  const applyPorts = (data) => {
    setPorts(data.ports || []);
    setScannedAt(data.scanned_at || null);
  };

  // Listing doesn't open the ports, so it's safe on load; scanning does.
  useEffect(() => {
    getMCUPorts().then(applyPorts, (err) => setPortsError(err.message));
  }, [mcu.connected]);

  const scan = async () => {
    setScanning(true);
    setPortsError('');
    try {
      const data = await scanMCUPorts();
      applyPorts(data);
      if (!(data.ports || []).some((p) => p.is_klipper)) {
        setPortsError(
          'No Klipper board answered. Check the USB cable and that nothing else has the port open.',
        );
      }
    } catch (err) {
      setPortsError(err.message);
    } finally {
      setScanning(false);
    }
  };

  const connectNow = useCallback(
    async (port) => {
      try {
        await mcu.connect(port, { useWorkspace: true, profileId: BOARD_PROFILE_ID });
        localStorage.setItem(LAST_PORT_KEY, port);
        toast.success(`Connected to ${port}`);
      } catch {
        // the session's error says why
      }
    },
    [mcu],
  );

  /** Connect, asking first when the wiring has unsaved changes. */
  const connect = (port) => {
    if (config.dirty) setPendingPort(port);
    else connectNow(port);
  };

  /** The answer to the unsaved-wiring question: 'save', 'keep' (connect without) or cancel. */
  const answerPending = async (answer) => {
    const port = pendingPort;
    setPendingPort(null);
    if (!port || answer === 'cancel') return;
    if (answer === 'save' && !(await config.save())) return;
    connectNow(port);
  };

  const reset = async () => {
    setResetting(true);
    try {
      await mcu.resetBoard();
      toast('Board restarted; every motor has stopped.');
    } catch (err) {
      toast.error(err.message);
    } finally {
      setResetting(false);
    }
  };

  return {
    config,
    mcu,
    ports,
    scannedAt,
    scanning,
    portsError,
    scan,
    connect,
    pendingPort,
    answerPending,
    reset,
    resetting,
    selectedHotspot,
    selectHotspot,
    lastPort: (() => {
      try {
        return localStorage.getItem(LAST_PORT_KEY);
      } catch {
        return null;
      }
    })(),
  };
}
