import { useState } from 'react';

import { sorterPieceImageURL } from '../../../helpers/api';
import { pieceBin, pieceName, pieceStatus, scorePct } from '../../../helpers/sorterPieces';

import styles from '../DashboardPage.module.css';

/** One piece: its first photo, name, part and color, status and bin. */
const PieceCard = ({ piece, onOpen }) => {
  const [imageFailed, setImageFailed] = useState(false);
  const status = pieceStatus(piece.status);
  // Photos are saved a moment after they're taken; until then there's nothing to show.
  const hasImage = !imageFailed && piece.status !== 'tracking';

  return (
    <button type="button" className={styles.pieceCard} onClick={() => onOpen?.(piece)}>
      <span className={styles.pieceThumb}>
        {hasImage ? (
          <img src={sorterPieceImageURL(piece.id, 0)} alt="" onError={() => setImageFailed(true)} />
        ) : (
          <span className={styles.pieceThumbEmpty} />
        )}
      </span>
      <span className={styles.pieceText}>
        <strong>{pieceName(piece)}</strong>
        <span className={styles.muted}>
          {piece.part_id && `${piece.part_id} ${scorePct(piece.part_score)}`}
          {piece.color_name && ` · ${piece.color_name}`}
        </span>
        <span className={styles.muted}>
          {piece.category_name && `${piece.category_name} → `}
          {pieceBin(piece)}
        </span>
      </span>
      <span className={`${styles.badge} ${styles[status.tone]}`}>{status.label}</span>
    </button>
  );
};

export default PieceCard;
