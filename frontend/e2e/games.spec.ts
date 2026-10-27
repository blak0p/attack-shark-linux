import { expect, test } from '@playwright/test';

test('games panel edits rules, keeps their order and builds the Steam command', async ({ page, context }) => {
  await context.grantPermissions(['clipboard-read', 'clipboard-write']);
  await page.setViewportSize({ width: 1024, height: 768 });
  await page.goto('/e2e.html');
  await page.getByRole('link', { name: 'Games' }).click();
  await expect(page.getByRole('heading', { name: 'Games', exact: true })).toBeVisible();
  await expect(page.getByRole('cell', { name: 'cs2', exact: true })).toBeVisible();

  await page.getByLabel('Executable name').fill('Hades.exe');
  await page.getByRole('button', { name: 'Add rule' }).click();
  await expect(page.getByRole('cell', { name: 'Hades.exe', exact: true })).toBeVisible();
  await page.getByRole('button', { name: /Move Hades.exe up/ }).click();
  await expect(page.locator('tbody tr').first()).toContainText('Hades.exe');

  await page.getByLabel('Executable name').fill('Hades.exe');
  await page.getByRole('button', { name: 'Add rule' }).click();
  await expect(page.getByRole('alert')).toContainText('already exists');
  await page.getByLabel('Executable name').fill('C:\\games\\x.exe');
  await page.getByRole('button', { name: 'Add rule' }).click();
  await expect(page.getByRole('alert')).toContainText('name only');

  await expect(page.getByLabel('Launch command', { exact: true })).toContainText('attack-shark-linux --profile "Automatic profile" -- %command%');
  await page.getByRole('button', { name: 'Copy' }).click();
  expect(await page.evaluate(() => navigator.clipboard.readText())).toBe('attack-shark-linux --profile "Automatic profile" -- %command%');

  await page.getByLabel('Switch profile automatically').uncheck();
  await expect(page.getByLabel('Executable name')).toBeDisabled();
  await page.screenshot({ path: '../bin/games-panel.png' });
});
