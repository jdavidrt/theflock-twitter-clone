import { test, expect } from '@playwright/test';

// D-36: the one required E2E spec — a real browser against an already-running
// `npm run dev` stack (this file does not start the app; see the README Runbook).
// Registers a unique user per run so the spec is rerunnable without resetting the database.
test('register, use the app, log out, and log back in', async ({ page }) => {
  const username = `e2e_${Date.now()}`;
  const email = `${username}@example.com`;
  const password = 'E2ePassword123!';

  await page.goto('/');
  await expect(page).toHaveURL(/\/login$/);

  await page.getByRole('link', { name: /sign up/i }).click();
  await expect(page.getByRole('heading', { name: /sign up/i })).toBeVisible();

  await page.getByLabel('Email').fill(email);
  await page.getByLabel('Username').fill(username);
  await page.getByLabel('Password (8-72 characters)').fill(password);
  await page.getByLabel('Confirm password').fill(password);
  await page.getByRole('button', { name: /sign up/i }).click();

  await expect(page).toHaveURL(/\/$/);
  await expect(page.getByRole('link', { name: username, exact: true }).first()).toBeVisible();

  await page.getByRole('button', { name: 'Log out' }).first().click();
  await expect(page).toHaveURL(/\/login$/);

  await page.getByLabel('Email').fill(email);
  await page.getByLabel('Password').fill(password);
  await page.getByRole('button', { name: /log in/i }).click();

  await expect(page).toHaveURL(/\/$/);
  await expect(page.getByRole('link', { name: username, exact: true }).first()).toBeVisible();
});
