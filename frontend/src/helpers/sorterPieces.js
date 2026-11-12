// How pieces read in the UI, shared by the Dashboard and Records pages.

export const PIECE_STATUS = {
  tracking: { label: 'Tracking', tone: 'muted' },
  identifying: { label: 'Identifying', tone: 'busy' },
  identified: { label: 'Identified', tone: 'good' },
  unidentified: { label: 'Unidentified', tone: 'warn' },
  id_failed: { label: 'ID failed', tone: 'bad' },
  multi_drop: { label: 'Multi drop', tone: 'warn' },
  distributed: { label: 'Distributed', tone: 'good' },
  lost: { label: 'Lost', tone: 'bad' },
};

export const PROBLEM_LABELS = {
  double_feed: 'Double feed',
  id_failed: 'Identification failed',
  id_timeout: 'Identification too slow',
  no_bin: 'No bin free',
  never_photographed: 'Never photographed',
  didnt_fall: "Didn't fall",
  lost: 'Lost',
};

export function pieceStatus(status) {
  return PIECE_STATUS[status] ?? { label: status, tone: 'muted' };
}

/** "Brick 2 x 4" or what became of a piece with no part. */
export function pieceName(piece) {
  if (piece.part_name) return piece.part_name;
  switch (piece.status) {
    case 'tracking':
    case 'identifying':
      return 'Waiting for the result';
    case 'multi_drop':
      return 'Several pieces together';
    case 'id_failed':
      return 'Identification failed';
    case 'unidentified':
      return 'Not identified';
    default:
      return 'Unidentified piece';
  }
}

/** "Layer 1 · bin 4", or the bucket. Only meaningful once routed. */
export function pieceBin(piece) {
  if (piece.bin_layer) return `Layer ${piece.bin_layer} · bin ${piece.bin_number}`;
  if (piece.route_reason === 'no_chute') return 'No chute (left out)';
  if (piece.route_reason) return 'Bucket';
  return '—';
}

export function scorePct(score) {
  return score ? `${Math.round(score * 100)}%` : '';
}
