import { expect, test } from '@playwright/test';

test('local profile CRUD, truthful switching, notice and mouse isolation', async ({ page }) => {
  await page.goto('/e2e.html');
  await expect(page.getByLabel('Local profile', { exact: true })).toHaveValue('automatic');
  await expect(page.getByText(/Automatic profile created at/)).toContainText('No mouse changes');
  await page.getByRole('button', { name: 'Dismiss profile notice' }).click();
  await page.getByRole('button', { name: 'Manage profiles' }).click();
  await expect(page.getByRole('button', { name: 'Delete Automatic profile' })).toBeDisabled();
  // Real pointer clicks guard against titlebar overflow clipping the profile overlays.
  await page.getByRole('button', { name: 'Create profile', exact: true }).click();
  await expect(page.getByRole('dialog', { name: 'Create profile', exact: true })).toBeVisible();
  await page.getByLabel('Profile name').fill('Work');
  await page.getByRole('button', { name: 'Confirm profile action' }).click();
  await expect(page.getByLabel('Local profile', { exact: true })).toHaveValue('profile-1');
  await page.getByRole('button', { name: 'Rename Work', exact: true }).click();
  await page.getByLabel('Profile name').fill('Office');
  await page.getByRole('button', { name: 'Confirm profile action' }).click();
  await page.getByRole('button', { name: 'Duplicate Office', exact: true }).click();
  await page.getByLabel('Profile name').fill('Gaming');
  await page.getByRole('button', { name: 'Confirm profile action' }).click();
  await expect(page.getByLabel('Local profile', { exact: true })).toHaveValue('profile-1');
  await page.getByRole('button', { name: 'Manage profiles' }).click();
  await page.evaluate(() => (window as unknown as { __routingTest: { failNextApply(): void } }).__routingTest.failNextApply());
  await page.getByLabel('Local profile', { exact: true }).selectOption('profile-2');
  await expect(page.getByRole('alert')).toContainText('partial_failure');
  await expect(page.getByLabel('Local profile', { exact: true })).toHaveValue('profile-1');
  await page.getByLabel('Local profile', { exact: true }).selectOption('profile-2');
  await expect(page.getByText('Profile applied.', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Manage profiles' }).click();
  await page.getByRole('button', { name: 'Delete Office', exact: true }).click();
  await page.getByRole('button', { name: 'Confirm profile action' }).click();
  await expect(page.getByRole('option', { name: 'Office', exact: true })).toHaveCount(0);
  await page.getByRole('button', { name: 'Manage profiles' }).click();
  await page.getByLabel('Mouse device').selectOption('beta');
  await expect(page.getByLabel('Local profile', { exact: true })).toHaveValue('automatic');
  await expect(page.getByRole('option', { name: 'Gaming', exact: true })).toHaveCount(0);
});
