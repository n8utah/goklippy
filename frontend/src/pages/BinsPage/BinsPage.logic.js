import { useCallback, useEffect, useState } from 'react';

import { emptySorterBin, getSorterBins, getSorterKits, resetSorterBins, routeSorterPiece, sendToSorterBin, setSorterBin } from '../../helpers/api';
import { useMachineSettings } from '../../helpers/sorterSettings';

export const ROUTE_REASONS = {
  assigned_bin: 'its category already has this bin',
  new_bin: 'its category gets this empty bin',
  shared_bin: 'its category shares this bin (every bin is taken)',
  misc_bin: 'no bin for its category, so the misc bin',
  misc: 'misc, with no misc bin: the bucket under the tower',
  too_big: 'longer than any bin takes: the bucket',
  no_bin: 'every bin is taken: the bucket',
};

export function useBinsPageLogic() {
  const [settings, setSettings, syncError, savedCount] = useMachineSettings();
  const [view, setView] = useState(null); // GET /sorter/bins
  const [selected, setSelected] = useState(null); // { layer, bin }
  const [error, setError] = useState('');
  const [route, setRoute] = useState({ part: '', color: '', size: '', result: null });
  const [kits, setKits] = useState([]);

  // Layer and section switches live in the machine settings, so read the bins
  // again after each save.
  useEffect(() => {
    let cancelled = false;
    getSorterKits().then(
      (data) => {
        if (!cancelled) setKits(data.kits);
      },
      () => {},
    );
    getSorterBins().then(
      (data) => {
        if (!cancelled) setView(data);
      },
      (err) => {
        if (!cancelled) setError(err.message);
      },
    );
    return () => {
      cancelled = true;
    };
  }, [savedCount]);

  const guard = useCallback(async (fn) => {
    setError('');
    try {
      await fn();
    } catch (err) {
      setError(err.message);
    }
  }, []);

  const selectedBin =
    selected &&
    view?.layers[selected.layer - 1]?.sections.flatMap((s) => s.bins).find((b) => b.bin === selected.bin);

  const saveBin = (categories) =>
    guard(async () => {
      setView(await setSorterBin(selected.layer, selected.bin, categories));
    });

  const resetBins = () =>
    guard(async () => {
      setView(await resetSorterBins());
      setSelected(null);
    });

  const pointChute = () => guard(() => sendToSorterBin(selected.layer, selected.bin));

  /** The bin was emptied: it counts from zero and takes pieces again. */
  const emptyBin = () =>
    guard(async () => {
      setView(await emptySorterBin(selected.layer, selected.bin));
    });

  const setDistributor = (patch) =>
    setSettings((prev) => ({ ...prev, distributor: { ...prev.distributor, ...patch } }));

  const setLayer = (layerNumber, patch) =>
    setDistributor({
      layers: settings.distributor.layers.map((l, i) => (i === layerNumber - 1 ? { ...l, ...patch } : l)),
    });

  const toggleSection = (layerNumber, section) => {
    const layer = settings.distributor.layers[layerNumber - 1];
    const off = layer.disabledSections || [];
    setLayer(layerNumber, {
      disabledSections: off.includes(section) ? off.filter((s) => s !== section) : [...off, section].sort((a, b) => a - b),
    });
  };

  const tryRoute = () =>
    guard(async () => {
      const result = await routeSorterPiece(route.part.trim(), route.color.trim(), Number(route.size) || 0);
      setRoute((prev) => ({ ...prev, result }));
    });

  return {
    view,
    settings,
    error: error || syncError,
    selected,
    selectedBin,
    select: setSelected,
    saveBin,
    resetBins,
    pointChute,
    emptyBin,
    kits,
    setDistributor,
    setLayer,
    toggleSection,
    route,
    setRoute,
    tryRoute,
  };
}
