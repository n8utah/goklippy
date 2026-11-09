import BoardDiagram from './components/BoardDiagram/BoardDiagram';

import ConnectionPanel from './components/ConnectionPanel/ConnectionPanel';

import GlobalSteppersPanel from './components/GlobalSteppersPanel/GlobalSteppersPanel';

import GlobalServosPanel from './components/GlobalServosPanel/GlobalServosPanel';

import ConfigPanel from './components/ConfigPanel/ConfigPanel';

import { useBoardSetupPageLogic } from './BoardSetupPage.logic';



import styles from './BoardSetupPage.module.css';

import appStyles from '../../styles/app.module.css';



const BoardSetupPage = () => {

  const {

    tab,

    setTab,

    profile,

    diagram,

    slotStates,

    selectedHotspot,

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
    resettingBoard,
    handleResetBoard,
    handleConnect,

    disconnect,

    readingForSlot,

    objectsBySlot,

    setDigitalOut,

    setServo,

    jogStepper,

    stopStepper,

  } = useBoardSetupPageLogic();



  return (

    <section className={appStyles.card}>

      <div className={styles.headerRow}>

        <div>

          <h2 className={styles.title}>Board Setup</h2>

          <p className={appStyles.muted}>{profile?.name || 'RAMPS 1.4'} — click pins to configure wiring</p>

        </div>

        <div className={appStyles.tabRow}>
          {tab === 'visual' && workspace && (
            <div className={styles.visualToolbar}>
              {workspace.dirty && <span className={styles.unsavedBadge}>Unsaved changes</span>}
              <button
                type="button"
                className={appStyles.buttonSecondary}
                disabled={configLoading || !workspace.dirty}
                onClick={saveConfig}
              >
                {configLoading ? 'Saving…' : 'Save wiring'}
              </button>
            </div>
          )}

          <button

            type="button"

            className={tab === 'visual' ? appStyles.navButtonActive : appStyles.navButton}

            onClick={() => setTab('visual')}

          >

            Visual

          </button>

          <button

            type="button"

            className={tab === 'config' ? appStyles.navButtonActive : appStyles.navButton}

            onClick={() => setTab('config')}

          >

            Config file

          </button>

        </div>

      </div>



      {connected && (

        <div className={styles.liveBanner}>

          <span>

            Live on <strong>{session.port}</strong>

            {wsConnected ? ' — telemetry connected' : ' — waiting for live telemetry…'}

          </span>

          <div className={styles.liveActions}>
            <button
              type="button"
              className={appStyles.buttonSecondary}
              disabled={resettingBoard}
              title="Restarts the board and stops all motors. Takes a few seconds."
              onClick={handleResetBoard}
            >
              {resettingBoard ? 'Resetting…' : 'Reset board'}
            </button>
            <button type="button" className={appStyles.buttonSecondary} disabled={resettingBoard} onClick={disconnect}>
              Disconnect
            </button>
          </div>

        </div>

      )}



      {!connected && tab === 'visual' && (

        <div className={styles.connectRow}>

          <p className={appStyles.muted}>Choose a serial port:</p>
          <ConnectInline
            ports={ports}
            port={selectedPort}
            onPortChange={setSelectedPort}
            connecting={connecting}
            scanning={scanningPorts}
            onScan={scanPorts}
            onConnect={handleConnect}
          />
          {scanMessage && <p className={appStyles.muted}>{scanMessage}</p>}

          {connecting && (
            <p className={appStyles.muted}>
              {connectStatus ||
                'Opening serial port and running Klipper identify — trying baud rates, usually 30–90 seconds. Click Connect only once.'}
            </p>
          )}

        </div>

      )}



      {(configError || (sessionError && !sessionError.includes('reconnecting'))) && (
        <p className={appStyles.errorText}>{configError || sessionError}</p>
      )}



      {tab === 'visual' && (

        <div className={styles.visualLayout}>

          <BoardDiagram

            diagram={diagram}

            profileId="ramps14"

            slotStates={slotStates}

            selectedHotspot={selectedHotspot}

            onSelectHotspot={selectHotspot}

            readingForSlot={connected ? readingForSlot : null}

          />

          <ConnectionPanel

            hotspot={selectedHotspot}

            slots={profile?.slots || {}}

            slotStates={slotStates}

            onUpdate={updateSlot}

            onClear={clearSlot}

            onOpenConfig={() => setTab('config')}

            connected={connected}

            liveObject={selectedHotspot?.slot_ids?.[0] ? objectsBySlot[selectedHotspot.slot_ids[0]] : null}

            liveReading={selectedHotspot?.slot_ids?.[0] ? readingForSlot(selectedHotspot.slot_ids[0]) : null}

            onSetDigitalOut={setDigitalOut}

            onSetServo={setServo}

            onJogStepper={jogStepper}

            onStopStepper={stopStepper}

          />

          <div className={styles.controlsColumn}>
            <GlobalSteppersPanel />
            <GlobalServosPanel
              pcaBoards={pcaBoards}
              configBusy={configLoading}
              onAddPCA9685Board={addPCA9685Board}
            />
          </div>

        </div>

      )}



      {tab === 'config' && (

        <ConfigPanel

          files={configFiles}

          workspace={workspace}

          rawText={rawText}

          loading={configLoading}

          error={configError}

          onLoadFile={loadConfigFile}

          onSave={saveConfig}

          onRawChange={setRawText}

          onApplyRaw={applyRawConfig}

        />

      )}

    </section>

  );

};



function portLabel(port) {
  if (port.is_klipper) {
    return `${port.name} — Klipper ${port.version || ''}`.trim();
  }
  return port.description ? `${port.name} — ${port.description}` : port.name;
}

function ConnectInline({ ports, port, onPortChange, connecting, scanning, onScan, onConnect }) {
  const busy = connecting || scanning;
  return (
    <div className={styles.connectInline}>
      <select
        className={styles.portInput}
        aria-label="Serial port"
        value={port}
        disabled={busy || ports.length === 0}
        onChange={(e) => onPortChange(e.target.value)}
      >
        {ports.length === 0 && <option value="">No serial ports found</option>}
        {ports.map((p) => (
          <option key={p.name} value={p.name}>
            {portLabel(p)}
          </option>
        ))}
      </select>
      <button
        type="button"
        className={appStyles.buttonSecondary}
        disabled={busy}
        title="Checks each port for a Klipper board. Boards may reset while being checked."
        onClick={onScan}
      >
        {scanning ? 'Scanning…' : 'Scan'}
      </button>
      <button type="button" className={appStyles.button} disabled={busy || !port} onClick={() => onConnect(port)}>
        {connecting ? 'Connecting…' : 'Connect'}
      </button>
    </div>
  );
}

export default BoardSetupPage;

