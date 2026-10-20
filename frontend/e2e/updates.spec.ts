import { test, expect } from "@playwright/test";

for (const width of [1280, 620]) {
  test(`combined updates are explicit and bounded at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 });
    await page.emulateMedia({ reducedMotion: "reduce" });
    await page.goto("/e2e.html?updates=partial");
    const notice = page.locator(".update-banner");
    await expect(notice).toHaveCount(1);
    await expect(notice).toContainText("AppImage and USB permissions");
    const calls = () => page.evaluate(() => (window as unknown as { __updatesTest: { calls: unknown[] } }).__updatesTest.calls);
    expect(await calls()).toEqual([{ operation: "check", args: [] }]);
    await page.getByRole("button", { name: "Review updates" }).click();
    await expect(page.getByRole("region", { name: "Device", exact: true })).toHaveAttribute("data-active", "true");
    const panel = page.locator(".unified-updates");
    await expect(panel).toBeVisible();
    expect(await calls()).toHaveLength(1);
    await panel.getByRole("button", { name: "Apply updates" }).click();
    await expect(panel.getByText("USB policy written; reload or activation failed.")).toBeVisible();
    await expect(panel.getByText("AppImage replaced; relaunch failed.")).toBeVisible();
    await expect(panel.getByText("USB permissions: reload failed")).toBeVisible();
    await expect(panel.getByText("Relaunch: launch failed")).toBeVisible();
    expect(await calls()).toEqual([{ operation: "check", args: [] }, { operation: "apply", args: [] }]);
    await panel.getByRole("button", { name: "Relaunch app" }).click();
    await expect(panel.getByRole("button", { name: "Relaunch app" })).toBeEnabled();
    expect(await calls()).toEqual([{ operation: "check", args: [] }, { operation: "apply", args: [] }, { operation: "apply", args: [] }]);
    for (const element of [notice, panel]) {
      const bounds = await element.boundingBox();
      expect(bounds).not.toBeNull();
      expect(bounds!.x).toBeGreaterThanOrEqual(0);
      expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(width);
    }
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  });
}

test("denial remains visible until an explicit combined retry", async ({ page }) => {
  await page.goto("/e2e.html?updates=denial");
  await page.getByRole("button", { name: "Review updates" }).click();
  const panel = page.locator(".unified-updates");
  await panel.getByRole("button", { name: "Apply updates" }).click();
  await expect(panel.getByText("USB permissions: authorization denied")).toBeVisible();
  await expect(panel.getByText("AppImage: download failed")).toBeVisible();
  expect(await page.evaluate(() => (window as any).__updatesTest.calls)).toEqual([{ operation: "check", args: [] }, { operation: "apply", args: [] }]);
  await panel.getByRole("button", { name: "Apply updates" }).click();
  await expect(panel.getByText("USB policy written and reload completed; reconnect the mouse if needed.")).toBeVisible();
  await expect(panel.getByText("AppImage replaced; restart requested, not confirmed.")).toBeVisible();
  expect(await page.evaluate(() => (window as any).__updatesTest.calls)).toEqual([{ operation: "check", args: [] }, { operation: "apply", args: [] }, { operation: "apply", args: [] }]);
});
