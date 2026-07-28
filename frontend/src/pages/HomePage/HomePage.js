import { useHomePageLogic } from './HomePage.logic';

import styles from './HomePage.module.css';

const HomePage = () => {
  const {
    email,
    setEmail,
    password,
    setPassword,
    tokenPair,
    profile,
    error,
    loading,
    handleLogin,
    handleRefresh,
    handleLogout,
    handleLoadProfile,
  } = useHomePageLogic();

  return (
    <main className={styles.page}>
      <section className={styles.card}>
        <h1 className={styles.title}>goklippy Framework Starter</h1>
        <p className={styles.subtitle}>Backend + frontend build check with JWT auth flow endpoints.</p>

        <label className={styles.label} htmlFor="email-input">
          Email
        </label>
        <input
          id="email-input"
          className={styles.input}
          type="email"
          value={email}
          onChange={(event) => setEmail(event.target.value)}
          placeholder="user@example.com"
        />

        <label className={styles.label} htmlFor="password-input">
          Password
        </label>
        <input
          id="password-input"
          className={styles.input}
          type="password"
          value={password}
          onChange={(event) => setPassword(event.target.value)}
          placeholder="password"
        />

        <div className={styles.actions}>
          <button className={styles.button} type="button" onClick={handleLogin} disabled={loading}>
            Login
          </button>
          <button className={styles.button} type="button" onClick={handleRefresh} disabled={loading}>
            Refresh
          </button>
          <button className={styles.button} type="button" onClick={handleLoadProfile} disabled={loading}>
            Load Profile
          </button>
          <button className={styles.buttonSecondary} type="button" onClick={handleLogout} disabled={loading}>
            Logout
          </button>
        </div>

        {error && <p className={styles.errorText}>{error}</p>}

        {tokenPair && (
          <div className={styles.tokenPreview}>
            <p className={styles.tokenHeading}>Access Token</p>
            <code className={styles.tokenValue}>{tokenPair.access_token}</code>
            <p className={styles.tokenHeading}>Refresh Token</p>
            <code className={styles.tokenValue}>{tokenPair.refresh_token}</code>
          </div>
        )}

        {profile && (
          <div className={styles.profileCard}>
            <p className={styles.tokenHeading}>Authenticated User</p>
            <p className={styles.profileItem}>User ID: {profile.user_id}</p>
            <p className={styles.profileItem}>Email: {profile.email}</p>
            <p className={styles.profileItem}>Role: {profile.role}</p>
          </div>
        )}
      </section>
    </main>
  );
};

export default HomePage;
