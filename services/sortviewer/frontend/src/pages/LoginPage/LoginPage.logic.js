import { useState } from 'react';

import { login } from '@/helpers/api';
import { useSession } from '@/helpers/session';

// What /?signin=... means, after coming back from the Gangway.
const SIGNIN_MESSAGES = {
  notadmin: 'That account isn’t an admin of this site.',
  failed: 'Admin sign-in didn’t work. Try again.',
  cancelled: 'Admin sign-in was cancelled.',
  unavailable: 'Admin sign-in isn’t reachable right now.',
};

/** Signing in with email and password; admins go through the Gangway instead. */
export function useLoginPage() {
  const { me, refresh } = useSession();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(
    () => SIGNIN_MESSAGES[new URLSearchParams(window.location.search).get('signin')] ?? '',
  );

  const submit = async (e) => {
    e.preventDefault();
    setBusy(true);
    setError('');
    try {
      await login(email, password);
      window.history.replaceState(null, '', window.location.pathname);
      await refresh();
    } catch (err) {
      setError(err.message);
      setBusy(false);
    }
  };

  return {
    email,
    setEmail,
    password,
    setPassword,
    busy,
    error,
    submit,
    adminSignIn: Boolean(me?.admin_sign_in),
    offline: Boolean(me?.offline),
  };
}
