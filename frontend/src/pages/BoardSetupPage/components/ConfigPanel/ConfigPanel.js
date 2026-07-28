import styles from './ConfigPanel.module.css';
import appStyles from '../../../../styles/app.module.css';

const ConfigPanel = ({
  files,
  workspace,
  rawText,
  loading,
  error,
  onLoadFile,
  onSave,
  onRawChange,
  onApplyRaw,
}) => {
  return (
    <div className={styles.wrap}>
      <div className={styles.sidebar}>
        <h3 className={styles.heading}>Config files</h3>
        {files.length === 0 && <p className={appStyles.muted}>No config files in config_root.</p>}
        <ul className={styles.fileList}>
          {files.map((file) => (
            <li key={file}>
              <button type="button" className={styles.fileButton} onClick={() => onLoadFile(file)} disabled={loading}>
                {file}
              </button>
            </li>
          ))}
        </ul>
        {workspace && (
          <p className={styles.workspaceMeta}>
            Loaded: {workspace.path}
            {workspace.dirty ? ' (unsaved)' : ''}
          </p>
        )}
      </div>

      <div className={styles.editor}>
        <div className={styles.toolbar}>
          <button type="button" className={appStyles.buttonSecondary} onClick={onApplyRaw} disabled={loading}>
            Apply edits
          </button>
          <button type="button" className={appStyles.button} onClick={onSave} disabled={loading || !workspace}>
            Save to disk
          </button>
        </div>
        {error && <p className={appStyles.errorText}>{error}</p>}
        <textarea
          className={styles.textarea}
          value={rawText}
          onChange={(event) => onRawChange(event.target.value)}
          placeholder="Load a config file or paste Klipper printer.cfg content here…"
          spellCheck={false}
        />
      </div>
    </div>
  );
};

export default ConfigPanel;
