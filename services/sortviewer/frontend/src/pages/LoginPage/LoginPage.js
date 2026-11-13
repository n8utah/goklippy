import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';

import { useLoginPage } from './LoginPage.logic';

/** Sign in: viewers with the email and password they were given. */
const LoginPage = () => {
  const logic = useLoginPage();
  return (
    <div className="ds flex min-h-dvh items-center justify-center bg-background px-4 py-10 text-foreground">
      <div className="w-full max-w-sm">
        <div className="mb-6 flex flex-col items-center gap-2 text-center">
          <svg viewBox="0 0 32 32" className="size-12" aria-hidden>
            <rect x="2" y="10" width="28" height="18" rx="3" className="fill-primary" />
            <rect x="7" y="5" width="7" height="6" rx="1.5" className="fill-primary" />
            <rect x="18" y="5" width="7" height="6" rx="1.5" className="fill-primary" />
          </svg>
          <h1 className="font-display text-2xl font-semibold">Watch your LEGO get sorted</h1>
          <p className="text-sm text-muted-foreground">
            Sign in with the email and password you were sent.
          </p>
        </div>
        <form
          onSubmit={logic.submit}
          className="flex flex-col gap-4 rounded-xl border border-border bg-card p-5"
        >
          <div className="flex flex-col gap-2">
            <Label htmlFor="email">Email</Label>
            <Input
              id="email"
              type="email"
              autoComplete="email"
              required
              value={logic.email}
              onChange={(e) => logic.setEmail(e.target.value)}
            />
          </div>
          <div className="flex flex-col gap-2">
            <Label htmlFor="password">Password</Label>
            <Input
              id="password"
              type="password"
              autoComplete="current-password"
              required
              value={logic.password}
              onChange={(e) => logic.setPassword(e.target.value)}
            />
          </div>
          {logic.error && (
            <p role="alert" className="text-sm text-destructive">
              {logic.error}
            </p>
          )}
          {logic.offline && (
            <p role="alert" className="text-sm text-destructive">
              The site isn’t answering. Try again in a moment.
            </p>
          )}
          <Button type="submit" disabled={logic.busy}>
            {logic.busy ? 'Signing in…' : 'Sign in'}
          </Button>
        </form>
        {logic.adminSignIn && (
          <p className="mt-4 text-center text-sm">
            <a
              href="/auth/login"
              className="text-muted-foreground hover:text-foreground hover:underline"
            >
              Admin sign-in
            </a>
          </p>
        )}
      </div>
    </div>
  );
};

export default LoginPage;
