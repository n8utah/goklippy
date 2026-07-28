import { expect, test } from '@playwright/test';

test('home page loads framework starter', async ({ page }) => {
  await page.goto('/');

  await expect(page.getByRole('heading', { name: 'goklippy Framework Starter' })).toBeVisible();
  await expect(page.getByLabel('Email')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Login' })).toBeVisible();
});
