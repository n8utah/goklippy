import { JOG_STEPS } from '../ChutePage.logic';

import styles from '../ChutePage.module.css';
import appStyles from '../../../styles/app.module.css';

const fmt = (angle) => (angle == null ? '—' : `${angle.toFixed(2)}°`);

/**
 * Two captures set the aim for every bin: jog to the middle of the first bin of a
 * section, capture, jog to the last bin of the same section, capture.
 */
const CalibrationPanel = ({ logic, canMove }) => {
  const { captures, calibration, calLayer, calibrationLayers, distributor } = logic;
  const perSection = calLayer?.perSection;

  return (
    <div className={styles.panel}>
      <h3 className={styles.panelTitle}>Calibrate the aim</h3>
      <ol className={styles.steps}>
        <li>Home the chute.</li>
        <li>
          Jog it to the middle of the <strong>first bin</strong> of a section on layer{' '}
          <select
            value={calLayer?.index ?? ''}
            onChange={(e) => logic.setCalibrationLayer(Number(e.target.value))}
            disabled={calibrationLayers.length === 0}
          >
            {calibrationLayers.map((l) => (
              <option key={l.index} value={l.index}>
                {l.index + 1} ({l.perSection} per section)
              </option>
            ))}
          </select>
          , then capture it.
        </li>
        <li>Jog clockwise to the middle of the <strong>last bin</strong> of that same section ({perSection ? `bin ${perSection}` : '—'} of it), then capture it.</li>
        <li>Work it out, check the numbers, and use them.</li>
      </ol>

      <div className={styles.buttonRow}>
        {JOG_STEPS.map((deg) => (
          <button key={deg} type="button" className={appStyles.buttonSecondary} disabled={!canMove} onClick={() => logic.jog(deg)}>
            {deg > 0 ? `+${deg}°` : `−${-deg}°`}
          </button>
        ))}
      </div>

      <div className={styles.captureRow}>
        <button type="button" className={appStyles.buttonSecondary} disabled={!canMove} onClick={() => logic.capture('first')}>
          Capture the first bin
        </button>
        <span className={styles.captureValue}>{fmt(captures.first)}</span>
        <button type="button" className={appStyles.buttonSecondary} disabled={!canMove} onClick={() => logic.capture('last')}>
          Capture the last bin
        </button>
        <span className={styles.captureValue}>{fmt(captures.last)}</span>
        <button
          type="button"
          className={appStyles.button}
          disabled={captures.first == null || captures.last == null || !calLayer}
          onClick={logic.workItOut}
        >
          Work it out
        </button>
      </div>

      {calibration && (
        <div className={styles.result}>
          <span>
            Section width <strong>{calibration.section_width.toFixed(2)}°</strong> (now {distributor.sectionWidth.toFixed(2)}°)
          </span>
          <span>
            First section starts <strong>{calibration.first_section_offset.toFixed(2)}°</strong> from home (now{' '}
            {distributor.firstSectionOffset.toFixed(2)}°)
          </span>
          <button type="button" className={appStyles.button} onClick={logic.applyCalibration}>
            Use these
          </button>
        </div>
      )}
    </div>
  );
};

export default CalibrationPanel;
