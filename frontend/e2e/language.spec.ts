import { test, expect, type Page } from "@playwright/test";

const choose = async (page: Page, trigger: string, option: string) => {
  await page.getByRole("button", { name: trigger, exact: true }).click();
  await page.getByRole("menuitem", { name: option, exact: true }).click();
};

test("language selector switches the interface, persists after a reload and never overlaps the top bar", async ({ page }) => {
  await page.goto("/e2e.html");
  await expect(page.getByRole("button", { name: "Language", exact: true })).toContainText("English");
  await expect(page.getByRole("heading", { name: "Performance", exact: true })).toBeVisible();

  await choose(page, "Language", "Español");
  await expect(page.getByRole("navigation", { name: "Secciones del espacio de trabajo" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Rendimiento", exact: true })).toBeVisible();
  await expect(page.locator("html")).toHaveAttribute("lang", "es");

  await page.reload();
  await expect(page.getByRole("button", { name: "Idioma", exact: true })).toContainText("Español");

  for (const width of [1280, 900, 850, 640, 541, 540, 400]) {
    await page.setViewportSize({ width, height: 720 });
    const language = await page.getByRole("button", { name: "Idioma", exact: true }).boundingBox();
    expect(language, `selector visible at ${width}px`).not.toBeNull();
    expect(language!.x).toBeGreaterThanOrEqual(0);
    expect(language!.x + language!.width).toBeLessThanOrEqual(width);
    const others = [
      // Class selectors: accessible names change with the language under test.
      page.locator(".profile-controls"),
      page.locator(".titlebar-device select"),
      page.locator(".window-controls"),
      page.locator(".brand-title"),
    ];
    for (const other of others) {
      const box = await other.boundingBox();
      if (!box) continue;
      const apart = language!.x + language!.width <= box.x || box.x + box.width <= language!.x
        || language!.y + language!.height <= box.y || box.y + box.height <= language!.y;
      expect(apart, `language selector overlaps a top bar control at ${width}px`).toBe(true);
    }
  }

  await choose(page, "Idioma", "English");
  await expect(page.getByRole("button", { name: "Language", exact: true })).toContainText("English");
  await expect(page.getByRole("heading", { name: "Performance", exact: true })).toBeVisible();
});
