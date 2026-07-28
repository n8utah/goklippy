import EditableBoardDiagram from './components/EditableBoardDiagram/EditableBoardDiagram';
import { useBoardEditorPageLogic } from './BoardEditorPage.logic';

import styles from './BoardEditorPage.module.css';
import appStyles from '../../styles/app.module.css';

const numberField = (label, value, onChange) => (
  <label className={styles.field}>
    <span className={styles.fieldLabel}>{label}</span>
    <input
      className={styles.input}
      type="number"
      value={Number.isFinite(value) ? value : 0}
      onChange={(event) => onChange(Number(event.target.value))}
    />
  </label>
);

const BoardEditorPage = () => {
  const {
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
  } = useBoardEditorPageLogic();

  return (
    <section className={appStyles.card}>
      <div className={styles.headerRow}>
        <div>
          <h2 className={styles.title}>Board Overlay Editor</h2>
          <p className={appStyles.muted}>
            Drag overlays on the board image, add or remove known slots, then save as the default profile map.
          </p>
        </div>
        <div className={styles.toolbar}>
          <label className={styles.selectWrap}>
            <span className={styles.fieldLabel}>Board</span>
            <select
              className={styles.select}
              value={profileId}
              onChange={(event) => setProfileId(event.target.value)}
            >
              {profileIds.map((id) => (
                <option key={id} value={id}>
                  {id}
                </option>
              ))}
            </select>
          </label>
          <button
            type="button"
            className={appStyles.button}
            onClick={saveHotspots}
            disabled={saving || !dirty || loading}
          >
            {saving ? 'Saving…' : dirty ? 'Save defaults' : 'Saved'}
          </button>
        </div>
      </div>

      {error && <p className={appStyles.errorText}>{error}</p>}
      {message && <p className={styles.successText}>{message}</p>}

      <div className={styles.layout}>
        <div className={styles.canvasCol}>
          <p className={appStyles.muted}>
            {profileName || profileId} — drag to move; drag corner handles to resize
          </p>
          {loading ? (
            <div className={styles.loadingBox}>Loading editor…</div>
          ) : (
            <EditableBoardDiagram
              diagram={diagram}
              profileId={profileId}
              hotspots={hotspots}
              selectedId={selectedId}
              onSelect={setSelectedId}
              onBeginDrag={beginDrag}
              onBeginResize={beginResize}
              onMoveDrag={moveDrag}
              onEndDrag={endDrag}
            />
          )}
        </div>

        <aside className={styles.sidebar}>
          <section className={styles.panel}>
            <h3 className={styles.panelTitle}>Overlays ({hotspots.length})</h3>
            <ul className={styles.list}>
              {hotspots.length === 0 && <li className={appStyles.muted}>No overlays yet.</li>}
              {hotspots.map((hotspot) => (
                <li key={hotspot.id}>
                  <button
                    type="button"
                    className={selectedId === hotspot.id ? styles.listButtonActive : styles.listButton}
                    onClick={() => setSelectedId(hotspot.id)}
                  >
                    {hotspot.label}
                  </button>
                </li>
              ))}
            </ul>
          </section>

          {selectedHotspot && (
            <section className={styles.panel}>
              <h3 className={styles.panelTitle}>Selected</h3>
              <p className={styles.meta}>ID: {selectedHotspot.id}</p>
              <p className={styles.meta}>Slots: {(selectedHotspot.slot_ids || []).join(', ')}</p>
              <label className={styles.field}>
                <span className={styles.fieldLabel}>Label</span>
                <input
                  className={styles.input}
                  value={selectedHotspot.label || ''}
                  onChange={(event) => updateHotspot(selectedHotspot.id, { label: event.target.value })}
                />
              </label>
              {selectedHotspot.shape !== 'circle' && (
                <div className={styles.fieldGrid}>
                  {numberField('X', selectedHotspot.x, (v) => updateHotspot(selectedHotspot.id, { x: v }))}
                  {numberField('Y', selectedHotspot.y, (v) => updateHotspot(selectedHotspot.id, { y: v }))}
                  {numberField('Width', selectedHotspot.width, (v) =>
                    updateHotspot(selectedHotspot.id, { width: v }),
                  )}
                  {numberField('Height', selectedHotspot.height, (v) =>
                    updateHotspot(selectedHotspot.id, { height: v }),
                  )}
                </div>
              )}
              <button
                type="button"
                className={appStyles.buttonSecondary}
                onClick={() => removeHotspot(selectedHotspot.id)}
              >
                Remove overlay
              </button>
            </section>
          )}

          <section className={styles.panel}>
            <h3 className={styles.panelTitle}>Add from board slots</h3>
            {availableSlots.length === 0 && (
              <p className={appStyles.muted}>All known slots already have overlays.</p>
            )}
            <ul className={styles.list}>
              {availableSlots.map((slot) => (
                <li key={slot.id}>
                  <button
                    type="button"
                    className={styles.addButton}
                    onClick={() => addHotspotForSlot(slot.id)}
                  >
                    + {slot.label} <span className={styles.slotType}>({slot.type})</span>
                  </button>
                </li>
              ))}
            </ul>
          </section>
        </aside>
      </div>
    </section>
  );
};

export default BoardEditorPage;
