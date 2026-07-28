import { useCallback, useEffect, useMemo, useRef, useState } from 'react';

import {
  getBoardProfileEditor,
  listBoardProfiles,
  updateBoardProfileHotspots,
} from '../../helpers/api';
import { acceptsForSlotType, defaultHotspotForSlot, resizeCircle, resizeRect } from '../../helpers/boardHotspots';

export function useBoardEditorPageLogic() {
  const [profileIds, setProfileIds] = useState([]);
  const [profileId, setProfileId] = useState('ramps14');
  const [profileName, setProfileName] = useState('');
  const [diagram, setDiagram] = useState(null);
  const [hotspots, setHotspots] = useState([]);
  const [slots, setSlots] = useState({});
  const [availableSlots, setAvailableSlots] = useState([]);
  const [selectedId, setSelectedId] = useState(null);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [dirty, setDirty] = useState(false);
  const [error, setError] = useState('');
  const [message, setMessage] = useState('');

  const dragRef = useRef(null);

  useEffect(() => {
    listBoardProfiles()
      .then((data) => setProfileIds(data.profiles || []))
      .catch((err) => setError(err.message));
  }, []);

  const loadEditor = useCallback(async (id) => {
    setLoading(true);
    setError('');
    setMessage('');
    try {
      const data = await getBoardProfileEditor(id);
      setProfileName(data.profile_name || id);
      setDiagram(data.diagram);
      setHotspots(data.hotspots || []);
      setSlots(data.slots || {});
      setAvailableSlots(data.available_slots || []);
      setSelectedId(null);
      setDirty(false);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (profileId) {
      loadEditor(profileId);
    }
  }, [profileId, loadEditor]);

  const selectedHotspot = useMemo(
    () => hotspots.find((h) => h.id === selectedId) || null,
    [hotspots, selectedId],
  );

  const updateHotspot = useCallback((id, patch) => {
    setHotspots((prev) => prev.map((h) => (h.id === id ? { ...h, ...patch } : h)));
    setDirty(true);
  }, []);

  const removeHotspot = useCallback((id) => {
    setHotspots((prev) => {
      const removed = prev.find((h) => h.id === id);
      const next = prev.filter((h) => h.id !== id);
      if (removed?.slot_ids?.[0]) {
        const slotId = removed.slot_ids[0];
        const slot = slots[slotId];
        if (slot) {
          setAvailableSlots((avail) =>
            [...avail, { id: slotId, label: slot.label, type: slot.type, group: slot.group }].sort(
              (a, b) => a.label.localeCompare(b.label),
            ),
          );
        }
      }
      return next;
    });
    if (selectedId === id) {
      setSelectedId(null);
    }
    setDirty(true);
  }, [selectedId, slots]);

  const addHotspotForSlot = useCallback(
    (slotId) => {
      const slot = slots[slotId];
      if (!slot || !diagram) {
        return;
      }
      const hotspot = defaultHotspotForSlot(slotId, slot, diagram, hotspots);
      setHotspots((prev) => [...prev, hotspot]);
      setAvailableSlots((prev) => prev.filter((s) => s.id !== slotId));
      setSelectedId(hotspot.id);
      setDirty(true);
    },
    [diagram, hotspots, slots],
  );

  const beginDrag = useCallback((id, offsetX, offsetY, shape = 'rect') => {
    dragRef.current = { mode: 'move', id, offsetX, offsetY, shape };
    setSelectedId(id);
  }, []);

  const beginResize = useCallback((id, handle, hotspot) => {
    dragRef.current = {
      mode: 'resize',
      id,
      handle,
      start: { ...hotspot },
    };
    setSelectedId(id);
  }, []);

  const moveDrag = useCallback(
    (svgX, svgY) => {
      const drag = dragRef.current;
      if (!drag) {
        return;
      }
      if (drag.mode === 'move') {
        if (drag.shape === 'circle') {
          updateHotspot(drag.id, {
            cx: Math.max(0, svgX - drag.offsetX),
            cy: Math.max(0, svgY - drag.offsetY),
          });
        } else {
          updateHotspot(drag.id, {
            x: Math.max(0, svgX - drag.offsetX),
            y: Math.max(0, svgY - drag.offsetY),
          });
        }
        return;
      }
      if (drag.mode === 'resize') {
        const { start, handle, id } = drag;
        if (start.shape === 'circle') {
          updateHotspot(id, resizeCircle(start, svgX, svgY));
        } else {
          updateHotspot(id, resizeRect(handle, start, svgX, svgY));
        }
      }
    },
    [updateHotspot],
  );

  const endDrag = useCallback(() => {
    dragRef.current = null;
  }, []);

  const saveHotspots = async () => {
    setSaving(true);
    setError('');
    setMessage('');
    try {
      const data = await updateBoardProfileHotspots(profileId, hotspots);
      setHotspots(data.hotspots || hotspots);
      setDirty(false);
      setMessage('Board overlay defaults saved.');
      await loadEditor(profileId);
    } catch (err) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  };

  return {
    profileIds,
    profileId,
    setProfileId,
    profileName,
    diagram,
    hotspots,
    availableSlots,
    selectedHotspot,
    selectedId,
    setSelectedId,
    updateHotspot,
    removeHotspot,
    addHotspotForSlot,
    beginDrag,
    beginResize,
    moveDrag,
    endDrag,
    saveHotspots,
    loading,
    saving,
    dirty,
    error,
    message,
  };
}
