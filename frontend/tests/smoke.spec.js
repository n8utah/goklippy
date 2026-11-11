import { expect, test } from '@playwright/test';

test('account page loads framework starter', async ({ page }) => {
  await page.goto('/system/account');

  await expect(page.getByRole('heading', { name: 'goklippy Framework Starter' })).toBeVisible();
  await expect(page.getByLabel('Email')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Login' })).toBeVisible();
});

test('every page URL opens inside the frame', async ({ page }) => {
  const pages = [
    ['/run/sort', 'Run'],
    ['/run/records', 'Run'],
    ['/rules/profiles', 'Rules'],
    ['/rules/bins', 'Rules'],
    ['/machine/board/wiring', 'Machine'],
    ['/machine/channels', 'Machine'],
    ['/machine/chute', 'Machine'],
    ['/machine/cameras', 'Machine'],
    ['/diagnose/journeys', 'Diagnose'],
    ['/system/settings', 'System'],
  ];
  for (const [url, group] of pages) {
    await page.goto(url);
    await expect(page.locator('main h1')).toHaveText(group);
    await expect(page.locator('header').getByRole('button', { name: 'Stop' })).toBeVisible();
  }
});

test('/ opens Run › Sort', async ({ page }) => {
  await page.goto('/');
  await expect(page).toHaveURL(/\/run\/sort$/);
});
