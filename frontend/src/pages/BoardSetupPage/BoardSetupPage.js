import { useState } from 'react';

import BoardDiagram from './components/BoardDiagram/BoardDiagram';

import ConnectionPanel from './components/ConnectionPanel/ConnectionPanel';

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

    session,

    connected,

    connecting,

    sessionError,

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

            Live on <strong>{session.port}</strong> — live readings via WebSocket

          </span>

          <button type="button" className={appStyles.buttonSecondary} onClick={disconnect}>

            Disconnect

          </button>

        </div>

      )}



      {!connected && tab === 'visual' && (

        <div className={styles.connectRow}>

          <p className={appStyles.muted}>Connect from Devices, or enter a port here:</p>

          <ConnectInline connecting={connecting} onConnect={handleConnect} />

        </div>

      )}



      {(configError || sessionError) && tab === 'visual' && (

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



function ConnectInline({ connecting, onConnect }) {

  const [port, setPort] = useState('COM6');

  return (

    <div className={styles.connectInline}>

      <input

        className={styles.portInput}

        value={port}

        onChange={(e) => setPort(e.target.value)}

        placeholder="COM6"

      />

      <button type="button" className={appStyles.button} disabled={connecting} onClick={() => onConnect(port)}>

        {connecting ? 'Connecting…' : 'Connect'}

      </button>

    </div>

  );

}



export default BoardSetupPage;

