import { reasonInfo, useJourneysPageLogic } from './JourneysPage.logic';
import JourneyDetail from './components/JourneyDetail';
import { sorterJourneyFileURL } from '../../helpers/api';

import styles from './JourneysPage.module.css';
import appStyles from '../../styles/app.module.css';

const topReasons = (reasons) =>
  Object.entries(reasons ?? {})
    .sort((a, b) => b[1] - a[1])
    .slice(0, 2)
    .map(([r, n]) => `${reasonInfo(r).label.toLowerCase()} ${n}`)
    .join(', ');

const JourneysPage = () => {
  const logic = useJourneysPageLogic();
  return (
    <section className={appStyles.card}>
      <h2 className={styles.title}>Piece journeys</h2>
      <p className={appStyles.muted}>
        Every piece the cameras tracked, newest first: each frame of it, which made good pictures
        for identifying it, and why the rest didn&apos;t. For tuning the picture rules under System
        › Settings › Pictures. Updates live while sorting.
      </p>
      {logic.error && <p className={styles.alert}>{logic.error}</p>}
      <div className={styles.layout}>
        <ul className={styles.list}>
          {logic.list.length === 0 && (
            <li className={styles.hint}>
              No pieces tracked yet. They appear here as the cameras see them.
            </li>
          )}
          {logic.list.map((j) => (
            <li key={j.id}>
              <button
                type="button"
                className={j.id === logic.selectedId ? styles.rowOn : styles.row}
                onClick={() => logic.select(j.id)}
              >
                {j.thumb ? (
                  <img
                    className={styles.rowThumb}
                    src={sorterJourneyFileURL(j.id, j.thumb)}
                    alt=""
                  />
                ) : (
                  <span className={styles.rowThumbEmpty}>none kept</span>
                )}
                <span className={styles.rowText}>
                  <strong>
                    {(j.channels ?? []).map((c) => c.toUpperCase()).join(' → ')} ·{' '}
                    {new Date(j.started).toLocaleTimeString()}
                  </strong>
                  <span className={styles.hint}>
                    {j.kept} kept of {j.seen} · {j.done ? j.end_reason : 'riding'}
                  </span>
                  {topReasons(j.reasons) && (
                    <span className={styles.hint}>{topReasons(j.reasons)}</span>
                  )}
                </span>
              </button>
            </li>
          ))}
        </ul>
        <JourneyDetail logic={logic} />
      </div>
    </section>
  );
};

export default JourneysPage;
