import {
  BrowserRouter,
  Link,
  Navigate,
  Route,
  Routes,
  useSearchParams,
} from 'react-router';

import AppFrame from './components/AppFrame/AppFrame';
import { MCUSessionProvider } from './context/MCUSessionContext';
import { MachineSettingsProvider } from './helpers/sorterSettings';
import { SorterStateProvider } from './helpers/sorterState';
import AccountPage from './pages/AccountPage/AccountPage';
import BenchPage from './pages/BenchPage/BenchPage';
import BinsPage from './pages/BinsPage/BinsPage';
import BoardPage from './pages/BoardPage/BoardPage';
import CamerasPage from './pages/CamerasPage/CamerasPage';
import ChannelsPage from './pages/ChannelsPage/ChannelsPage';
import ChutePage from './pages/ChutePage/ChutePage';
import HomePage from './pages/HomePage/HomePage';
import JourneysPage from './pages/JourneysPage/JourneysPage';
import ProfilesPage from './pages/ProfilesPage/ProfilesPage';
import RecordsPage from './pages/RecordsPage/RecordsPage';
import SettingsPage from './pages/SettingsPage/SettingsPage';
import SortPage from './pages/SortPage/SortPage';
import UiKitPage from './pages/UiKitPage/UiKitPage';

/** A page from before the new UI: always light until build step 4 rebuilds it. */
const Legacy = ({ children }) => <div className="legacy-page">{children}</div>;

const RecordsRoute = () => {
  const [params] = useSearchParams();
  const piece = params.get('piece');
  return (
    <Legacy>
      <RecordsPage key={piece || 'records'} initialPieceId={piece} />
    </Legacy>
  );
};

const NotFound = () => (
  <div className="ds rounded-xl border border-border bg-card p-6">
    <p className="font-display text-lg font-semibold">No page here</p>
    <p className="mt-1 text-sm text-muted-foreground">
      This address isn’t a page in goklippy. Search with Ctrl K, or{' '}
      <Link to="/" className="font-medium text-primary-text underline">
        go Home
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
        <MachineSettingsProvider>
          <Routes>
            <Route element={<AppFrame />}>
              <Route index element={<HomePage />} />

              <Route path="run">
                <Route index element={<Navigate to="sort" replace />} />
                <Route path="sort" element={<SortPage />} />
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
                {/* One route, so the page keeps its state from tab to tab. */}
                <Route path="board/:part?" element={<BoardPage />} />
                <Route path="channels/:part?" element={<ChannelsPage />} />
                <Route path="chute/:part?" element={<ChutePage />} />
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
                <Route path="bench" element={<BenchPage />} />
              </Route>

              <Route path="system">
                <Route index element={<Navigate to="settings" replace />} />
                <Route path="settings" element={<SettingsPage />} />
                <Route path="settings/:section" element={<SettingsPage />} />
                <Route
                  path="account"
                  element={
                    <Legacy>
                      <AccountPage />
                    </Legacy>
                  }
                />
                <Route path="ui-kit" element={<UiKitPage />} />
              </Route>

              <Route path="*" element={<NotFound />} />
            </Route>
          </Routes>
        </MachineSettingsProvider>
      </SorterStateProvider>
    </MCUSessionProvider>
  </BrowserRouter>
);

export default App;
