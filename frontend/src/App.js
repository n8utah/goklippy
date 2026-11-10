import { useState } from 'react';
import AppNav from './components/AppNav/AppNav';
import { MCUSessionProvider } from './context/MCUSessionContext';
import HomePage from './pages/HomePage/HomePage';
import DevicesPage from './pages/DevicesPage/DevicesPage';
import BoardSetupPage from './pages/BoardSetupPage/BoardSetupPage';
import BoardEditorPage from './pages/BoardEditorPage/BoardEditorPage';
import SorterPage from './pages/SorterPage/SorterPage';
import ChutePage from './pages/ChutePage/ChutePage';
import BinsPage from './pages/BinsPage/BinsPage';
import ProfilesPage from './pages/ProfilesPage/ProfilesPage';
import DashboardPage from './pages/DashboardPage/DashboardPage';
import RecordsPage from './pages/RecordsPage/RecordsPage';
import CamerasPage from './pages/CamerasPage/CamerasPage';
import styles from './styles/app.module.css';

const App = () => {
  const [page, setPage] = useState('board');
  const [pieceId, setPieceId] = useState(null);

  const openPiece = (piece) => {
    setPieceId(piece.id);
    setPage('records');
  };

  return (
    <MCUSessionProvider>
      <div className={styles.appPage}>
        <div className={styles.appShell}>
          <header className={styles.header}>
            <h1 className={styles.title}>goklippy</h1>
            <p className={styles.subtitle}>Klipper MCU workbench — visual board setup and config editing</p>
          </header>
          <AppNav page={page} onChange={setPage} />
          {page === 'dashboard' && <DashboardPage onOpenPiece={openPiece} />}
          {page === 'records' && <RecordsPage key={pieceId || 'records'} initialPieceId={pieceId} />}
          {page === 'cameras' && <CamerasPage />}
          {page === 'home' && <HomePage />}
          {page === 'devices' && <DevicesPage onGoBoard={() => setPage('board')} />}
          {page === 'board' && <BoardSetupPage />}
          {page === 'editor' && <BoardEditorPage />}
          {page === 'sorter' && <SorterPage onGoBoard={() => setPage('board')} />}
          {page === 'chute' && <ChutePage />}
          {page === 'bins' && <BinsPage />}
          {page === 'profiles' && <ProfilesPage />}
        </div>
      </div>
    </MCUSessionProvider>
  );
};

export default App;
