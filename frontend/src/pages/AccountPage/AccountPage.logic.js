import { useCallback, useState } from 'react';

import { getMe, login, logout, refresh } from '../../helpers/api';

export const useAccountPageLogic = () => {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [tokenPair, setTokenPair] = useState(null);
  const [profile, setProfile] = useState(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const handleLogin = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const result = await login({ email, password });
      setTokenPair(result);
      const me = await getMe(result.access_token);
      setProfile(me);
    } catch (requestError) {
      setError(requestError.message);
    } finally {
      setLoading(false);
    }
  }, [email, password]);

  const handleRefresh = useCallback(async () => {
    if (!tokenPair?.refresh_token) {
      setError('No refresh token available.');
      return;
    }

    setLoading(true);
    setError('');
    try {
      const result = await refresh(tokenPair.refresh_token);
      setTokenPair(result);
      const me = await getMe(result.access_token);
      setProfile(me);
    } catch (requestError) {
      setError(requestError.message);
    } finally {
      setLoading(false);
    }
  }, [tokenPair]);

  const handleLogout = useCallback(async () => {
    if (!tokenPair?.refresh_token) {
      setTokenPair(null);
      return;
    }

    setLoading(true);
    setError('');
    try {
      await logout(tokenPair.refresh_token);
      setTokenPair(null);
      setProfile(null);
    } catch (requestError) {
      setError(requestError.message);
    } finally {
      setLoading(false);
    }
  }, [tokenPair]);

  const handleLoadProfile = useCallback(async () => {
    if (!tokenPair?.access_token) {
      setError('No access token available.');
      return;
    }

    setLoading(true);
    setError('');
    try {
      const me = await getMe(tokenPair.access_token);
      setProfile(me);
    } catch (requestError) {
      setError(requestError.message);
    } finally {
      setLoading(false);
    }
  }, [tokenPair]);

  return {
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
  };
};
