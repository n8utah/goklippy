import { BrowserRouter, Link, Navigate, Route, Routes } from 'react-router';

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
import OverviewPage from './pages/OverviewPage/OverviewPage';
import ProfilesPage from './pages/ProfilesPage/ProfilesPage';
import RecordsPage from './pages/RecordsPage/RecordsPage';
import SettingsPage from './pages/SettingsPage/SettingsPage';
import SortPage from './pages/SortPage/SortPage';
import UiKitPage from './pages/UiKitPage/UiKitPage';

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
                <Route path="records/:part?" element={<RecordsPage />} />
              </Route>

              <Route path="rules">
                <Route index element={<Navigate to="profiles" replace />} />
                <Route path="profiles" element={<ProfilesPage />} />
                <Route path="bins/:part?" element={<BinsPage />} />
              </Route>

              <Route path="machine">
                <Route index element={<OverviewPage />} />
                {/* One route, so the page keeps its state from tab to tab. */}
                <Route path="board/:part?" element={<BoardPage />} />
                <Route path="channels/:part?" element={<ChannelsPage />} />
                <Route path="chute/:part?" element={<ChutePage />} />
                <Route path="cameras/:part?" element={<CamerasPage />} />
              </Route>

              <Route path="diagnose">
                <Route index element={<Navigate to="journeys" replace />} />
                <Route path="journeys" element={<JourneysPage />} />
                <Route path="bench" element={<BenchPage />} />
              </Route>

              <Route path="system">
                <Route index element={<Navigate to="settings" replace />} />
                <Route path="settings" element={<SettingsPage />} />
                <Route path="settings/:section" element={<SettingsPage />} />
                <Route path="account" element={<AccountPage />} />
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
