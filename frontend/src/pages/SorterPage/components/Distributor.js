import GearCalibration from './GearCalibration';

import styles from '../SorterPage.module.css';
import appStyles from '../../../styles/app.module.css';

const CENTER = 150;
const OUTER = 138;
const INNER = 92;

function point(radius, degrees) {
  const rad = ((degrees - 90) * Math.PI) / 180;
  return `${CENTER + radius * Math.cos(rad)} ${CENTER + radius * Math.sin(rad)}`;
}

function wedgePath(center, width) {
  const start = center - width / 2 + 0.8;
  const end = center + width / 2 - 0.8;
  return `M ${point(INNER, start)} L ${point(OUTER, start)} A ${OUTER} ${OUTER} 0 0 1 ${point(OUTER, end)} L ${point(INNER, end)} A ${INNER} ${INNER} 0 0 0 ${point(INNER, start)} Z`;
}

function BinMap({ distributor, onPoint }) {
  const { bins, angle, alignment, targetBin, moving } = distributor;
  const width = 360 / bins;
  return (
    <svg viewBox="0 0 300 300" className={styles.binMap} role="img" aria-label={`Bin map, chute at bin ${alignment.slot + 1} of ${bins}`}>
      {Array.from({ length: bins }, (_, bin) => {
        const center = bin * width;
        const stateClass =
          bin === targetBin && (moving || bin !== alignment.slot)
            ? styles.binTarget
            : bin === alignment.slot
              ? styles.binPointed
              : styles.bin;
        const [x, y] = point((OUTER + INNER) / 2, center).split(' ');
        return (
          <g key={bin} className={styles.binGroup} onClick={() => onPoint(bin)}>
            <title>{`Point to bin ${bin + 1}`}</title>
            <path d={wedgePath(center, width)} className={stateClass} />
            <text x={x} y={y} className={styles.binLabel}>
              {bin + 1}
            </text>
          </g>
        );
      })}
      <g
        style={{
          transform: `rotate(${angle}deg)`,
          transformOrigin: `${CENTER}px ${CENTER}px`,
          transition: 'transform 220ms linear',
        }}
      >
        <line x1={CENTER} y1={CENTER} x2={CENTER} y2={CENTER - INNER + 8} className={styles.chute} />
        <path d={`M ${CENTER - 11} ${CENTER - INNER + 18} L ${CENTER} ${CENTER - INNER + 1} L ${CENTER + 11} ${CENTER - INNER + 18} Z`} className={styles.chuteHead} />
      </g>
      <circle cx={CENTER} cy={CENTER} r={20} className={styles.hub} />
    </svg>
  );
}

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
  const { config, alignment, activeLayer, bins, targetBin, moving, oid } = distributor;
  const offset = `${alignment.offset >= 0 ? '+' : ''}${alignment.offset.toFixed(1)}°`;

  return (
    <div className={styles.distributor}>
      <header className={styles.cardHeader}>
        <h3 className={styles.cardTitle}>Distributor</h3>
        <span className={moving ? styles.badgeOn : styles.badgeOff}>{moving ? `Moving ${distributor.measuredRpm.toFixed(1)} rpm` : 'Stopped'}</span>
      </header>
      {connected && oid == null && <p className={styles.alert}>{config.stepper} isn&apos;t in the connected config. Add it on Board Setup.</p>}

      <div className={styles.distributorBody}>
        <BinMap distributor={distributor} onPoint={logic.pointToBin} />

        <div className={styles.distributorSide}>
          <div className={styles.stats}>
            <div className={styles.stat}>
              <span className={styles.statLabel}>Active layer</span>
              <span className={styles.statValue}>{activeLayer + 1}</span>
              <span className={styles.statNote}>of {config.layers.length}</span>
            </div>
            <div className={styles.stat}>
              <span className={styles.statLabel}>Pointing at bin</span>
              <span className={styles.statValue}>{alignment.slot + 1}</span>
              <span className={styles.statNote}>of {bins}, {offset} off</span>
            </div>
            <div className={styles.stat}>
              <span className={styles.statLabel}>Target bin</span>
              <span className={styles.statValue}>{targetBin == null ? '—' : targetBin + 1}</span>
              <span className={styles.statNote}>click a bin to move</span>
            </div>
          </div>

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
        </div>
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
            <button type="button" className={`${appStyles.buttonSecondary} ${styles.alignEnd}`} disabled={oid == null} onClick={logic.zeroDistributor}>
              Set position as bin 1
            </button>
          </div>
          <GearCalibration
            config={config}
            microsteps={distributor.microsteps}
            microstepsAssumed={distributor.microstepsAssumed}
            stepsPerRev={distributor.stepsPerRev}
            slot={{ degrees: 360 / bins, name: 'bin' }}
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
