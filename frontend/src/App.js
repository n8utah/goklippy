import {
  BrowserRouter,
  Link,
  Navigate,
  Route,
  Routes,
  useNavigate,
  useParams,
  useSearchParams,
} from 'react-router';

import AppFrame from './components/AppFrame/AppFrame';
import { MCUSessionProvider } from './context/MCUSessionContext';
import { START_PATH } from './helpers/navigation';
import { SorterStateProvider } from './helpers/sorterState';
import BinsPage from './pages/BinsPage/BinsPage';
import BoardEditorPage from './pages/BoardEditorPage/BoardEditorPage';
import BoardSetupPage from './pages/BoardSetupPage/BoardSetupPage';
import CamerasPage from './pages/CamerasPage/CamerasPage';
import ChutePage from './pages/ChutePage/ChutePage';
import DashboardPage from './pages/DashboardPage/DashboardPage';
import DevicesPage from './pages/DevicesPage/DevicesPage';
import HomePage from './pages/HomePage/HomePage';
import JourneysPage from './pages/JourneysPage/JourneysPage';
import ProfilesPage from './pages/ProfilesPage/ProfilesPage';
import RecordsPage from './pages/RecordsPage/RecordsPage';
import SorterPage from './pages/SorterPage/SorterPage';
import TuningPage from './pages/TuningPage/TuningPage';
import UiKitPage from './pages/UiKitPage/UiKitPage';

const WIRING_PATH = '/machine/board/wiring';

/** A page from before the new UI: always light until build step 4 rebuilds it. */
const Legacy = ({ children }) => <div className="legacy-page">{children}</div>;

const SortRoute = () => {
  const navigate = useNavigate();
  return (
    <Legacy>
      <DashboardPage onOpenPiece={(piece) => navigate(`/run/records?piece=${piece.id}`)} />
    </Legacy>
  );
};

const RecordsRoute = () => {
  const [params] = useSearchParams();
  const piece = params.get('piece');
  return (
    <Legacy>
      <RecordsPage key={piece || 'records'} initialPieceId={piece} />
    </Legacy>
  );
};

/** Machine › Board: Connection, Wiring and Board picture, today's three pages. */
const BoardRoute = () => {
  const navigate = useNavigate();
  const { part = 'connection' } = useParams();
  if (part === 'wiring')
    return (
      <Legacy>
        <BoardSetupPage />
      </Legacy>
    );
  if (part === 'picture')
    return (
      <Legacy>
        <BoardEditorPage />
      </Legacy>
    );
  if (part !== 'connection') return <Navigate to="/machine/board" replace />;
  return (
    <Legacy>
      <DevicesPage onGoBoard={() => navigate(WIRING_PATH)} />
    </Legacy>
  );
};

const ChannelsRoute = () => {
  const navigate = useNavigate();
  return (
    <Legacy>
      <SorterPage onGoBoard={() => navigate(WIRING_PATH)} />
    </Legacy>
  );
};

const NotFound = () => (
  <div className="ds rounded-xl border border-border bg-card p-6">
    <p className="font-display text-lg font-semibold">No page here</p>
    <p className="mt-1 text-sm text-muted-foreground">
      This address isn’t a page in goklippy. Search with Ctrl K, or{' '}
      <Link to={START_PATH} className="font-medium text-primary-text underline">
        go to Run › Sort
      </Link>
      .
    </p>
  </div>
);

/** URLs are /<group>/<page>/<part> (docs/ui-ux-spec.md, "Pages"). */
const App = () => (
  <BrowserRouter>
    <MCUSessionProvider>
      <SorterStateProvider>
        <Routes>
          <Route element={<AppFrame />}>
            <Route index element={<Navigate to={START_PATH} replace />} />

            <Route path="run">
              <Route index element={<Navigate to="sort" replace />} />
              <Route path="sort" element={<SortRoute />} />
              <Route path="records" element={<RecordsRoute />} />
            </Route>

            <Route path="rules">
              <Route index element={<Navigate to="profiles" replace />} />
              <Route
                path="profiles"
                element={
                  <Legacy>
                    <ProfilesPage />
                  </Legacy>
                }
              />
              <Route
                path="bins"
                element={
                  <Legacy>
                    <BinsPage />
                  </Legacy>
                }
              />
            </Route>

            <Route path="machine">
              <Route index element={<Navigate to="board" replace />} />
              <Route path="board" element={<BoardRoute />} />
              <Route path="board/:part" element={<BoardRoute />} />
              <Route path="channels" element={<ChannelsRoute />} />
              <Route
                path="chute"
                element={
                  <Legacy>
                    <ChutePage />
                  </Legacy>
                }
              />
              <Route
                path="cameras"
                element={
                  <Legacy>
                    <CamerasPage />
                  </Legacy>
                }
              />
            </Route>

            <Route path="diagnose">
              <Route index element={<Navigate to="journeys" replace />} />
              <Route
                path="journeys"
                element={
                  <Legacy>
                    <JourneysPage />
                  </Legacy>
                }
              />
            </Route>

            <Route path="system">
              <Route index element={<Navigate to="settings" replace />} />
              <Route
                path="settings"
                element={
                  <Legacy>
                    <TuningPage />
                  </Legacy>
                }
              />
              <Route
                path="account"
                element={
                  <Legacy>
                    <HomePage />
                  </Legacy>
                }
              />
              <Route path="ui-kit" element={<UiKitPage />} />
            </Route>

            <Route path="*" element={<NotFound />} />
          </Route>
        </Routes>
      </SorterStateProvider>
    </MCUSessionProvider>
  </BrowserRouter>
);

export default App;
