import styles from '../../styles/app.module.css';

const AppNav = ({ page, onChange }) => {
  const items = [
    { id: 'dashboard', label: 'Dashboard' },
    { id: 'devices', label: 'Devices' },
    { id: 'board', label: 'Board Setup' },
    { id: 'editor', label: 'Overlay Editor' },
    { id: 'sorter', label: 'LEGO Sorter' },
    { id: 'chute', label: 'Chute' },
    { id: 'cameras', label: 'Cameras' },
    { id: 'tuning', label: 'Tuning' },
    { id: 'bins', label: 'Bins' },
    { id: 'profiles', label: 'Profiles' },
    { id: 'records', label: 'Records' },
    { id: 'journeys', label: 'Piece journeys' },
    { id: 'uikit', label: 'UI kit' },
    { id: 'home', label: 'Auth / Dev' },
  ];

  return (
    <nav className={styles.nav}>
      {items.map((item) => (
        <button
          key={item.id}
          type="button"
          className={page === item.id ? styles.navButtonActive : styles.navButton}
          onClick={() => onChange(item.id)}
        >
          {item.label}
        </button>
      ))}
    </nav>
  );
};

export default AppNav;
