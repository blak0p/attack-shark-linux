import { test, expect, type Page } from "@playwright/test";

async function assignMacro(page: Page, button: number, name: string, repeat = 1) {
  await page.getByRole("button", { name: `Button ${button} action`, exact: true }).click();
  await page.getByRole("menuitem", { name: "Macro", exact: true }).click();
  await page.getByRole("menuitem", { name, exact: true }).click();
  await expect(page.getByLabel(`Button ${button} repetitions`, { exact: true })).toBeVisible();
  if (repeat !== 1) await page.getByLabel(`Button ${button} repetitions`, { exact: true }).fill(String(repeat));
}
import type { Macro } from "../src/desktop-contract";

declare global {
  interface Window { __macroTest: { calls: string[]; library(): Macro[] } }
}

for (const width of [1280, 620]) {
  test(`saved macro assignment guidance leads to button remapping at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 1000 });
    await page.goto("/e2e.html");
    await page.getByRole("link", { name: "Macros", exact: true }).click();
    await page.getByRole("button", { name: "New macro", exact: true }).click();
    await expect(page.getByText("To assign this macro to a mouse button, open Button remapping.")).toBeVisible();
    await expect(page.getByRole("button", { name: "Assign to button" })).not.toBeVisible();
    await page.getByLabel("Macro name", { exact: true }).fill("Saved shortcut");
    for (const name of ["Middle click", "Forward click"]) {
      await page.getByRole("button", { name, exact: true }).click();
      await page.getByRole("button", { name: "Add click" }).click();
    }
    await page.getByRole("button", { name: "Save to library" }).click();
    await expect(page.getByText("Saved to library. No device changes made.")).toBeVisible();
    const saved = await page.evaluate(() => window.__macroTest.library()[0]);
    await page.getByRole("link", { name: "Button remapping", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Button remapping", exact: true, level: 1 })).toBeVisible();
    expect(await page.evaluate(() => window.__macroTest.calls)).toEqual(["CreateMacro"]);
    await assignMacro(page, 6, saved.name, 2);
    await expect(page.getByLabel("Remap assignment summary")).toContainText("Saved shortcut × 2");
    const staged = await page.evaluate(() => window.__routingTest.remapSnapshot("alpha")?.MacroPending);
    expect(staged).toMatchObject({ ID: saved.id, Name: saved.name, Events: saved.events, Button: 6, Repeat: 2 });
    expect(await page.evaluate(() => window.__routingTest.calls.map((call) => call.operation))).toEqual(["StageMacroAssignment", "StageMacroAssignment"]);
    await page.getByRole("button", { name: "Apply remap" }).click();
    await expect(page.getByLabel("Button remapping status")).toContainText("transport confirmed");
    expect(await page.evaluate(() => window.__macroTest.calls)).toEqual(["CreateMacro"]);
  });
}

test("saved macro stages one device overlay, discards locally and reports confirmed or partial transport", async ({ page }) => {
  await page.goto("/e2e.html");
  await page.getByRole("link", { name: "Macros", exact: true }).click();
  await page.getByRole("button", { name: "New macro", exact: true }).click();
  await page.getByLabel("Macro name", { exact: true }).fill("Left pair");
  await page.getByRole("button", { name: "Back click", exact: true }).click();
  await page.getByRole("button", { name: "Add click" }).click();
  await page.getByRole("button", { name: "Forward click", exact: true }).click();
  await page.getByRole("button", { name: "Add click" }).click();
  await page.getByRole("button", { name: "Save to library" }).click();
  await expect(page.getByText("Saved to library. No device changes made.")).toBeVisible();
  await page.getByRole("link", { name: "Button remapping", exact: true }).click();
  await page.getByRole("button", { name: "Button 4 action" }).click();
  await page.getByRole("menuitem", { name: "Basic", exact: true }).click();
  await page.getByRole("menuitem", { name: "Fire", exact: true }).click();
  await assignMacro(page, 7, "Left pair", 255);
  await expect(page.getByLabel("Remap assignment summary")).toContainText("Button 7: Left pair × 255");
  await expect(page.getByLabel("Remap assignment summary")).toContainText("Button 4: Fire");
  expect(await page.evaluate(() => window.__routingTest.calls.filter((call) => call.operation === "ApplyRemap"))).toEqual([]);
  await page.getByRole("button", { name: "Discard remap" }).click();
  await expect(page.getByLabel("Remap assignment summary")).toContainText("Button 7: DPI-");
  await assignMacro(page, 7, "Left pair", 255);
  await page.getByRole("combobox", { name: "Mouse device" }).selectOption("beta");
  await expect(page.getByLabel("Remap assignment summary")).not.toContainText("Left pair");
  await page.getByRole("combobox", { name: "Mouse device" }).selectOption("alpha");
  await expect(page.getByLabel("Remap assignment summary")).toContainText("Left pair × 255");
  await page.getByRole("button", { name: "Apply remap" }).click();
  await expect(page.getByLabel("Button remapping status")).toContainText("transport confirmed");
  await expect(page.getByLabel("Button remapping status")).toContainText("Playback and device persistence unverified");
  const applied = await page.evaluate(() => window.__routingTest.remapSnapshot("alpha")?.MacroApplied);
  expect(applied?.Button).toBe(7);
  expect(applied?.Events).toEqual(["mouse_back", "mouse_forward"].flatMap((type) =>
    ["down", "up"].map((action) => ({ type, action, delay_ms: 0 }))));
  await assignMacro(page, 6, "Left pair", 255);
  await expect(page.getByLabel("Remap assignment summary")).toContainText("Button 7: Left pair × 255");
  await page.evaluate(() => window.__routingTest.failNextApply());
  await page.getByRole("button", { name: "Apply remap" }).click();
  await expect(page.getByLabel("Button remapping status")).toContainText("partial or unknown");
  await expect(page.getByLabel("Remap assignment summary")).toContainText("Button 6: Left pair × 255");
  await page.getByRole("button", { name: "Button 6 action" }).click();
  await page.getByRole("menuitem", { name: "Basic", exact: true }).click();
  await page.getByRole("menuitem", { name: "Fire", exact: true }).click();
  await expect(page.getByLabel("Remap assignment summary")).toContainText("Button 6: Fire");
  expect(await page.evaluate(() => window.__routingTest.remapSnapshot("alpha")?.MacroDrafts?.[6])).toBeUndefined();
  expect(await page.evaluate(() => window.__routingTest.remapSnapshot("alpha")?.MacroDrafts?.[7]?.Name)).toBe("Left pair");
  await page.getByRole("button", { name: "Button 7 action" }).click();
  await page.getByRole("menuitem", { name: "Basic", exact: true }).click();
  await page.getByRole("menuitem", { name: "Fire", exact: true }).click();
  await page.getByRole("button", { name: "Apply remap" }).click();
  await expect(page.getByLabel("Button remapping status")).toContainText("Button remapping applied");
  await expect(page.getByLabel("Button remapping status")).not.toContainText("macro transport confirmed");
  await expect(page.getByText(/Prior macro transport progress retained/)).toContainText("partial or unknown");
  const ordinarySnapshot = await page.evaluate(() => window.__routingTest.remapSnapshot("alpha"));
  expect(ordinarySnapshot?.MacroApplied).toBeNull();
  expect(ordinarySnapshot?.MacroProgress).toEqual({ Assignment: 2, Upload: 1 });
  await page.getByRole("button", { name: "Discard remap" }).click();
  await expect(page.getByLabel("Remap assignment summary")).not.toContainText("Left pair");
  const applies = await page.evaluate(() => window.__routingTest.calls.filter((call) => call.operation === "ApplyRemap"));
  expect(applies).toHaveLength(3);
  expect(applies.every((call) => call.destination.Serial === "alpha" && call.config?.Buttons.length === 7)).toBe(true);
});

for (const type of ["mouse_left", "mouse_right", "mouse_middle", "mouse_back", "mouse_forward"]) {
  test(`saved ${type} click stages independently and rejects a stale saved draft`, async ({ page }) => {
    await page.goto("/e2e.html");
    await page.getByRole("link", { name: "Macros", exact: true }).click();
    await page.getByRole("button", { name: "New macro", exact: true }).click();
    await page.getByLabel("Macro name", { exact: true }).fill(type);
    const label = type.replace("mouse_", "");
    await page.getByRole("button", { name: `${label[0].toUpperCase()}${label.slice(1)} click`, exact: true }).click();
    await page.getByRole("button", { name: "Add click" }).click();
    await page.getByRole("button", { name: "Save to library" }).click();
    await expect(page.getByText("Saved to library. No device changes made.")).toBeVisible();
    expect(await page.evaluate(() => window.__routingTest.calls.filter((call) => call.operation !== "SelectDevice"))).toEqual([]);
    await page.getByRole("link", { name: "Button remapping", exact: true }).click();
    await assignMacro(page, 6, type);
    const frozen = await page.evaluate(() => window.__routingTest.remapSnapshot("alpha")?.MacroPending);
    expect(frozen?.Button).toBe(6);
    expect(frozen?.Events.map((event) => event.type)).toEqual([type, type]);
    await page.getByRole("link", { name: "Macros", exact: true }).click();
    await page.getByLabel("Macro name", { exact: true }).fill("Changed saved name");
    await page.getByRole("button", { name: "Save to library" }).click();
    await expect(page.getByText("Saved to library. No device changes made.")).toBeVisible();
    expect(await page.evaluate(() => window.__routingTest.remapSnapshot("alpha")?.MacroPending)).toEqual(frozen);
    await page.getByRole("link", { name: "Button remapping", exact: true }).click();
    await page.getByRole("button", { name: "Apply remap" }).click();
    await expect(page.getByLabel("Button remapping status")).toContainText("not confirmed");
    expect(await page.evaluate(() => window.__routingTest.remapSnapshot("alpha")?.MacroApplied)).toBeNull();
    await assignMacro(page, 6, "Changed saved name");
    await page.getByRole("button", { name: "Apply remap" }).click();
    await expect(page.getByLabel("Button remapping status")).toContainText("transport confirmed");
    expect(await page.evaluate(() => window.__routingTest.remapSnapshot("alpha")?.MacroApplied?.Name)).toBe("Changed saved name");
  });
}

for (const width of [1280, 620]) {
  test(`open macro options remain readable and bounded at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 1000 });
    await page.goto("/e2e.html");
    await page.getByRole("link", { name: "Macros", exact: true }).click();
    await page.getByRole("button", { name: "New macro", exact: true }).click();
    await page.getByText("Macro options", { exact: true }).click();
    const options = page.locator(".macro-options");
    const importButton = page.getByRole("button", { name: "Import JSON", exact: true });
    await expect(importButton).toBeVisible();
    await page.screenshot({ path: testInfo.outputPath(`options-${width}.png`) });
    await page.getByRole("button", { name: "Save to library" }).scrollIntoViewIfNeeded();
    await page.screenshot({ path: testInfo.outputPath(`footer-${width}.png`) });
    const bounds = await importButton.boundingBox();
    const optionsBounds = await options.boundingBox();
    expect(bounds).not.toBeNull();
    expect(optionsBounds).not.toBeNull();
    expect(bounds!.x).toBeGreaterThanOrEqual(optionsBounds!.x);
    expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(optionsBounds!.x + optionsBounds!.width);
    expect(optionsBounds!.x + optionsBounds!.width).toBeLessThanOrEqual(width);
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);
    await expect(page.getByRole("button", { name: "Add click" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Save to library" })).toBeVisible();
    await expect(page.getByText("Advanced events and recording")).not.toBeVisible();
  });
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

for (const width of [1280, 620]) {
  test(`premium composer and secondary tools are keyboard accessible at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 1000 });
    await page.goto("/e2e.html?offline");
    await page.getByRole("link", { name: "Macros", exact: true }).click();
    await page.getByRole("button", { name: "New macro", exact: true }).click();
    await page.getByLabel("Macro name", { exact: true }).fill("Five-button sequence");
    await expect(page.getByRole("button", { name: "Export saved macro" })).not.toBeVisible();
    for (const name of ["Left", "Right", "Middle", "Back", "Forward"]) {
      const choice = page.getByRole("button", { name: `${name} click`, exact: true });
      await choice.focus();
      await page.keyboard.press("Enter");
      await expect(choice).toHaveAttribute("aria-pressed", "true");
      await page.getByRole("button", { name: "Add click" }).focus();
      await page.keyboard.press("Space");
    }
    await expect(page.getByRole("list", { name: "Ordered timeline" }).locator("li")).toHaveCount(5);
    expect(await page.evaluate(() => window.__macroTest.calls)).toEqual([]);
    await expect(page.getByText(/Local sequence only/)).toBeVisible();
    await page.getByRole("button", { name: "Save to library" }).click();
    await expect(page.getByText("Saved to library. No device changes made.")).toBeVisible();
    expect(await page.locator(".macro-manager").evaluate((element) => element.scrollWidth <= element.clientWidth)).toBe(true);
    // The desktop shell scrolls internally; reset it for an honest top-of-editor capture.
    await page.locator(".macro-manager").evaluate((element) => {
      for (let parent = element.parentElement; parent; parent = parent.parentElement) parent.scrollTop = 0;
    });
    await page.screenshot({ path: testInfo.outputPath(`premium-${width}.png`), fullPage: true });
    const options = page.getByText("Macro options", { exact: true });
    await options.focus();
    await page.keyboard.press("Enter");
    await expect(page.getByRole("button", { name: "Import JSON" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Export saved macro" })).toBeEnabled();
    await expect(page.getByText("Advanced events and recording")).not.toBeVisible();
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
