import { ALL_CHANNELS, CHANNEL_LABELS, deviceLabel, useCameraFinder } from '../CamerasPage.logic';
import { sorterCameraFindImageURL } from '../../../helpers/api';

import styles from '../CamerasPage.module.css';
import appStyles from '../../../styles/app.module.css';

const pct = (v) => `${(v * 100).toFixed(1)}%`;
const label = (ch) => CHANNEL_LABELS[ch] ?? 'C-channel 1';

/** A picture with the compared middle outlined. */
function FindPicture({ channel, which, stamp, centerShare, caption }) {
  const margin = `${((1 - centerShare) / 2) * 100}%`;
  return (
    <figure className={styles.findFigure}>
      <div className={styles.findPicture}>
        <img src={sorterCameraFindImageURL(channel, which, stamp)} alt={caption} />
        <span className={styles.findCenter} style={{ inset: margin }} />
      </div>
      <figcaption className={styles.hint}>{caption}</figcaption>
    </figure>
  );
}

function FindResult({ finder, logic }) {
  const r = finder.result;
  const name = (id) => {
    const d = logic.devices.list.find((x) => x.id === id);
    return d ? deviceLabel(d) : 'a camera no longer listed';
  };
  const proposed = Object.entries(r.proposed ?? {});
  return (
    <div className={styles.findResult}>
      <p className={styles.hint}>Searched {new Date(r.finished).toLocaleString()}.</p>
      {r.error && <p className={styles.alert}>{r.error}</p>}
      {r.notes?.map((n) => (
        <p key={n} className={styles.warnText}>
          {n}
        </p>
      ))}

      {proposed.length > 0 && (
        <>
          <h4 className={styles.subTitle}>Proposed</h4>
          {proposed.map(([ch, id]) => (
            <div key={ch} className={styles.findPair}>
              <strong>
                {label(ch)}: {name(id)}
              </strong>
              <div className={styles.findPictures}>
                <FindPicture
                  channel={ch}
                  which="before"
                  stamp={finder.stamp}
                  centerShare={r.center_share}
                  caption="Before the turn"
                />
                <FindPicture
                  channel={ch}
                  which="after"
                  stamp={finder.stamp}
                  centerShare={r.center_share}
                  caption="After the turn"
                />
              </div>
            </div>
          ))}
          <div className={styles.buttonRow}>
            <button type="button" className={appStyles.button} onClick={finder.apply}>
              Use these
            </button>
            <span className={styles.hint}>Cameras not proposed keep their channel.</span>
          </div>
        </>
      )}

      {r.scores && (
        <details>
          <summary className={styles.hint}>How much each picture changed</summary>
          <table className={styles.findTable}>
            <thead>
              <tr>
                <th>Turned</th>
                {r.devices.map((d) => (
                  <th key={d}>{name(d)}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {ALL_CHANNELS.filter((ch) => r.scores[ch]).map((ch) => (
                <tr key={ch}>
                  <td>{label(ch)}</td>
                  {r.devices.map((d) => (
                    <td key={d} className={r.proposed?.[ch] === d ? styles.findBest : undefined}>
                      {pct(r.scores[ch][d] ?? 0)}
                    </td>
                  ))}
                </tr>
              ))}
              <tr>
                <td>Nothing moving</td>
                {r.devices.map((d) => (
                  <td key={d}>{pct(r.noise?.[d] ?? 0)}</td>
                ))}
              </tr>
            </tbody>
          </table>
        </details>
      )}
    </div>
  );
}

/** Finds which camera is where by turning each channel a little while watching them. */
const CameraFinder = ({ logic }) => {
  const finder = useCameraFinder(logic);
  const { devices } = logic;
  return (
    <div className={styles.panel}>
      <h3 className={styles.panelTitle}>Find which camera is where</h3>
      <p className={styles.hint}>
        Turns C1–C4 one at a time a few degrees and back while the ticked cameras watch. The camera
        whose picture changes most is over that channel. Only the middle of each picture is
        compared, so a neighboring channel at the edge doesn’t count. Something on each channel has
        to show the turn: a piece or a strip of tape.
      </p>

      <div className={styles.findPicks}>
        {devices.list.map((d) => (
          <label key={d.id} className={styles.check}>
            <input
              type="checkbox"
              checked={finder.isPicked(d.id)}
              onChange={() => finder.toggle(d.id)}
              disabled={finder.running}
            />
            {deviceLabel(d)}
          </label>
        ))}
      </div>

      <div className={styles.buttonRow}>
        <label className={styles.field}>
          <span className={styles.fieldLabel}>Turn each channel (°)</span>
          <input
            type="number"
            min="1"
            max="90"
            value={finder.degrees}
            disabled={finder.running}
            onChange={(e) => {
              const v = Number(e.target.value);
              if (v > 0 && v <= 90) finder.setDegrees(v);
            }}
          />
        </label>
        {finder.running ? (
          <button type="button" className={appStyles.buttonSecondary} onClick={finder.cancel}>
            Cancel
          </button>
        ) : (
          <button
            type="button"
            className={appStyles.button}
            disabled={Boolean(finder.blocked)}
            onClick={finder.start}
          >
            Find cameras
          </button>
        )}
      </div>
      {finder.blocked && !finder.running && <p className={styles.hint}>{finder.blocked}</p>}
      {finder.job && (
        <p className={styles.hint}>
          {finder.job.total > 0 && `Step ${finder.job.step} of ${finder.job.total}: `}
          {finder.job.detail}
        </p>
      )}
      {finder.error && <p className={styles.alert}>{finder.error}</p>}
      {finder.result && !finder.running && <FindResult finder={finder} logic={logic} />}
    </div>
  );
};

export default CameraFinder;
