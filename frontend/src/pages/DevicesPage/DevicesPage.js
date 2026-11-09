import { useDevicesPageLogic } from './DevicesPage.logic';



import styles from './DevicesPage.module.css';

import appStyles from '../../styles/app.module.css';



const DevicesPage = ({ onGoBoard }) => {

  const { ports, scannedAt, loading, error, handleScan, session, connecting, connectError, handleConnect, handleDisconnect } =

    useDevicesPageLogic();



  return (

    <section className={appStyles.card}>

      <div className={styles.headerRow}>

        <div>

          <h2 className={styles.title}>Devices</h2>

          <p className={appStyles.muted}>Scan for Klipper MCU boards attached over USB/serial.</p>

        </div>

        <button type="button" className={appStyles.button} onClick={handleScan} disabled={loading}>

          {loading ? 'Scanning…' : 'Scan ports'}

        </button>

      </div>



      {session?.port && (

        <div className={styles.sessionBanner}>

          <span>

            Connected to <strong>{session.port}</strong> ({session.objects?.length || 0} objects)

          </span>

          <div className={styles.sessionActions}>

            <button type="button" className={appStyles.buttonSecondary} onClick={handleDisconnect}>

              Disconnect

            </button>

            {onGoBoard && (

              <button type="button" className={appStyles.button} onClick={onGoBoard}>

                Open board setup

              </button>

            )}

          </div>

        </div>

      )}



      {scannedAt && <p className={appStyles.muted}>Last scan: {new Date(scannedAt).toLocaleString()}</p>}

      {(error || connectError) && <p className={appStyles.errorText}>{error || connectError}</p>}



      <div className={styles.tableWrap}>

        <table className={styles.table}>

          <thead>

            <tr>

              <th>Port</th>

              <th>Klipper</th>

              <th>Version</th>

              <th>Messages</th>

              <th>Actions</th>

              <th>Notes</th>

            </tr>

          </thead>

          <tbody>

            {ports.length === 0 && (

              <tr>

                <td colSpan={6} className={appStyles.muted}>

                  No ports found. Click Scan ports.

                </td>

              </tr>

            )}

            {ports.map((port) => {

              const isConnected = session?.port === port.name;

              return (

                <tr key={port.name}>

                  <td>{port.name}</td>

                  <td>{port.is_klipper ? 'Yes' : 'No'}</td>

                  <td>{port.version || '—'}</td>

                  <td>{port.message_count ?? '—'}</td>

                  <td>

                    {port.is_klipper && (

                      <button

                        type="button"

                        className={isConnected ? appStyles.buttonSecondary : appStyles.button}

                        disabled={connecting || isConnected}

                        onClick={() => handleConnect(port.name)}

                      >

                        {connecting ? 'Connecting…' : isConnected ? 'Connected' : 'Connect'}

                      </button>

                    )}

                  </td>

                  <td className={styles.notesCell}>
                    {isConnected ? 'Active session' : port.error || port.description || '—'}
                  </td>

                </tr>

              );

            })}

          </tbody>

        </table>

      </div>

    </section>

  );

};



export default DevicesPage;

