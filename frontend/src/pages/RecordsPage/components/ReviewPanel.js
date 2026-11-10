import { useState } from 'react';

import styles from '../RecordsPage.module.css';

const ISSUES = [
  { id: 'not_lego', label: 'Not LEGO' },
  { id: 'multiple_pieces', label: 'Several pieces in frame' },
  { id: 'no_piece', label: 'No piece in frame' },
  { id: 'assembly', label: 'Assembly' },
  { id: 'entangled', label: 'Pieces entangled' },
  { id: 'blurry', label: 'Blurry' },
];

function YesNo({ value, onChange }) {
  return (
    <span className={styles.yesNo}>
      <button type="button" className={value === true ? styles.choiceOn : styles.choice} onClick={() => onChange(true)}>
        Yes
      </button>
      <button type="button" className={value === false ? styles.choiceOn : styles.choice} onClick={() => onChange(false)}>
        No
      </button>
    </span>
  );
}

/**
 * A person's check of the identification. Every click saves; the whole review is
 * sent each time, so what's shown is what's stored.
 */
const ReviewPanel = ({ piece, onSave }) => {
  const saved = piece.review;
  const [review, setReview] = useState({
    part_correct: saved?.part_correct ?? null,
    color_correct: saved?.color_correct ?? null,
    color_id: saved?.color_id ?? '',
    issues: saved?.issues ?? [],
  });
  const [status, setStatus] = useState('');
  const colors = piece.prediction?.colors ?? [];

  const save = async (patch) => {
    const next = { ...review, ...patch };
    if (next.color_correct !== false) next.color_id = '';
    setReview(next);
    setStatus('Saving…');
    try {
      await onSave(next);
      setStatus('Saved');
    } catch (err) {
      setStatus(`Not saved: ${err.message}`);
    }
  };

  const toggleIssue = (id) =>
    save({ issues: review.issues.includes(id) ? review.issues.filter((i) => i !== id) : [...review.issues, id] });

  return (
    <div className={styles.review}>
      <h4 className={styles.subTitle}>Review</h4>
      <div className={styles.reviewRow}>
        <span>Part correct?</span>
        <YesNo value={review.part_correct} onChange={(v) => save({ part_correct: v })} />
      </div>
      <div className={styles.reviewRow}>
        <span>Color correct?{piece.color_name ? ` (${piece.color_name})` : ''}</span>
        <YesNo value={review.color_correct} onChange={(v) => save({ color_correct: v })} />
        {review.color_correct === false && (
          <span className={styles.yesNo}>
            It&apos;s
            <select value={review.color_id} onChange={(e) => save({ color_id: e.target.value })}>
              <option value="">pick one</option>
              {colors.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.name}
                </option>
              ))}
              {review.color_id && !colors.some((c) => c.id === review.color_id) && <option value={review.color_id}>id {review.color_id}</option>}
            </select>
            or id
            <input className={styles.colorInput} value={review.color_id} onChange={(e) => setReview({ ...review, color_id: e.target.value })} onBlur={() => save({})} />
          </span>
        )}
      </div>
      <div className={styles.issues}>
        {ISSUES.map((i) => (
          <button key={i.id} type="button" className={review.issues.includes(i.id) ? styles.choiceOn : styles.choice} onClick={() => toggleIssue(i.id)}>
            {i.label}
          </button>
        ))}
      </div>
      {status && <span className={styles.muted}>{status}</span>}
    </div>
  );
};

export default ReviewPanel;
