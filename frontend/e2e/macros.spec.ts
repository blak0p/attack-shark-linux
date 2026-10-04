import { test, expect } from "@playwright/test";
import type { Macro } from "../src/desktop-contract";

declare global {
  interface Window { __macroTest: { calls: string[]; library(): Macro[] } }
}

for (const offline of [false, true]) {
  test(`local macro navigation/create/rename/delete ${offline ? "offline" : "across devices"}`, async ({ page }) => {
    await page.goto(`/e2e.html${offline ? "?offline" : ""}`);
    await page.getByRole("link", { name: "Macros", exact: true }).click();
    await expect(page.getByText("No macros in your library yet.")).toBeVisible();
    await page.getByRole("button", { name: "New macro", exact: true }).click();
    await page.getByLabel("Macro name", { exact: true }).fill("Clicks");
    await page.getByRole("button", { name: "Save to library" }).click();
    await expect(page.getByRole("button", { name: "Clicks · 0 events" })).toHaveAttribute("aria-pressed", "true");
    const id = await page.evaluate(() => window.__macroTest.library()[0].id);
    if (!offline) await page.getByRole("combobox", { name: "Mouse device" }).selectOption("beta");
    await page.getByRole("link", { name: "Device", exact: true }).click();
    await page.getByRole("link", { name: "Macros", exact: true }).click();
    await expect(page.getByLabel("Macro name", { exact: true })).toHaveValue("Clicks");
    await page.getByLabel("Macro name", { exact: true }).fill("Renamed local macro");
    await page.getByRole("button", { name: "Save to library" }).click();
    await expect(page.getByRole("button", { name: "Renamed local macro · 0 events" })).toHaveAttribute("aria-pressed", "true");
    expect(await page.evaluate(() => window.__macroTest.library()[0])).toEqual({ id, name: "Renamed local macro", events: [] });
    await page.getByRole("button", { name: "Delete macro", exact: true }).click();
    await expect(page.getByRole("dialog", { name: "Delete macro confirmation" })).toBeVisible();
    await page.getByRole("button", { name: "Cancel", exact: true }).click();
    expect(await page.evaluate(() => window.__macroTest.calls)).toEqual(["CreateMacro", "UpdateMacro"]);
    await page.getByRole("button", { name: "Delete macro", exact: true }).click();
    await page.getByRole("button", { name: "Confirm delete" }).click();
    await expect(page.getByText("No macros in your library yet.")).toBeVisible();
    expect(await page.evaluate(() => window.__macroTest.calls)).toEqual(["CreateMacro", "UpdateMacro", "DeleteMacro"]);
    expect(await page.evaluate(() => window.__routingTest.calls.filter((call) => call.operation !== "SelectDevice"))).toEqual([]);
  });
}

test("macro cards and details stack without horizontal overflow on narrow screens", async ({ page }) => {
  await page.setViewportSize({ width: 620, height: 900 });
  await page.goto("/e2e.html?offline");
  await page.getByRole("link", { name: "Macros", exact: true }).click();
  await page.getByRole("button", { name: "New macro", exact: true }).click();
  const library = await page.getByRole("article", { name: "Macro library", exact: true }).boundingBox();
  const detail = await page.getByRole("article", { name: "Macro details", exact: true }).boundingBox();
  expect(library).not.toBeNull(); expect(detail).not.toBeNull();
  expect(detail!.y).toBeGreaterThanOrEqual(library!.y + library!.height);
  expect(detail!.x).toBe(library!.x);
  expect(await page.locator(".macro-manager").evaluate((element) => element.scrollWidth <= element.clientWidth)).toBe(true);
});
