import { expect, test } from '@playwright/test';

test('account page shows sign in', async ({ page }) => {
  await page.goto('/system/account');

  await expect(page.getByRole('heading', { name: 'Sign in' })).toBeVisible();
  await expect(page.getByLabel('Email')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Login' })).toBeVisible();
});

test('every page URL opens inside the frame', async ({ page }) => {
  const pages = [
    ['/run/sort', 'Run'],
    ['/run/records', 'Run'],
    ['/rules/profiles', 'Rules'],
    ['/rules/bins', 'Rules'],
    ['/machine', 'Machine'],
    ['/machine/board/wiring', 'Machine'],
    ['/machine/cameras/c3', 'Machine'],
    ['/machine/chute/test', 'Machine'],
    ['/rules/bins/lookup', 'Rules'],
    ['/run/records/runs', 'Run'],
    ['/machine/board/config', 'Machine'],
    ['/diagnose/bench', 'Diagnose'],
    ['/machine/channels', 'Machine'],
    ['/machine/chute', 'Machine'],
    ['/machine/cameras', 'Machine'],
    ['/diagnose/journeys', 'Diagnose'],
    ['/system/settings', 'System'],
    ['/system/settings/power', 'System'],
  ];
  for (const [url, group] of pages) {
    await page.goto(url);
    await expect(page.locator('main h1')).toHaveText(group);
    await expect(page.locator('header').getByRole('button', { name: 'Stop' })).toBeVisible();
  }
});

test('/ opens Home', async ({ page }) => {
  await page.goto('/');
  await expect(page.locator('main h1')).toHaveText('Home');
  await expect(page.getByRole('link', { name: /Sorting rules/ })).toBeVisible();
});
