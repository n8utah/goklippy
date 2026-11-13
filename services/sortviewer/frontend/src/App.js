import { BrowserRouter, Navigate, Route, Routes } from 'react-router';

import AppShell from '@/components/AppShell/AppShell';
import { Toaster } from '@/components/ui/sonner';
import { SessionProvider, useSession } from '@/helpers/session';
import AdminPage from '@/pages/AdminPage/AdminPage';
import CollectionPage from '@/pages/CollectionPage/CollectionPage';
import HomePage from '@/pages/HomePage/HomePage';
import LoginPage from '@/pages/LoginPage/LoginPage';

function Routed() {
  const { me } = useSession();
  if (!me) {
    return <div className="ds min-h-dvh bg-background" />;
  }
  if (!me.signed_in) {
    return <LoginPage />;
  }
  return (
    <Routes>
      <Route element={<AppShell />}>
        <Route index element={<HomePage />} />
        <Route path="c/:id/:tab?" element={<CollectionPage />} />
        <Route
          path="admin/:tab?"
          element={me.admin ? <AdminPage /> : <Navigate to="/" replace />}
        />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Route>
    </Routes>
  );
}

/** URLs: / (your collections), /c/<id>/<tab> (one collection), /admin/<tab>. */
const App = () => (
  <BrowserRouter>
    <SessionProvider>
      <Routed />
      <Toaster />
    </SessionProvider>
  </BrowserRouter>
);

export default App;
