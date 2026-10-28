import { expect, test } from '@playwright/test';

test('games panel edits rules, keeps their order and builds the Steam command', async ({ page, context }) => {
  await context.grantPermissions(['clipboard-read', 'clipboard-write']);
  await page.setViewportSize({ width: 1024, height: 768 });
  await page.goto('/e2e.html');
  await page.getByRole('link', { name: 'Games' }).click();
  await expect(page.getByRole('heading', { name: 'Games', exact: true })).toBeVisible();
  await expect(page.getByRole('cell', { name: 'cs2', exact: true })).toBeVisible();

  await page.getByRole('button', { name: 'Add a game' }).click();
  const dialog = page.getByRole('dialog', { name: 'Add a game' });
  await dialog.getByText('Enter an executable name').click();
  await dialog.getByLabel('Executable name').fill('Hades.exe');
  await dialog.getByRole('button', { name: 'Add', exact: true }).click();
  await expect(page.getByRole('cell', { name: 'Hades.exe', exact: true })).toBeVisible();
  await page.getByRole('button', { name: /Move Hades.exe up/ }).click();
  await expect(page.locator('tbody tr').first()).toContainText('Hades.exe');

  await page.getByRole('button', { name: 'Add a game' }).click();
  await dialog.getByText('Enter an executable name').click();
  await dialog.getByLabel('Executable name').fill('Hades.exe');
  await dialog.getByRole('button', { name: 'Add', exact: true }).click();
  await expect(dialog.getByRole('alert')).toContainText('already exists');
  await dialog.getByLabel('Executable name').fill('C:\\games\\x.exe');
  await dialog.getByRole('button', { name: 'Add', exact: true }).click();
  await expect(dialog.getByRole('alert')).toContainText('name only');
  await dialog.getByRole('button', { name: 'Cancel' }).click();

  await expect(page.getByLabel('Launch command', { exact: true })).toContainText('attack-shark-linux --profile "Automatic profile" -- %command%');
  await page.getByRole('button', { name: 'Copy' }).click();
  expect(await page.evaluate(() => navigator.clipboard.readText())).toBe('attack-shark-linux --profile "Automatic profile" -- %command%');

  await expect(page.getByText('Automatic switching is off.')).toHaveCount(0);
  await page.getByLabel('Switch profile automatically').uncheck();
  await expect(page.getByText('Automatic switching is off. Your rules are saved and will apply when you turn it on.')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Add a game' })).toBeEnabled();
  await page.getByRole('button', { name: /Move Hades.exe down/ }).click();
  await expect(page.locator('tbody tr').nth(1)).toContainText('Hades.exe');
  await page.screenshot({ path: '../bin/games-panel.png' });
});

test('game picker adds an installed game by display name', async ({ page }) => {
  await page.setViewportSize({ width: 1024, height: 768 });
  await page.goto('/e2e.html');
  await page.getByRole('link', { name: 'Games' }).click();
  await page.getByRole('button', { name: 'Add a game' }).click();
  const dialog = page.getByRole('dialog', { name: 'Add a game' });
  await expect(dialog.getByRole('button', { name: /Firefox/ })).toBeHidden();
  await page.screenshot({ path: '../bin/games-dialog.png' });
  await dialog.getByLabel('Search games').fill('terra');
  await expect(dialog.getByRole('button', { name: /Rocket League/ })).toHaveCount(0);
  await dialog.getByRole('button', { name: /Terraria/ }).click();
  await dialog.getByRole('button', { name: 'Add', exact: true }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  const row = page.locator('tbody tr').filter({ hasText: 'Terraria' });
  await expect(row.locator('.games-name')).toHaveText('Terraria');
  await expect(row.locator('.games-sub')).toHaveText('/home/user/.steam/steamapps/common/Terraria');
  await page.screenshot({ path: '../bin/games-panel-folder.png' });
});
