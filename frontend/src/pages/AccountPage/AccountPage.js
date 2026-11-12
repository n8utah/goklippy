import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';

import { useAccountPageLogic } from './AccountPage.logic';

/**
 * System › Account: sign in, refresh and sign out (the JWT flow), and for development the
 * tokens themselves.
 */
const AccountPage = () => {
  const logic = useAccountPageLogic();
  const { tokenPair, profile } = logic;

  return (
    <div className="ds grid items-start gap-4 lg:grid-cols-[minmax(0,420px)_minmax(0,1fr)]">
      <section className="flex flex-col gap-4 rounded-xl border border-border bg-card p-5">
        <h2 className="font-display text-xl font-semibold">Sign in</h2>
        <form
          className="flex flex-col gap-3"
          onSubmit={(e) => {
            e.preventDefault();
            logic.handleLogin();
          }}
        >
          <label htmlFor="email-input" className="flex flex-col gap-1.5 text-sm font-medium">
            Email
            <Input
              id="email-input"
              type="email"
              autoComplete="username"
              value={logic.email}
              onChange={(e) => logic.setEmail(e.target.value)}
              placeholder="user@example.com"
            />
          </label>
          <label htmlFor="password-input" className="flex flex-col gap-1.5 text-sm font-medium">
            Password
            <Input
              id="password-input"
              type="password"
              autoComplete="current-password"
              value={logic.password}
              onChange={(e) => logic.setPassword(e.target.value)}
            />
          </label>
          <div className="flex flex-wrap gap-2 pt-1">
            <Button type="submit" disabled={logic.loading}>
              Login
            </Button>
            <Button
              type="button"
              variant="outline"
              disabled={logic.loading}
              onClick={logic.handleRefresh}
            >
              Refresh
            </Button>
            <Button
              type="button"
              variant="outline"
              disabled={logic.loading}
              onClick={logic.handleLoadProfile}
            >
              Load profile
            </Button>
            <Button
              type="button"
              variant="ghost"
              disabled={logic.loading}
              onClick={logic.handleLogout}
            >
              Logout
            </Button>
          </div>
        </form>
        {logic.error && (
          <p role="alert" className="text-sm text-destructive">
            {logic.error}
          </p>
        )}
        {profile && (
          <div className="rounded-lg border border-running/40 bg-running-bg px-4 py-3 text-sm text-running-text">
            <p className="font-medium">Authenticated User</p>
            <p>User ID: {profile.user_id}</p>
            <p>Email: {profile.email}</p>
            <p>Role: {profile.role}</p>
          </div>
        )}
      </section>

      {tokenPair && (
        <section className="flex min-w-0 flex-col gap-3 rounded-xl border border-border bg-card p-5">
          <h2 className="font-display text-lg font-semibold">Tokens</h2>
          <p className="text-sm text-muted-foreground">For development: what the backend issued.</p>
          {[
            ['Access token', tokenPair.access_token],
            ['Refresh token', tokenPair.refresh_token],
          ].map(([label, value]) => (
            <div key={label} className="flex flex-col gap-1">
              <p className="text-sm font-medium">{label}</p>
              <code className="block rounded-md bg-muted p-3 font-mono text-xs break-all">
                {value}
              </code>
            </div>
          ))}
        </section>
      )}
    </div>
  );
};

export default AccountPage;
