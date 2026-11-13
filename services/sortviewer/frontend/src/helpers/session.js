import { createContext, useCallback, useContext, useEffect, useState } from 'react';

import { getMe, logout } from './api';

const SessionContext = createContext(null);

/**
 * Who's signed in: { signed_in, admin, name, email, admin_sign_in }, or null while
 * loading. `refresh` reloads it after signing in; `signOut` signs out.
 */
export function SessionProvider({ children }) {
  const [me, setMe] = useState(null);
  const refresh = useCallback(
    () =>
      getMe().then(setMe, () => setMe({ signed_in: false, admin_sign_in: false, offline: true })),
    [],
  );
  useEffect(() => {
    refresh();
  }, [refresh]);

  const signOut = useCallback(async () => {
    if (me?.admin) {
      // Signs out of the Gangway portal too.
      window.location.href = '/auth/logout';
      return;
    }
    await logout().catch(() => {});
    await refresh();
  }, [me, refresh]);

  return (
    <SessionContext.Provider value={{ me, refresh, signOut }}>{children}</SessionContext.Provider>
  );
}

export const useSession = () => useContext(SessionContext);
