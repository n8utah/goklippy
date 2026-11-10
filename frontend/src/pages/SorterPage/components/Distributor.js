import GearCalibration from './GearCalibration';

import styles from '../SorterPage.module.css';
import appStyles from '../../../styles/app.module.css';

function LayerSettings({ layer, index, canRemove, servoObjects, onChange, onRemove }) {
  return (
    <div className={styles.layerSettings}>
      <label className={styles.field}>
        <span className={styles.fieldLabel}>Layer {index + 1} servo</span>
        <select value={layer.servo} onChange={(e) => onChange(index, { servo: e.target.value })}>
          <option value="">None</option>
          {layer.servo && !servoObjects.some((obj) => obj.section === layer.servo) && <option value={layer.servo}>{layer.servo}</option>}
          {servoObjects.map((obj) => (
            <option key={obj.oid} value={obj.section}>
              {obj.section}
            </option>
          ))}
        </select>
      </label>
      <label className={styles.field}>
        <span className={styles.fieldLabel}>Bins</span>
        <select value={layer.bins} onChange={(e) => onChange(index, { bins: Number(e.target.value) })}>
          <option value={18}>18</option>
          <option value={12}>12</option>
        </select>
      </label>
      <label className={styles.field}>
        <span className={styles.fieldLabel}>Open °</span>
        <input type="number" min="0" max="180" value={layer.openAngle} onChange={(e) => onChange(index, { openAngle: Number(e.target.value) })} />
      </label>
      <label className={styles.field}>
        <span className={styles.fieldLabel}>Closed °</span>
        <input type="number" min="0" max="180" value={layer.closedAngle} onChange={(e) => onChange(index, { closedAngle: Number(e.target.value) })} />
      </label>
      <button type="button" className={appStyles.buttonSecondary} disabled={!canRemove} onClick={() => onRemove(index)}>
        Remove
      </button>
    </div>
  );
}

const Distributor = ({ logic, showSettings, connected }) => {
  const { distributor, servoObjects, stepperObjects } = logic;
  const { config, activeLayer, moving, oid } = distributor;

  return (
    <div className={styles.distributor}>
      <header className={styles.cardHeader}>
        <h3 className={styles.cardTitle}>Distributor</h3>
        <span className={moving ? styles.badgeOn : styles.badgeOff}>{moving ? `Moving ${distributor.measuredRpm.toFixed(1)} rpm` : 'Stopped'}</span>
      </header>
      {connected && oid == null && <p className={styles.alert}>{config.stepper} isn&apos;t in the connected config. Add it on Board Setup.</p>}
      <p className={appStyles.muted}>Home, calibrate and aim the chute, and test bins, on the Chute page.</p>

      <div className={styles.layerStack}>
        {distributor.layers.map((layer, index) => (
          <button
            key={index}
            type="button"
            className={index === activeLayer ? styles.layerActive : styles.layer}
            onClick={() => logic.selectLayer(index)}
          >
            <span className={styles.layerName}>Layer {index + 1}</span>
            <span>{layer.bins} bins</span>
            <span>{layer.servoObject ? (index === activeLayer ? `Open ${layer.openAngle}°` : `Closed ${layer.closedAngle}°`) : 'No servo'}</span>
          </button>
        ))}
      </div>

      {showSettings && (
        <div className={styles.distributorSettings}>
          <div className={styles.fields}>
            <label className={styles.field}>
              <span className={styles.fieldLabel}>Motor</span>
              <select value={config.stepper} onChange={(e) => logic.setDistributor({ stepper: e.target.value })}>
                {!stepperObjects.some((obj) => obj.section === config.stepper) && <option value={config.stepper}>{config.stepper}</option>}
                {stepperObjects.map((obj) => (
                  <option key={obj.oid} value={obj.section}>
                    {obj.section}
                  </option>
                ))}
              </select>
            </label>
            <label className={styles.field}>
              <span className={styles.fieldLabel}>Speed (rpm)</span>
              <input type="number" min="0.1" step="1" value={config.rpm} onChange={(e) => Number(e.target.value) > 0 && logic.setDistributor({ rpm: Number(e.target.value) })} />
            </label>
            <label className={styles.field}>
              <span className={styles.fieldLabel}>Accel (rpm/s)</span>
              <input type="number" min="0" step="10" value={config.accel} onChange={(e) => Number(e.target.value) >= 0 && logic.setDistributor({ accel: Number(e.target.value) })} />
            </label>
          </div>
          <GearCalibration
            config={config}
            microsteps={distributor.microsteps}
            microstepsAssumed={distributor.microstepsAssumed}
            stepsPerRev={distributor.stepsPerRev}
            slot={{ degrees: distributor.sectionPitch, name: 'section' }}
            canTurn={oid != null && !moving}
            onChange={logic.setDistributor}
            onTurn={logic.turnDistributorRev}
          />
          {distributor.layers.map((layer, index) => (
            <LayerSettings
              key={index}
              layer={layer}
              index={index}
              canRemove={distributor.layers.length > 1}
              servoObjects={servoObjects}
              onChange={logic.setLayer}
              onRemove={logic.removeLayer}
            />
          ))}
          <button type="button" className={appStyles.buttonSecondary} onClick={logic.addLayer}>
            Add layer
          </button>
        </div>
      )}
    </div>
  );
};

export default Distributor;
