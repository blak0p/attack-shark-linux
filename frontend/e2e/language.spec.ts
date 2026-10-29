import { test, expect } from "@playwright/test";

test("language selector switches the interface, persists after a reload and stays reachable", async ({ page }) => {
  await page.goto("/e2e.html");
  const select = page.getByRole("combobox", { name: "Language" });
  await expect(select).toHaveValue("en");
  await expect(page.getByRole("heading", { name: "Performance", exact: true })).toBeVisible();

  await select.selectOption("es");
  await expect(page.getByRole("navigation", { name: "Secciones del espacio de trabajo" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Rendimiento", exact: true })).toBeVisible();
  await expect(page.locator("html")).toHaveAttribute("lang", "es");

  await page.reload();
  await expect(page.getByRole("combobox", { name: "Idioma" })).toHaveValue("es");
  await expect(page.getByRole("heading", { name: "Rendimiento", exact: true })).toBeVisible();

  for (const width of [1280, 640, 400]) {
    await page.setViewportSize({ width, height: 720 });
    const box = await page.getByRole("combobox", { name: "Idioma" }).boundingBox();
    expect(box, `selector visible at ${width}px`).not.toBeNull();
    expect(box!.x).toBeGreaterThanOrEqual(0);
    expect(box!.x + box!.width).toBeLessThanOrEqual(width);
    expect(box!.y).toBeGreaterThanOrEqual(0);
  }

  await page.getByRole("combobox", { name: "Idioma" }).selectOption("en");
  await expect(page.getByRole("combobox", { name: "Language" })).toHaveValue("en");
  await expect(page.getByRole("heading", { name: "Performance", exact: true })).toBeVisible();
});
