import { useMemo } from 'react';

import { ROUTE_REASONS, useBinsPageLogic } from './BinsPage.logic';
import BinEditor from './components/BinEditor';
import IdentifyPanel from './components/IdentifyPanel';

import styles from './BinsPage.module.css';
import appStyles from '../../styles/app.module.css';

function binLabel(bin) {
  return `layer ${bin.layer}, bin ${bin.bin} (section ${bin.section})`;
}

function RouteTester({ route, setRoute, onTry, canTry }) {
  const { result } = route;
  const set = (key) => (e) => setRoute((prev) => ({ ...prev, [key]: e.target.value, result: null }));
  return (
    <div className={styles.panel}>
      <h3 className={styles.panelTitle}>Where would a piece go?</h3>
      <form
        className={styles.fields}
        onSubmit={(e) => {
          e.preventDefault();
          onTry();
        }}
      >
        <label className={styles.field}>
          <span className={styles.fieldLabel}>Part</span>
          <input value={route.part} onChange={set('part')} placeholder="3001" />
        </label>
        <label className={styles.field}>
          <span className={styles.fieldLabel}>Color id</span>
          <input value={route.color} onChange={set('color')} placeholder="5" />
        </label>
        <label className={styles.field}>
          <span className={styles.fieldLabel}>Longest side (mm)</span>
          <input type="number" min="0" value={route.size} onChange={set('size')} placeholder="optional" />
        </label>
        <button type="submit" className={appStyles.button} disabled={!canTry || !route.part.trim()}>
          Check
        </button>
      </form>
      {result && (
        <p className={styles.routeResult}>
          <strong>{result.category_name}</strong> → {result.choice.bin ? binLabel(result.choice.bin) : 'the bucket'}:{' '}
          {ROUTE_REASONS[result.choice.reason] ?? result.choice.reason}. Nothing was saved.
        </p>
      )}
    </div>
  );
}

const BinsPage = () => {
  const logic = useBinsPageLogic();
  const { view, settings, selected, selectedBin } = logic;

  // Which bins each category is in, for "now in …" in the editor.
  const binOf = useMemo(() => {
    const out = {};
    for (const layer of view?.layers ?? []) {
      for (const section of layer.sections) {
        for (const bin of section.bins) {
          for (const c of bin.categories) (out[c.id] ??= []).push({ layer: layer.layer, bin: bin.bin });
        }
      }
    }
    return out;
  }, [view]);

  if (!view) {
    return (
      <section className={appStyles.card}>
        <h2 className={styles.title}>Bins</h2>
        {logic.error ? <p className={styles.alert}>{logic.error}</p> : <p className={appStyles.muted}>Loading…</p>}
      </section>
    );
  }

  return (
    <section className={appStyles.card}>
      <div className={styles.headerRow}>
        <div>
          <h2 className={styles.title}>Bins</h2>
          <p className={appStyles.muted}>
            {view.profile ? (
              <>
                Sorting with <strong>{view.profile.name}</strong>. A category with no bin gets the first free one when its
                first piece arrives.
              </>
            ) : (
              'No profile is active. Activate one on the Profiles page to assign bins.'
            )}
          </p>
        </div>
        <div className={styles.buttonRow}>
          <label className={styles.checkField}>
            <input
              type="checkbox"
              checked={settings.distributor.sharedBins}
              onChange={(e) => logic.setDistributor({ sharedBins: e.target.checked })}
            />
            Share bins once every bin is taken
          </label>
          <button type="button" className={appStyles.buttonSecondary} disabled={!view.profile} onClick={logic.resetBins}>
            Unassign every bin
          </button>
        </div>
      </div>

      {logic.error && <p className={styles.alert}>{logic.error}</p>}

      {selectedBin && (
        <BinEditor
          key={`${selected.layer}-${selected.bin}`}
          bin={selectedBin}
          layer={selected.layer}
          categories={view.categories}
          binOf={binOf}
          onSave={logic.saveBin}
          onPoint={logic.pointChute}
          onEmpty={logic.emptyBin}
          onClose={() => logic.select(null)}
        />
      )}

      {view.layers.map((layer) => (
        <div key={layer.layer} className={layer.disabled ? styles.layerOff : styles.layer}>
          <div className={styles.layerHeader}>
            <h3 className={styles.panelTitle}>
              Layer {layer.layer}{' '}
              <span className={styles.muted}>
                {layer.bins} bins{layer.max_piece_mm ? `, pieces up to ${layer.max_piece_mm} mm` : ''}
              </span>
            </h3>
            <div className={styles.buttonRow}>
              <label className={styles.checkField}>
                Full at
                <input
                  type="number"
                  min="0"
                  className={styles.smallInput}
                  value={layer.max_pieces}
                  onChange={(e) => e.target.value !== '' && logic.setLayer(layer.layer, { maxPieces: Math.max(0, Math.round(Number(e.target.value))) })}
                />
                pieces (0: never)
              </label>
              <label className={styles.checkField}>
                <input type="checkbox" checked={!layer.disabled} onChange={(e) => logic.setLayer(layer.layer, { disabled: !e.target.checked })} />
                In use
              </label>
            </div>
          </div>
          {layer.sections.length === 0 ? (
            <p className={styles.muted}>{layer.bins} bins don&apos;t split into the chute&apos;s sections; fix the bin count on the LEGO Sorter page.</p>
          ) : (
            <div className={styles.sections}>
              {layer.sections.map((section) => (
                <div key={section.section} className={section.disabled ? styles.sectionOff : styles.section}>
                  <label className={styles.sectionHeader}>
                    <input type="checkbox" checked={!section.disabled} onChange={() => logic.toggleSection(layer.layer, section.section)} />
                    Section {section.section}
                  </label>
                  {section.bins.map((bin) => {
                    const isSelected = selected?.layer === layer.layer && selected?.bin === bin.bin;
                    return (
                      <button
                        key={bin.bin}
                        type="button"
                        className={`${styles.bin} ${isSelected ? styles.binSelected : ''} ${bin.reachable ? '' : styles.binUnreachable}`}
                        disabled={!view.profile}
                        onClick={() => logic.select({ layer: layer.layer, bin: bin.bin })}
                      >
                        <span className={styles.binNumber}>
                          {bin.bin}
                          {bin.count > 0 && <span className={bin.full ? styles.binFull : styles.binCount}>{bin.full ? `full · ${bin.count}` : bin.count}</span>}
                        </span>
                        <span className={bin.categories.length ? styles.binCats : styles.binEmpty}>
                          {bin.categories.length ? bin.categories.map((c) => c.name).join(', ') : bin.reachable ? 'Unassigned' : 'Out of reach'}
                        </span>
                      </button>
                    );
                  })}
                </div>
              ))}
            </div>
          )}
        </div>
      ))}

      {logic.kits.length > 0 && (
        <div className={styles.panel}>
          <h3 className={styles.panelTitle}>Kits</h3>
          {logic.kits.map((k) => (
            <div key={k.category} className={styles.kit}>
              <span>{k.name}</span>
              <progress max={k.need} value={k.found} />
              <span className={styles.muted}>
                {k.found} of {k.need}
              </span>
            </div>
          ))}
        </div>
      )}

      <RouteTester route={logic.route} setRoute={logic.setRoute} onTry={logic.tryRoute} canTry={Boolean(view.profile)} />
      <IdentifyPanel />
    </section>
  );
};

export default BinsPage;
