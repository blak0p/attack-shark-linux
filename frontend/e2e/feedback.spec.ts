import { test, expect } from "@playwright/test";

for (const width of [1280, 620]) {
  test(`shared feedback preserves the workspace at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 900 });
    await page.goto("/e2e.html?feedback");
    await page.getByRole("radio", { name: "500 Hz" }).click();
    const status = page.getByRole("status", { name: "Operation status" });
    await expect(status).toHaveText("Working…");
    await expect(page.locator(".pending-spinner")).toHaveCount(1);
    await expect(page.locator(".pending-spinner")).toHaveCSS("animation-name", "pending-spin");
    await expect(status).toHaveText("✓Applied");
    await expect(page.locator(".operation-feedback")).toHaveCSS("position", "fixed");
    const box = await status.boundingBox();
    expect(box).not.toBeNull();
    expect(box!.x).toBeGreaterThanOrEqual(0);
    expect(box!.x + box!.width).toBeLessThanOrEqual(width);
    await page.screenshot({ path: testInfo.outputPath(`feedback-${width}.png`) });
    await expect(status).toHaveCount(0, { timeout: 5000 });
    await page.getByRole("slider", { name: "Stage 4 DPI" }).fill("3500");
    await expect(status).toHaveText("Working…");
    await expect(status).toHaveText("✓Applied");
    await expect(page.getByText("Firmware applied", { exact: true })).toHaveCount(0);
    await expect(page.getByText("Persistence saved", { exact: true })).toHaveCount(0);
  });
}

test("reduced motion keeps progress visible without spinning", async ({ page }) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto("/e2e.html?feedback");
  await page.getByRole("radio", { name: "500 Hz" }).click();
  await expect(page.getByRole("status", { name: "Operation status" })).toHaveText("Working…");
  await expect(page.locator(".pending-spinner")).toHaveCSS("animation-name", "none");
  await expect(page.getByRole("status", { name: "Operation status" })).toHaveText("✓Applied");
});
