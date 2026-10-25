import { expect, test } from '@playwright/test';

test('profile notices share the bottom-right host and expire without stored dismissal', async ({ page }) => {
  await page.clock.install();
  await page.goto('/e2e.html');
  const notice = page.getByText(/Automatic profile created at/);
  await expect(notice).toBeVisible();
  const host = page.getByRole('region', { name: 'Operation feedback', exact: true });
  await expect(host).toHaveCount(1);
  await expect(host).toContainText('No mouse changes');
  const bounds = await host.boundingBox();
  const viewport = page.viewportSize()!;
  expect(Math.abs(bounds!.x + bounds!.width - viewport.width + 20)).toBeLessThanOrEqual(1);
  expect(Math.abs(bounds!.y + bounds!.height - viewport.height + 20)).toBeLessThanOrEqual(1);
  await page.screenshot({ path: '../bin/profiles-titlebar-notification.png' });
  await page.clock.fastForward(8000);
  await expect(notice).toHaveCount(0);
  await page.getByLabel('Mouse device').selectOption('beta');
  await expect(notice).toBeVisible();
  await page.getByLabel('Mouse device').selectOption('alpha');
  await expect(notice).toBeVisible();
});

test('local profile CRUD, truthful switching, notice and mouse isolation', async ({ page }) => {
  await page.goto('/e2e.html');
  const selector = page.getByRole('button', { name: 'Profile', exact: true });
  await expect(selector).toContainText('Automatic profile');
  await expect(page.getByText(/Automatic profile created at/)).toContainText('No mouse changes');
  await page.getByRole('button', { name: 'Dismiss profile notice' }).click();
  for (const width of [1280, 640, 400]) {
    await page.setViewportSize({ width, height: 720 });
    const block = await page.getByRole('region', { name: 'Local profiles' }).boundingBox();
    expect(Math.abs(block!.x + block!.width / 2 - width / 2)).toBeLessThanOrEqual(1);
    const header = await page.locator('.top-bar').boundingBox();
    const controls = await page.getByRole('button', { name: 'Close window' }).boundingBox();
    const device = await page.getByLabel('Mouse device').boundingBox();
    for (const rect of [block!, controls!, device!]) {
      expect(rect.y).toBeGreaterThanOrEqual(header!.y);
      expect(rect.y + rect.height).toBeLessThanOrEqual(header!.y + header!.height);
      expect(rect.x).toBeGreaterThanOrEqual(0);
      expect(rect.x + rect.width).toBeLessThanOrEqual(width);
    }
    expect(block!.x + block!.width <= device!.x || block!.y >= device!.y + device!.height).toBe(true);
    await selector.click();
    await expect(page.getByRole('menuitem', { name: 'Automatic profile', exact: true })).toBeVisible();
    await page.screenshot({ path: `../bin/profiles-titlebar-${width}.png` });
    await selector.press('Escape');
  }
  await page.setViewportSize({ width: 1280, height: 720 });
  await selector.click();
  const menu = page.getByRole('menu', { name: 'Profile categories', exact: true });
  await expect(menu).toBeVisible();
  const style = await menu.evaluate(element => ({ background: getComputedStyle(element).backgroundColor, opacity: getComputedStyle(element).opacity }));
  expect(style.background).toMatch(/^rgb\(/);
  expect(style.opacity).toBe('1');
  await page.screenshot({ path: '../bin/profiles-selector-menu.png' });
  await selector.press('Escape');
  await page.getByRole('button', { name: 'Manage profiles' }).click();
  await expect(page.getByRole('button', { name: 'Delete Automatic profile' })).toBeDisabled();
  const panel = page.getByRole('region', { name: 'Profile management' });
  await expect(panel).toBeVisible();
  await expect(panel.getByText('Active', { exact: true })).toBeVisible();
  for (const [width, height, suffix] of [[1280, 720, 'desktop'], [640, 720, 'compact']] as const) {
    await page.setViewportSize({ width, height });
    const bounds = await panel.boundingBox();
    expect(bounds).not.toBeNull();
    expect(bounds!.x).toBeGreaterThanOrEqual(0);
    expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(width);
    expect(Math.abs(bounds!.x + bounds!.width / 2 - width / 2)).toBeLessThanOrEqual(1);
    expect(Math.abs(bounds!.y + bounds!.height / 2 - (64 + (height - 64) / 2))).toBeLessThanOrEqual(1);
    await page.screenshot({ path: `../bin/profiles-selector-${suffix}.png` });
  }
  await page.setViewportSize({ width: 1280, height: 720 });
  await page.getByRole('button', { name: 'Create profile', exact: true }).click();
  await expect(page.getByRole('dialog', { name: 'Create profile', exact: true })).toBeVisible();
  await page.getByLabel('Profile name').fill('Work');
  await page.getByRole('button', { name: 'Confirm profile action' }).click();
  await expect(selector).toContainText('Work');
  await page.getByRole('button', { name: 'Rename Work', exact: true }).click();
  await page.getByLabel('Profile name').fill('Office');
  await page.getByRole('button', { name: 'Confirm profile action' }).click();
  await page.getByRole('button', { name: 'Duplicate Office', exact: true }).click();
  await page.getByLabel('Profile name').fill('Gaming');
  await page.getByRole('button', { name: 'Confirm profile action' }).click();
  await expect(selector).toContainText('Office');
  await page.getByRole('button', { name: 'Close profile management' }).click();
  await page.evaluate(() => (window as unknown as { __routingTest: { failNextApply(): void } }).__routingTest.failNextApply());
  await selector.click();
  await page.getByRole('menuitem', { name: 'Gaming', exact: true }).click();
  await expect(page.getByRole('alert')).toContainText('partial_failure');
  await expect(selector).toContainText('Office');
  await selector.click();
  await page.getByRole('menuitem', { name: 'Gaming', exact: true }).click();
  await expect(page.getByText('Profile applied.', { exact: true })).toBeVisible();
  await expect(selector).toContainText('Gaming');
  await page.getByRole('button', { name: 'Manage profiles' }).click();
  await page.getByRole('button', { name: 'Delete Office', exact: true }).click();
  await page.getByRole('button', { name: 'Confirm profile action' }).click();
  await expect(page.getByRole('button', { name: 'Rename Office', exact: true })).toHaveCount(0);
  await page.getByRole('button', { name: 'Close profile management' }).click();
  await selector.click();
  await expect(page.getByRole('menuitem', { name: 'Office', exact: true })).toHaveCount(0);
  await selector.press('Escape');
  await page.getByLabel('Mouse device').selectOption('beta');
  await expect(selector).toContainText('Automatic profile');
  await selector.click();
  await expect(page.getByRole('menuitem', { name: 'Gaming', exact: true })).toHaveCount(0);
});
