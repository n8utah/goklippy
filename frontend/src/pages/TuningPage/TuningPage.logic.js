import { DEFAULT_SORTER_SETTINGS, useMachineSettings } from '../../helpers/sorterSettings';

// Every tuning value, grouped, with what it does. Defaults are Sorter V2's.
export const SECTIONS = [
  {
    group: 'feeder',
    title: 'Feeder (C1–C3)',
    blurb: 'Pulses carry pieces along C2 and C3 and push them off each exit one at a time. Distances are rotor degrees.',
    fields: [
      { key: 'dropPulseDeg', label: 'Drop-zone pulse', unit: '°', step: 1, hint: 'How far a piece short of the exit is nudged each pulse. It always stops at the precise zone.' },
      { key: 'dropPauseMs', label: 'Pause after a drop-zone pulse', unit: 'ms', step: 10, hint: 'Lets the camera look again before the next nudge.' },
      { key: 'exitPulseDeg', label: 'Exit pulse', unit: '°', step: 0.5, hint: 'How far a piece at the exit is nudged. Smaller is gentler and less likely to push two through together.' },
      { key: 'exitPauseMs', label: 'Pause after an exit pulse', unit: 'ms', step: 10 },
      { key: 'holdAfterFallMs', label: 'Hold after a fall', unit: 'ms', step: 50, hint: 'After a piece falls off an exit, that exit pushes nothing for this long, so the next channel sees it land.' },
      { key: 'bulkPulseDeg', label: 'C1 bulk pulse', unit: '°', step: 1, hint: 'C1 has no camera: it turns this far whenever C2’s drop zone is clear.' },
      { key: 'bulkPauseMs', label: 'Pause after a C1 pulse', unit: 'ms', step: 100 },
      { key: 'minMoveDeg', label: 'Smallest move', unit: '°', step: 0.1, hint: 'Moves smaller than this are skipped as noise.' },
      { key: 'maxMoveDeg', label: 'Largest move', unit: '°', step: 1, hint: 'A cap on any single pulse.' },
      { key: 'staleMs', label: 'Camera frame too old after', unit: 'ms', step: 100, hint: 'An older frame counts as no information: the channel waits.' },
    ],
  },
  {
    group: 'platter',
    title: 'Classification platter (C4)',
    blurb: 'Pieces land in the sector under C3, are photographed and identified while the platter carries them, and drop into the chute.',
    fields: [
      { key: 'sectors', label: 'Sectors', step: 1, hint: 'Equal sectors; one index turns one sector.' },
      { key: 'exitSector', label: 'Indexes to the exit', step: 1, hint: 'A piece drops into the chute this many indexes after it lands.' },
      { key: 'settleMs', label: 'Settle after landing or an index', unit: 'ms', step: 50 },
      { key: 'burstFrames', label: 'Photos per piece', step: 1, hint: 'Sent to Brickognize together (up to 8).' },
      { key: 'identifyTimeoutMs', label: 'Brickognize timeout', unit: 'ms', step: 1000, hint: 'A piece still waiting when it reaches the exit goes to misc.' },
      { key: 'idleIndexMs', label: 'Index anyway after', unit: 'ms', step: 100, hint: 'With pieces aboard and nothing landing, index so they still get delivered.' },
      { key: 'stallMs', label: 'Stuck after', unit: 'ms', step: 1000, hint: 'No progress this long with pieces aboard: turn the channel round to clear it, then ask.' },
    ],
  },
  {
    group: 'pictures',
    title: 'Piece pictures',
    blurb:
      'Which frames of a tracked piece are good pictures of it for identifying. A frame must be whole, alone, settled, steady, its usual size and sharp. See each decision on the Piece journeys page.',
    fields: [
      { key: 'isolationPx', label: 'Alone: no other piece within', unit: 'px', step: 1, hint: 'Another piece’s box closer than this makes a frame “not alone”.' },
      { key: 'settleFrames', label: 'Settling: skip the first', unit: 'frames', step: 1, hint: 'While a piece lands and bounces.' },
      { key: 'maxDegPerFrame', label: 'Moving: at most', unit: '°/frame', step: 0.5, hint: 'Faster than this, the piece is moving too much for a sharp picture.' },
      { key: 'minSharpness', label: 'Sharp: at least', step: 5, hint: 'The crop’s detail (variance of its Laplacian). Look at the values on the Piece journeys page to set it.' },
      { key: 'sizeFactor', label: 'Size: within a factor of', unit: '×', step: 0.1, hint: 'Of the piece’s usual box area. A jump means it’s touching something or partly hidden.' },
      { key: 'keepEveryDeg', label: 'Keep one per', unit: '°', step: 1, hint: 'Kept pictures this far apart along the channel show the piece turned different ways; the sharpest in each stretch wins.' },
      { key: 'maxKept', label: 'Keep at most', unit: 'per channel', step: 1 },
      { key: 'cropMarginPct', label: 'Crop margin', unit: '%', step: 5, hint: 'Around the piece’s box, on each side.' },
      { key: 'cropMaxPx', label: 'Crop size at most', unit: 'px', step: 32, hint: 'The crop’s longer side.' },
      { key: 'keepJourneys', label: 'Keep debug pictures for', unit: 'pieces', step: 50, hint: 'The newest pieces’ journeys; older ones are deleted.' },
    ],
  },
];

export function useTuningPageLogic() {
  const [settings, setSettings, syncError] = useMachineSettings();

  const value = (group, key) => settings[group]?.[key] ?? DEFAULT_SORTER_SETTINGS[group][key];
  const defaultOf = (group, key) => DEFAULT_SORTER_SETTINGS[group][key];

  const set = (group, key, v) => setSettings((prev) => ({ ...prev, [group]: { ...prev[group], [key]: v } }));

  const resetAll = (group) => setSettings((prev) => ({ ...prev, [group]: { ...DEFAULT_SORTER_SETTINGS[group] } }));

  return { value, defaultOf, set, resetAll, error: syncError };
}
