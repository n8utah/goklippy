import { CHANNEL_NAMES, STATE_LABELS, useDashboardPageLogic } from './DashboardPage.logic';
import PieceCard from './components/PieceCard';

import styles from './DashboardPage.module.css';
import appStyles from '../../styles/app.module.css';

function ChannelTile({ id, summary }) {
  const live = summary?.valid && summary.age_ms < 2000;
  const zones = [
    summary?.in_drop && 'drop',
    summary?.in_precise && 'precise',
    summary?.in_exit && 'exit',
  ].filter(Boolean);
  return (
    <div className={styles.channel}>
      <div className={styles.channelHeader}>
        <strong>{CHANNEL_NAMES[id]}</strong>
        <span className={`${styles.badge} ${live ? styles.good : styles.muted}`}>{live ? 'Live' : summary?.valid ? 'Stale' : 'No camera'}</span>
      </div>
      <span className={styles.muted}>
        {live
          ? `${summary.pieces} piece${summary.pieces === 1 ? '' : 's'}${zones.length ? ` · in ${zones.join(', ')}` : ''}`
          : 'Waiting for frames'}
      </span>
      {summary?.falls > 0 && <span className={styles.muted}>{summary.falls} pushed off this run</span>}
    </div>
  );
}

function IncidentCard({ incident, busy, onResolve }) {
  return (
    <div className={styles.incident}>
      <div>
        <strong>{incident.title}</strong>
        <p className={styles.incidentMessage}>{incident.message}</p>
      </div>
      <div className={styles.buttonRow}>
        {incident.actions.map((action) => (
          <button key={action} type="button" className={appStyles.button} disabled={busy} onClick={() => onResolve(action)}>
            {action === 'done' ? 'Done' : action}
          </button>
        ))}
      </div>
    </div>
  );
}

const DashboardPage = ({ onOpenPiece }) => {
  const logic = useDashboardPageLogic();
  const { state } = logic;
  const aboard = state?.aboard ?? [];
  const recent = state?.recent ?? [];

  return (
    <section className={appStyles.card}>
      <div className={styles.headerRow}>
        <div>
          <h2 className={styles.title}>
            Sorter <span className={`${styles.badge} ${styles[state?.state] ?? styles.muted}`}>{STATE_LABELS[state?.state] ?? '…'}</span>
          </h2>
          <p className={appStyles.muted}>
            {!logic.connected
              ? 'Connecting to the backend…'
              : state?.hardware?.connected
                ? `${state.hardware.simulated ? 'Simulated machine' : `Board on ${state.hardware.port}`}${state.run_id ? ` · run ${state.run_id}` : ''}`
                : state?.hardware?.message || 'No board connected'}
          </p>
        </div>
        <div className={styles.buttonRow}>
          <button type="button" className={appStyles.buttonSecondary} disabled={!logic.canHome || logic.busy} onClick={() => logic.command('home')}>
            {state?.chute?.homed ? 'Re-home' : 'Home'}
          </button>
          {logic.canResume ? (
            <button type="button" className={appStyles.button} disabled={logic.busy} onClick={() => logic.command('resume')}>
              Resume
            </button>
          ) : logic.canPause ? (
            <button type="button" className={appStyles.button} disabled={logic.busy} onClick={() => logic.command('pause')}>
              Pause
            </button>
          ) : (
            <button type="button" className={appStyles.button} disabled={!logic.canStart || logic.busy} onClick={() => logic.command('start')}>
              Start sorting
            </button>
          )}
          <button type="button" className={appStyles.buttonSecondary} disabled={!logic.canStop || logic.busy} onClick={() => logic.command('stop')}>
            Stop
          </button>
        </div>
      </div>

      {state?.error && <p className={styles.alert}>{state.error}</p>}
      {logic.error && <p className={styles.alert}>{logic.error}</p>}
      {state?.job && (
        <p className={styles.notice}>
          {state.job.name}
          {state.job.total > 0 && ` — step ${state.job.step} of ${state.job.total}`}
          {state.job.detail && `: ${state.job.detail}`}
        </p>
      )}
      {state?.incident && <IncidentCard incident={state.incident} busy={logic.busy} onResolve={logic.resolve} />}
      {state?.state === 'standby' && <p className={styles.notice}>Press Home to start the hardware and home the chute before sorting.</p>}

      <div className={styles.channels}>
        {['c2', 'c3', 'c4'].map((id) => (
          <ChannelTile key={id} id={id} summary={state?.channels?.[id]} />
        ))}
      </div>

      <div className={styles.pieces}>
        <h3 className={styles.panelTitle}>On the classification channel</h3>
        {aboard.length === 0 ? (
          <p className={styles.muted}>Nothing aboard.</p>
        ) : (
          aboard.map((p) => <PieceCard key={p.id} piece={p} onOpen={onOpenPiece} />)
        )}
        <h3 className={styles.panelTitle}>Distributed</h3>
        {recent.length === 0 ? (
          <p className={styles.muted}>No pieces yet.</p>
        ) : (
          recent.map((p) => <PieceCard key={p.id} piece={p} onOpen={onOpenPiece} />)
        )}
      </div>
    </section>
  );
};

export default DashboardPage;
