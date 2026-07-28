import styles from '../../styles/app.module.css';

const AppNav = ({ page, onChange }) => {
  const items = [
    { id: 'devices', label: 'Devices' },
    { id: 'board', label: 'Board Setup' },
    { id: 'editor', label: 'Overlay Editor' },
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
