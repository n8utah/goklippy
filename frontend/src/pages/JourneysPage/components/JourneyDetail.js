import Moment from './Moment';
import RingDiagram from './RingDiagram';
import { KEPT_COLOR, REASONS, reasonInfo } from '../JourneysPage.logic';
import { sorterJourneyFileURL } from '../../../helpers/api';

import styles from '../JourneysPage.module.css';

function FrameLarge({ journeyId, f, onClose }) {
  const name = f.crop || f.thumb;
  return (
    <div className={styles.large}>
      {name ? (
        <img src={sorterJourneyFileURL(journeyId, name)} alt="The piece in this frame" />
      ) : (
        <p className={styles.hint}>No picture kept for this frame.</p>
      )}
      <dl className={styles.facts}>
        <dt>Decision</dt>
        <dd style={{ color: f.kept ? KEPT_COLOR : undefined }}>
          {f.kept ? 'Kept' : reasonInfo(f.reason).label}
        </dd>
        {!f.kept && reasonInfo(f.reason).hint && (
          <>
            <dt />
            <dd className={styles.hint}>{reasonInfo(f.reason).hint}</dd>
          </>
        )}
        <dt>When</dt>
        <dd>
          {new Date(f.at).toLocaleTimeString([], { hour12: false, fractionalSecondDigits: 1 })}
        </dd>
        <dt>Where</dt>
        <dd>
          {f.angle.toFixed(1)}° {f.zone ? `· ${f.zone} zone` : ''}
        </dd>
        <dt>Sharpness</dt>
        <dd>{f.sharpness.toFixed(1)}</dd>
        <dt>Box</dt>
        <dd>
          {f.box.x2 - f.box.x1}×{f.box.y2 - f.box.y1} px ({f.area} px²) at ({f.box.x1}, {f.box.y1})
        </dd>
      </dl>
      <button type="button" className={styles.linkButton} onClick={onClose}>
        Close
      </button>
    </div>
  );
}

function Track({ j, t, ti, logic }) {
  const counts = {};
  t.frames.forEach((f) => {
    const k = f.kept ? 'kept' : f.reason;
    counts[k] = (counts[k] ?? 0) + 1;
  });
  const shown = t.frames.map((f, i) => [f, i]).filter(([f]) => logic.showFrame(f));
  const large = logic.frame?.track === ti ? t.frames[logic.frame.index] : null;
  return (
    <section className={styles.track}>
      <div className={styles.trackHead}>
        <RingDiagram track={t} onPick={(index) => logic.setFrame({ track: ti, index })} />
        <div className={styles.trackText}>
          <h3 className={styles.trackTitle}>
            {t.channel.toUpperCase()} · track #{t.track_id}
          </h3>
          <p className={styles.hint}>
            {new Date(t.started).toLocaleTimeString()} to{' '}
            {t.ended ? new Date(t.ended).toLocaleTimeString() : 'now'} ·{' '}
            {t.end_reason || 'still riding'} · {t.frames.length + (t.unlisted || 0)} frames
            {t.unlisted ? ` (${t.unlisted} past the list's limit not shown)` : ''}
          </p>
          <div className={styles.chips}>
            <button
              type="button"
              className={logic.filter === 'all' ? styles.chipOn : styles.chip}
              onClick={() => logic.setFilter('all')}
            >
              All {t.frames.length}
            </button>
            <button
              type="button"
              className={logic.filter === 'kept' ? styles.chipOn : styles.chip}
              onClick={() => logic.setFilter('kept')}
            >
              <span className={styles.dot} style={{ background: KEPT_COLOR }} /> Kept{' '}
              {counts.kept ?? 0}
            </button>
            {REASONS.filter((r) => counts[r.id]).map((r) => (
              <button
                key={r.id}
                type="button"
                title={r.hint}
                className={logic.filter === r.id ? styles.chipOn : styles.chip}
                onClick={() => logic.setFilter(r.id)}
              >
                <span className={styles.dot} style={{ background: r.color }} /> {r.label}{' '}
                {counts[r.id]}
              </button>
            ))}
          </div>
        </div>
      </div>

      {t.moments.length > 0 && (
        <div className={styles.moments}>
          {t.moments.map((m) => (
            <Moment
              key={m.image}
              journeyId={j.id}
              moment={m}
              frameW={t.frame_w}
              frameH={t.frame_h}
            />
          ))}
        </div>
      )}

      {large && <FrameLarge journeyId={j.id} f={large} onClose={() => logic.setFrame(null)} />}

      <div className={styles.frames}>
        {shown.map(([f, i]) => (
          <button
            key={i}
            type="button"
            className={f.kept ? styles.frameKept : styles.frame}
            title={`${f.angle.toFixed(0)}° · ${f.kept ? 'kept' : f.reason} · sharpness ${f.sharpness.toFixed(0)}`}
            onClick={() => logic.setFrame({ track: ti, index: i })}
          >
            {f.thumb ? (
              <img src={sorterJourneyFileURL(j.id, f.thumb)} alt="" />
            ) : (
              <span className={styles.noThumb}>no picture</span>
            )}
            <span
              className={styles.frameLabel}
              style={{ borderColor: f.kept ? KEPT_COLOR : reasonInfo(f.reason).color }}
            >
              {f.kept ? 'kept' : reasonInfo(f.reason).label}
            </span>
          </button>
        ))}
        {shown.length === 0 && <p className={styles.hint}>No frames of this kind.</p>}
      </div>
    </section>
  );
}

/** One piece's journey: every channel it was tracked on, frame by frame. */
const JourneyDetail = ({ logic }) => {
  const j = logic.journey;
  if (!logic.selectedId)
    return (
      <p className={styles.empty}>Pick a piece to see its pictures and the decisions about them.</p>
    );
  if (!j) return <p className={styles.empty}>Loading…</p>;
  return (
    <div className={styles.detail}>
      <p className={styles.hint}>
        {j.kept} good picture{j.kept === 1 ? '' : 's'} kept of {j.seen} frames ·{' '}
        {j.done ? 'finished' : 'still riding, updating live'}. Click a dot or a picture for its
        details.
      </p>
      {j.tracks.map((t, ti) => (
        <Track key={`${t.channel}-${t.track_id}`} j={j} t={t} ti={ti} logic={logic} />
      ))}
    </div>
  );
};

export default JourneyDetail;
