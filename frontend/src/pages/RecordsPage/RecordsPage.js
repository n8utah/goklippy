import { PAGE_SIZE, useRecordsPageLogic } from './RecordsPage.logic';
import PieceDetail from './components/PieceDetail';

import { sorterPieceImageURL } from '../../helpers/api';
import { PIECE_STATUS, pieceBin, pieceName, pieceStatus } from '../../helpers/sorterPieces';

import styles from './RecordsPage.module.css';
import appStyles from '../../styles/app.module.css';

const when = (t) => (t ? new Date(t).toLocaleString() : '—');

function PiecesTab({ logic }) {
  const { filters, pieces, offset } = logic;
  return (
    <>
      <div className={styles.filters}>
        <input placeholder="Part, category or piece id" value={filters.q} onChange={(e) => logic.setFilter('q', e.target.value)} />
        <select value={filters.status} onChange={(e) => logic.setFilter('status', e.target.value)}>
          <option value="">Any status</option>
          {Object.entries(PIECE_STATUS).map(([id, s]) => (
            <option key={id} value={id}>
              {s.label}
            </option>
          ))}
        </select>
        <select value={filters.run} onChange={(e) => logic.setFilter('run', e.target.value)}>
          <option value="">Every run</option>
          {logic.runs.map((r) => (
            <option key={r.id} value={r.id}>
              Run {r.id} · {new Date(r.started_at).toLocaleDateString()}
            </option>
          ))}
        </select>
        <button type="button" className={appStyles.buttonSecondary} onClick={logic.refresh}>
          Refresh
        </button>
      </div>
      {pieces.list.length === 0 ? (
        <p className={styles.muted}>No pieces match.</p>
      ) : (
        <table className={styles.table}>
          <thead>
            <tr>
              <th />
              <th>Piece</th>
              <th>Color</th>
              <th>Category → bin</th>
              <th>Status</th>
              <th>Seen</th>
            </tr>
          </thead>
          <tbody>
            {pieces.list.map((p) => {
              const status = pieceStatus(p.status);
              return (
                <tr key={p.id} className={logic.pieceId === p.id ? styles.rowSelected : undefined} onClick={() => logic.openPiece(p.id)}>
                  <td className={styles.thumbCell}>
                    {p.images?.length > 0 ? <img src={sorterPieceImageURL(p.id, p.images[0].index)} alt="" /> : null}
                  </td>
                  <td>
                    {pieceName(p)}
                    {p.part_id && <span className={styles.muted}> {p.part_id}</span>}
                  </td>
                  <td>{p.color_name || '—'}</td>
                  <td>
                    {p.category_name ? `${p.category_name} → ` : ''}
                    {pieceBin(p)}
                  </td>
                  <td>
                    <span className={`${styles.badge} ${styles[status.tone]}`}>{status.label}</span>
                    {p.problems?.length > 0 && <span className={styles.muted}> {p.problems.length} problem(s)</span>}
                  </td>
                  <td className={styles.muted}>{when(p.seen_at)}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
      )}
      <div className={styles.pager}>
        <span className={styles.muted}>
          {pieces.total === 0 ? '' : `${offset + 1}–${Math.min(offset + PAGE_SIZE, pieces.total)} of ${pieces.total}`}
        </span>
        <button type="button" className={appStyles.buttonSecondary} disabled={offset === 0} onClick={() => logic.setOffset(Math.max(0, offset - PAGE_SIZE))}>
          Newer
        </button>
        <button type="button" className={appStyles.buttonSecondary} disabled={offset + PAGE_SIZE >= pieces.total} onClick={() => logic.setOffset(offset + PAGE_SIZE)}>
          Older
        </button>
      </div>
    </>
  );
}

function RunsTab({ logic }) {
  if (logic.runs.length === 0) return <p className={styles.muted}>No runs yet.</p>;
  return (
    <table className={styles.table}>
      <thead>
        <tr>
          <th>Run</th>
          <th>Profile</th>
          <th>Started</th>
          <th>Ended</th>
          <th>Pieces</th>
          <th>Distributed</th>
        </tr>
      </thead>
      <tbody>
        {logic.runs.map((r) => (
          <tr key={r.id} onClick={() => logic.showRun(r.id)}>
            <td>{r.id}</td>
            <td>{r.profile_name || '—'}</td>
            <td>{when(r.started_at)}</td>
            <td>{r.ended_at ? when(r.ended_at) : 'Running'}</td>
            <td>{r.pieces}</td>
            <td>{r.distributed}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function IncidentsTab({ logic }) {
  if (logic.incidents.length === 0) return <p className={styles.muted}>No incidents.</p>;
  return (
    <table className={styles.table}>
      <thead>
        <tr>
          <th>What</th>
          <th>Opened</th>
          <th>Resolved</th>
        </tr>
      </thead>
      <tbody>
        {logic.incidents.map((i) => (
          <tr key={i.id}>
            <td>
              <strong>{i.title}</strong>
              <br />
              <span className={styles.muted}>{i.message}</span>
            </td>
            <td>{when(i.opened_at)}</td>
            <td>{i.resolved_at ? `${when(i.resolved_at)} (${i.resolution})` : 'Open'}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

const TABS = [
  { id: 'pieces', label: 'Pieces' },
  { id: 'runs', label: 'Runs' },
  { id: 'incidents', label: 'Incidents' },
];

const RecordsPage = ({ initialPieceId }) => {
  const logic = useRecordsPageLogic(initialPieceId);
  const { stats } = logic;

  return (
    <section className={appStyles.card}>
      <h2 className={styles.title}>Records</h2>
      {stats && (
        <p className={appStyles.muted}>
          {stats.pieces.toLocaleString()} pieces seen, {stats.distributed.toLocaleString()} distributed, over {stats.runs} run
          {stats.runs === 1 ? '' : 's'}.
        </p>
      )}
      {logic.error && <p className={styles.alert}>{logic.error}</p>}

      {logic.piece && <PieceDetail piece={logic.piece} onClose={logic.closePiece} onReview={logic.reviewPiece} />}

      <div className={styles.tabs}>
        {TABS.map((t) => (
          <button key={t.id} type="button" className={logic.tab === t.id ? styles.tabActive : styles.tab} onClick={() => logic.setTab(t.id)}>
            {t.label}
          </button>
        ))}
      </div>
      {logic.tab === 'pieces' && <PiecesTab logic={logic} />}
      {logic.tab === 'runs' && <RunsTab logic={logic} />}
      {logic.tab === 'incidents' && <IncidentsTab logic={logic} />}
    </section>
  );
};

export default RecordsPage;
