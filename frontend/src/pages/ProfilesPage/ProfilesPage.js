import { useState } from 'react';

import { ACTIVATION_MODES, useProfilesPageLogic } from './ProfilesPage.logic';

import styles from './ProfilesPage.module.css';
import appStyles from '../../styles/app.module.css';

function ProfileRow({ profile, busy, onActivate, onDelete }) {
  const [mode, setMode] = useState('keep');
  const { summary } = profile;
  const hint = ACTIVATION_MODES.find((m) => m.mode === mode)?.hint;

  return (
    <li className={profile.active ? styles.rowActive : styles.row}>
      <div className={styles.rowMain}>
        <div className={styles.rowTitle}>
          <strong>{profile.name}</strong>
          {profile.active && <span className={styles.badge}>Active</span>}
          {summary.set_based && <span className={styles.badgeMuted}>Set</span>}
        </div>
        <div className={styles.rowFacts}>
          {summary.categories.length} categories · {summary.part_count.toLocaleString()} parts ·{' '}
          {summary.format === 'program' ? 'compiled program' : 'part map'}
          {summary.fallback_by_color && ' · rest by color'} · added {new Date(profile.created_at).toLocaleDateString()}
        </div>
        {summary.description && <p className={styles.description}>{summary.description}</p>}
      </div>
      <div className={styles.rowActions}>
        <select value={mode} onChange={(e) => setMode(e.target.value)} disabled={busy} aria-label="What happens to the bins">
          {ACTIVATION_MODES.map((m) => (
            <option key={m.mode} value={m.mode}>
              {m.label}
            </option>
          ))}
        </select>
        <button type="button" className={appStyles.button} disabled={busy} onClick={() => onActivate(profile, mode)} title={hint}>
          {profile.active ? 'Re-activate' : 'Activate'}
        </button>
        <button type="button" className={appStyles.buttonSecondary} disabled={busy || profile.active} onClick={() => onDelete(profile)}>
          Delete
        </button>
      </div>
    </li>
  );
}

const ProfilesPage = () => {
  const logic = useProfilesPageLogic();

  return (
    <section className={appStyles.card}>
      <h2 className={styles.title}>Sorting profiles</h2>
      <p className={appStyles.muted}>
        A profile says which category each part goes in; the machine decides which bin each category gets (see the Bins
        page). Upload a <code>sorting_profile.json</code> built in Hive.
      </p>

      <label className={styles.upload}>
        <span className={appStyles.button}>{logic.busy ? 'Working…' : 'Upload a profile'}</span>
        <input
          type="file"
          accept=".json,application/json"
          disabled={logic.busy}
          onChange={(e) => {
            const file = e.target.files?.[0];
            e.target.value = '';
            if (file) logic.upload(file);
          }}
        />
      </label>

      {logic.error && <p className={styles.alert}>{logic.error}</p>}
      {logic.notice && <p className={styles.notice}>{logic.notice}</p>}

      {logic.loading ? (
        <p className={appStyles.muted}>Loading…</p>
      ) : logic.profiles.length === 0 ? (
        <p className={appStyles.muted}>No profiles yet.</p>
      ) : (
        <ul className={styles.list}>
          {logic.profiles.map((p) => (
            <ProfileRow key={p.id} profile={p} busy={logic.busy} onActivate={logic.activate} onDelete={logic.remove} />
          ))}
        </ul>
      )}
    </section>
  );
};

export default ProfilesPage;
