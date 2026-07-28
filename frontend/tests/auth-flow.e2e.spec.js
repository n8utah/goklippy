import { expect, test } from '@playwright/test';

test('auth flow: login, refresh, and logout', async ({ page }) => {
  await page.goto('/');

  await page.getByLabel('Email').fill('admin@admin.com');
  await page.getByLabel('Password').fill('admin');

  await page.getByRole('button', { name: 'Login' }).click();
  await expect(page.getByText('Authenticated User')).toBeVisible();
  await expect(page.getByText('Email: admin@admin.com')).toBeVisible();
  await expect(page.getByText('Role: admin')).toBeVisible();

  await page.getByRole('button', { name: 'Refresh' }).click();
  await expect(page.getByText('Authenticated User')).toBeVisible();

  await page.getByRole('button', { name: 'Logout' }).click();
  await expect(page.getByText('Authenticated User')).toHaveCount(0);
});
