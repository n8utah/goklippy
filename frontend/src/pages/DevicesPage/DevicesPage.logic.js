import { useCallback, useEffect, useState } from 'react';
import { getMCUPorts, scanMCUPorts } from '../../helpers/api';
import { useMCUSession } from '../../context/MCUSessionContext';

export function useDevicesPageLogic() {
  const [ports, setPorts] = useState([]);
  const [scannedAt, setScannedAt] = useState(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const { session, connecting, error: connectError, connect, disconnect } = useMCUSession();

  const loadPorts = useCallback(async () => {
    try {
      const data = await getMCUPorts();
      setPorts(data.ports || []);
      setScannedAt(data.scanned_at || null);
    } catch (err) {
      setError(err.message);
    }
  }, []);

  useEffect(() => {
    loadPorts();
  }, [loadPorts]);

  const handleScan = async () => {
    setLoading(true);
    setError('');
    try {
      const data = await scanMCUPorts();
      setPorts(data.ports || []);
      setScannedAt(data.scanned_at || null);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const handleConnect = async (port) => {
    setError('');
    try {
      await connect(port, { useWorkspace: true, profileId: 'ramps14' });
    } catch {
      // connectError set in context
    }
  };

  const handleDisconnect = async () => {
    await disconnect();
  };

  return {
    ports,
    scannedAt,
    loading,
    error,
    handleScan,
    session,
    connecting,
    connectError,
    handleConnect,
    handleDisconnect,
  };
}
