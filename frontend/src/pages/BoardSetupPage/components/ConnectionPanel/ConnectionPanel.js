import { useCallback, useEffect, useRef, useState } from 'react';

import { useMCUSession } from '../../../../context/MCUSessionContext';
import { DEFAULT_OUTPUT_TIMEOUT_S, MAX_OUTPUT_TIMEOUT_S, isOnOffSlot } from '../../../../helpers/boardSlotConfig';

import styles from './ConnectionPanel.module.css';

import appStyles from '../../../../styles/app.module.css';



const ConnectionPanel = ({

  hotspot,

  slots,

  slotStates,

  onUpdate,

  onClear,

  onOpenConfig,

  connected,

  liveObject,

  liveReading,

  onSetDigitalOut,

  onSetServo,

  onJogStepper,

  onStopStepper,

}) => {

  if (!hotspot) {

    return (

      <aside className={styles.panel}>

        <h3 className={styles.heading}>Connection</h3>

        <p className={appStyles.muted}>Click a pin on the board diagram to configure what is wired there.</p>

        {connected && <p className={styles.liveHint}>Live session active — connected slots show readings on the diagram.</p>}

      </aside>

    );

  }



  const slotId = hotspot.slot_ids?.[0];

  const slot = slots[slotId] || {};

  const state = slotStates[slotId] || { connected: false, label: slot.label || hotspot.label };

  const timeout = state.maxMcuDuration ?? DEFAULT_OUTPUT_TIMEOUT_S;



  const handleConnect = () => {

    onUpdate(slotId, {

      connected: true,

      label: state.label || slot.label || hotspot.label,

      type: slot.type,

      maxMcuDuration: timeout,

    });

  };



  return (

    <aside className={styles.panel}>

      <h3 className={styles.heading}>{hotspot.label}</h3>

      <p className={styles.meta}>Slot: {slotId}</p>

      <p className={styles.meta}>Type: {isOnOffSlot(slot) ? 'on/off output' : slot.type || 'unknown'}</p>

      <p className={styles.meta}>Accepts: {(hotspot.accepts || []).join(', ') || '—'}</p>



      {connected && liveObject && (

        <LiveControls

          slot={slot}

          liveObject={liveObject}

          liveReading={liveReading}

          onSetDigitalOut={onSetDigitalOut}

          onSetServo={onSetServo}

          onJogStepper={onJogStepper}

          onStopStepper={onStopStepper}

        />

      )}



      {!connected && (

        <>

          <label className={styles.label} htmlFor="slot-label">

            Label

          </label>

          <input

            id="slot-label"

            className={styles.input}

            value={state.label || ''}

            onChange={(event) => onUpdate(slotId, { ...state, label: event.target.value, localOnly: true })}

          />



          {isOnOffSlot(slot) && (
            <>
              <label className={styles.label} htmlFor="slot-timeout">
                Safety timeout (seconds, 0 = off)
              </label>
              <input
                id="slot-timeout"
                className={styles.input}
                type="number"
                min="0"
                max={MAX_OUTPUT_TIMEOUT_S}
                step="0.5"
                value={timeout}
                onChange={(event) => {
                  const v = Number(event.target.value);
                  if (event.target.value !== '' && v >= 0 && v <= MAX_OUTPUT_TIMEOUT_S) {
                    onUpdate(slotId, { ...state, maxMcuDuration: v, localOnly: true });
                  }
                }}
              />
              <p className={styles.meta}>
                If the backend stops refreshing this output for this long (it crashed, or the USB cable came out), the board shuts down: the
                output and every motor stop. Up to {MAX_OUTPUT_TIMEOUT_S} s.
              </p>
            </>
          )}

          {slot.defaults?.sensor_type && !isOnOffSlot(slot) && (

            <>

              <label className={styles.label}>Sensor type</label>

              <input className={styles.input} readOnly value={slot.defaults.sensor_type} />

            </>

          )}



          {slot.pins && (

            <div className={styles.pinBlock}>

              <p className={styles.label}>Pins</p>

              <ul className={styles.pinList}>

                {Object.entries(slot.pins).map(([key, value]) => (

                  <li key={key}>

                    {key}: {value}

                  </li>

                ))}

              </ul>

            </div>

          )}



          <div className={styles.actions}>

            <button type="button" className={appStyles.button} onClick={handleConnect}>

              {state.connected ? 'Update' : 'Mark connected'}

            </button>

            {state.connected && (

              <button type="button" className={appStyles.buttonSecondary} onClick={() => onClear(slotId)}>

                Clear

              </button>

            )}

          </div>

        </>

      )}



      <button type="button" className={styles.linkButton} onClick={onOpenConfig}>

        Edit in config file →

      </button>

    </aside>

  );

};



function LiveControls({ slot, liveObject, liveReading, onSetDigitalOut, onSetServo, onJogStepper, onStopStepper }) {

  const kind = liveObject.kind;

  const oid = liveObject.oid;

  const isServo = kind === 'pwm_out' && liveObject.meta?.device === 'servo';



  if (kind === 'analog_in' || slot.type === 'thermistor') {

    const val = liveReading?.value;

    const display = val != null ? `${Number(val).toFixed(3)} (${liveReading?.unit || 'adc'})` : '—';

    return (

      <div className={styles.liveBlock}>

        <p className={styles.liveLabel}>Live reading</p>

        <p className={styles.liveValue}>{display}</p>

        <p className={styles.meta}>Section: {liveObject.section}</p>

      </div>

    );

  }



  if (kind === 'endstop') {

    const triggered = liveReading?.value ? 'TRIGGERED' : 'open';

    return (

      <div className={styles.liveBlock}>

        <p className={styles.liveLabel}>Endstop</p>

        <p className={styles.liveValue}>{triggered}</p>

      </div>

    );

  }



  if (kind === 'digital_out' || (kind === 'pwm_out' && !isServo)) {

    return <DigitalOutControl oid={oid} onSetDigitalOut={onSetDigitalOut} label={slot.type === 'fan' ? 'Fan' : 'Output'} />;

  }



  if (isServo) {

    return <ServoControl oid={oid} onSetServo={onSetServo} />;

  }



  if (kind === 'stepper') {

    return <StepperControl oid={oid} liveReading={liveReading} onJogStepper={onJogStepper} onStopStepper={onStopStepper} />;

  }



  return (

    <div className={styles.liveBlock}>

      <p className={styles.meta}>Configured ({kind}) — OID {oid}</p>

    </div>

  );

}



function DigitalOutControl({ oid, onSetDigitalOut, label }) {

  const [on, setOn] = useState(false);

  const [busy, setBusy] = useState(false);



  const toggle = async () => {

    setBusy(true);

    try {

      const next = !on;

      await onSetDigitalOut(oid, next);

      setOn(next);

    } finally {

      setBusy(false);

    }

  };



  return (

    <div className={styles.liveBlock}>

      <p className={styles.liveLabel}>{label}</p>

      <button type="button" className={appStyles.button} disabled={busy} onClick={toggle}>

        {on ? 'Turn off' : 'Turn on'}

      </button>

    </div>

  );

}



function ServoControl({ oid, onSetServo }) {
  const { servoAngles } = useMCUSession();
  const [angle, setAngle] = useState(servoAngles[oid] ?? 90);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (servoAngles[oid] != null) {
      setAngle(servoAngles[oid]);
    }
  }, [servoAngles, oid]);

  const clampAngle = (value) => Math.min(180, Math.max(0, Math.round(value) || 0));

  const apply = async (nextAngle = angle) => {
    setBusy(true);
    try {
      const applied = await onSetServo(oid, nextAngle);
      if (applied != null) {
        setAngle(applied);
      }
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className={styles.liveBlock}>
      <p className={styles.liveLabel}>Servo position</p>
      <div className={styles.speedRow}>
        <button
          type="button"
          className={styles.stepBtn}
          aria-label="Decrease angle by 5"
          disabled={busy}
          onClick={() => setAngle((prev) => clampAngle(prev - 5))}
        >
          ▼
        </button>
        <input
          type="number"
          min="0"
          max="180"
          className={styles.input}
          value={angle}
          disabled={busy}
          onChange={(e) => setAngle(clampAngle(Number(e.target.value)))}
        />
        <button
          type="button"
          className={styles.stepBtn}
          aria-label="Increase angle by 5"
          disabled={busy}
          onClick={() => setAngle((prev) => clampAngle(prev + 5))}
        >
          ▲
        </button>
      </div>
      <input
        type="range"
        min="0"
        max="180"
        value={angle}
        disabled={busy}
        onChange={(e) => setAngle(Number(e.target.value))}
        onMouseUp={() => apply()}
        onTouchEnd={() => apply()}
        className={styles.range}
      />
      <p className={styles.meta}>{angle}°</p>
      <button type="button" className={appStyles.button} disabled={busy} onClick={() => apply()}>
        Set angle
      </button>
    </div>
  );
}



function StepperControl({ oid, liveReading, onJogStepper, onStopStepper }) {

  const { isStepperRunning, setStepperRunning } = useMCUSession();

  const [forward, setForward] = useState(true);

  const [speedHz, setSpeedHz] = useState(500);

  const [mode, setMode] = useState('continuous');

  const [steps, setSteps] = useState(200);

  const [busy, setBusy] = useState(false);
  const running = isStepperRunning(oid);
  const paramsRef = useRef({ forward: true, speedHz: 500 });

  const clampSpeed = (value) => Math.min(50000, Math.max(1, value));

  const adjustSpeed = (delta) => {
    setSpeedHz((prev) => clampSpeed(prev + delta));
  };

  const applyContinuousJog = useCallback(
    async (params) => {
      await onJogStepper(oid, { forward: params.forward, speedHz: params.speedHz, steps: 0 });
    },
    [oid, onJogStepper],
  );

  useEffect(() => {
    if (!running || mode !== 'continuous') {
      return undefined;
    }

    const prev = paramsRef.current;
    if (prev.forward === forward && prev.speedHz === speedHz) {
      return undefined;
    }

    const onlySpeed = prev.forward === forward && prev.speedHz !== speedHz;
    paramsRef.current = { forward, speedHz };

    const delay = onlySpeed ? 150 : 0;
    const timer = setTimeout(() => {
      applyContinuousJog({ forward, speedHz }).catch(() => setStepperRunning(oid, false));
    }, delay);

    return () => clearTimeout(timer);
  }, [forward, speedHz, running, mode, applyContinuousJog, oid, setStepperRunning]);

  const run = async () => {
    setBusy(true);
    try {
      const stepCount = mode === 'distance' ? steps : 0;
      paramsRef.current = { forward, speedHz };
      await onJogStepper(oid, { forward, speedHz, steps: stepCount });
      if (mode === 'continuous') {
        setStepperRunning(oid, true);
      } else {
        setStepperRunning(oid, false);
      }
    } finally {
      setBusy(false);
    }
  };

  const stop = async () => {
    setBusy(true);
    try {
      await onStopStepper(oid);
    } finally {
      setBusy(false);
    }
  };

  const handleModeChange = async (nextMode) => {
    if (running && mode === 'continuous' && nextMode !== 'continuous') {
      await stop();
    }
    setMode(nextMode);
  };

  const livePosition = liveReading?.value;
  const positionDisplay = livePosition != null ? Number(livePosition) : null;



  return (

    <div className={styles.liveBlock}>

      <p className={styles.liveLabel}>Stepper jog</p>



      <label className={styles.label}>Direction</label>

      <div className={styles.segmentRow}>

        <button

          type="button"

          className={forward ? appStyles.navButtonActive : appStyles.navButton}

          onClick={() => setForward(true)}

        >

          Forward

        </button>

        <button

          type="button"

          className={!forward ? appStyles.navButtonActive : appStyles.navButton}

          onClick={() => setForward(false)}

        >

          Reverse

        </button>

      </div>



      <label className={styles.label} htmlFor="stepper-speed">

        Speed (steps/s)

      </label>

      <div className={styles.speedRow}>

        <button type="button" className={styles.stepBtn} aria-label="Decrease speed by 100" onClick={() => adjustSpeed(-100)}>

          ▼

        </button>

        <input

          id="stepper-speed"

          type="number"

          min="1"

          max="50000"

          className={styles.input}

          value={speedHz}

          onChange={(e) => setSpeedHz(clampSpeed(Number(e.target.value) || 1))}

        />

        <button type="button" className={styles.stepBtn} aria-label="Increase speed by 100" onClick={() => adjustSpeed(100)}>

          ▲

        </button>

      </div>



      <label className={styles.label}>Mode</label>

      <div className={styles.segmentRow}>

        <button

          type="button"

          className={mode === 'continuous' ? appStyles.navButtonActive : appStyles.navButton}

          onClick={() => handleModeChange('continuous')}

        >

          Continuous

        </button>

        <button

          type="button"

          className={mode === 'distance' ? appStyles.navButtonActive : appStyles.navButton}

          onClick={() => handleModeChange('distance')}

        >

          Distance

        </button>

      </div>



      {mode === 'distance' && (

        <>

          <label className={styles.label} htmlFor="stepper-steps">

            Steps

          </label>

          <input

            id="stepper-steps"

            type="number"

            min="1"

            max="1000000"

            className={styles.input}

            value={steps}

            onChange={(e) => setSteps(Number(e.target.value))}

          />

        </>

      )}



      <div className={styles.actions}>
        <button type="button" className={appStyles.button} disabled={busy || (mode === 'continuous' && running)} onClick={run}>
          {mode === 'continuous' ? 'Run' : `Run ${steps} steps`}
        </button>
        {mode === 'continuous' && running && (
          <button type="button" className={appStyles.buttonSecondary} disabled={busy} onClick={stop}>
            Stop
          </button>
        )}
      </div>

      {positionDisplay != null && (
        <>
          <p className={styles.meta}>Position</p>
          <p className={styles.liveValue}>{positionDisplay} steps</p>
        </>
      )}

    </div>

  );

}



export default ConnectionPanel;

