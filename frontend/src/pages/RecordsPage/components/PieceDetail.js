import { sorterPieceImageURL } from '../../../helpers/api';
import { PROBLEM_LABELS, pieceBin, pieceName, pieceStatus, scorePct } from '../../../helpers/sorterPieces';

import styles from '../RecordsPage.module.css';
import appStyles from '../../../styles/app.module.css';

const time = (t) => (t ? new Date(t).toLocaleString() : '—');

/** Everything recorded about one piece. */
const PieceDetail = ({ piece, onClose }) => {
  const status = pieceStatus(piece.status);
  const prediction = piece.prediction;
  const items = prediction?.items?.slice(0, 5) ?? [];
  const colors = prediction?.colors?.slice(0, 3) ?? [];

  return (
    <div className={styles.detail}>
      <div className={styles.detailHeader}>
        <h3 className={styles.panelTitle}>
          {pieceName(piece)} <span className={`${styles.badge} ${styles[status.tone]}`}>{status.label}</span>
        </h3>
        <button type="button" className={appStyles.buttonSecondary} onClick={onClose}>
          Close
        </button>
      </div>
      <p className={styles.muted}>Piece {piece.id.slice(0, 8)}</p>

      {piece.images?.length > 0 ? (
        <div className={styles.photos}>
          {piece.images.map((img) => (
            <a key={img.index} href={sorterPieceImageURL(piece.id, img.index)} target="_blank" rel="noreferrer" className={img.used ? styles.photoUsed : styles.photo}>
              <img src={sorterPieceImageURL(piece.id, img.index)} alt={`Photo ${img.index + 1}`} />
            </a>
          ))}
        </div>
      ) : (
        <p className={styles.muted}>No photos of this piece.</p>
      )}

      <dl className={styles.facts}>
        <dt>Part</dt>
        <dd>
          {piece.part_id ? `${piece.part_name} (${piece.part_id}) ${scorePct(piece.part_score)}` : '—'}
          {piece.part_category && <span className={styles.muted}> · {piece.part_category}</span>}
        </dd>
        <dt>Color</dt>
        <dd>{piece.color_name ? `${piece.color_name} (id ${piece.color_id}) ${scorePct(piece.color_score)}` : '—'}</dd>
        <dt>Category</dt>
        <dd>{piece.category_name || '—'}</dd>
        <dt>Bin</dt>
        <dd>{pieceBin(piece)}</dd>
        <dt>Seen</dt>
        <dd>{time(piece.seen_at)}</dd>
        <dt>Identified</dt>
        <dd>{time(piece.identified_at)}</dd>
        <dt>Distributed</dt>
        <dd>{time(piece.distributed_at)}</dd>
        {piece.run_id ? (
          <>
            <dt>Run</dt>
            <dd>{piece.run_id}</dd>
          </>
        ) : null}
      </dl>

      {piece.problems?.length > 0 && (
        <div className={styles.problems}>
          {piece.problems.map((p, i) => (
            <p key={i}>
              <strong>{PROBLEM_LABELS[p.kind] ?? p.kind}</strong>: {p.message} <span className={styles.muted}>{new Date(p.at).toLocaleTimeString()}</span>
            </p>
          ))}
        </div>
      )}

      {items.length > 0 && (
        <div>
          <h4 className={styles.subTitle}>Brickognize&apos;s answer</h4>
          <ul className={styles.matches}>
            {items.map((item) => (
              <li key={`${item.id}-${item.rank}`}>
                {item.img_url && <img src={item.img_url} alt="" />}
                <span>
                  {item.name} <span className={styles.muted}>{item.id} · {item.category} · {scorePct(item.score)}</span>
                </span>
              </li>
            ))}
          </ul>
          {colors.length > 0 && (
            <p className={styles.muted}>Colors: {colors.map((c) => `${c.name} ${scorePct(c.score)}`).join(', ')}</p>
          )}
        </div>
      )}
    </div>
  );
};

export default PieceDetail;
