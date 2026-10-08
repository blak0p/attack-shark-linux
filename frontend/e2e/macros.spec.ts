import { test, expect } from "@playwright/test";
import type { Macro } from "../src/desktop-contract";

declare global {
  interface Window { __macroTest: { calls: string[]; library(): Macro[] } }
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
  await page.getByLabel("Saved macro", { exact: true }).selectOption({ label: "Left pair" });
  await page.getByLabel("Macro target button").selectOption("7");
  await page.getByLabel("Fixed repetitions").fill("255");
  await page.getByRole("button", { name: "Stage macro assignment" }).click();
  await expect(page.getByLabel("Remap assignment summary")).toContainText("Button 7: Left pair × 255");
  await expect(page.getByLabel("Remap assignment summary")).toContainText("Button 4: Fire");
  expect(await page.evaluate(() => window.__routingTest.calls.filter((call) => call.operation === "ApplyRemap"))).toEqual([]);
  await page.getByRole("button", { name: "Discard remap" }).click();
  await expect(page.getByLabel("Remap assignment summary")).toContainText("Button 7: DPI-");
  await page.getByRole("button", { name: "Stage macro assignment" }).click();
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
  await page.getByLabel("Saved macro", { exact: true }).selectOption({ label: "Left pair" });
  await page.getByLabel("Fixed repetitions").fill("255");
  await page.getByLabel("Macro target button").selectOption("6");
  await page.getByRole("button", { name: "Stage macro assignment" }).click();
  await expect(page.getByLabel("Remap assignment summary")).toContainText("Button 7: DPI-");
  await page.evaluate(() => window.__routingTest.failNextApply());
  await page.getByRole("button", { name: "Apply remap" }).click();
  await expect(page.getByLabel("Button remapping status")).toContainText("partial or unknown");
  await expect(page.getByLabel("Remap assignment summary")).toContainText("Button 6: Left pair × 255");
  await page.getByRole("button", { name: "Button 6 action" }).click();
  await page.getByRole("menuitem", { name: "Basic", exact: true }).click();
  await page.getByRole("menuitem", { name: "Fire", exact: true }).click();
  await expect(page.getByLabel("Remap assignment summary")).toContainText("Button 6: Fire");
  expect(await page.evaluate(() => window.__routingTest.remapSnapshot("alpha")?.MacroPending)).toBeNull();
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
    await page.getByLabel("Saved macro", { exact: true }).selectOption({ label: type });
    await page.getByLabel("Macro target button").selectOption("6");
    await page.getByRole("button", { name: "Stage macro assignment" }).click();
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
    await page.getByRole("button", { name: "Stage macro assignment" }).click();
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
    const file = page.getByLabel("Import macro JSON", { exact: true });
    await expect(file).toBeVisible();
    await page.screenshot({ path: testInfo.outputPath(`options-${width}.png`) });
    await page.getByRole("button", { name: "Save to library" }).scrollIntoViewIfNeeded();
    await page.screenshot({ path: testInfo.outputPath(`footer-${width}.png`) });
    const bounds = await file.boundingBox();
    const optionsBounds = await options.boundingBox();
    expect(bounds).not.toBeNull();
    expect(optionsBounds).not.toBeNull();
    // Reserve room for the native chooser and status without depending on its locale or font.
    expect(bounds!.width).toBeGreaterThanOrEqual(280);
    expect(bounds!.x).toBeGreaterThanOrEqual(optionsBounds!.x);
    expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(optionsBounds!.x + optionsBounds!.width);
    expect(optionsBounds!.x + optionsBounds!.width).toBeLessThanOrEqual(width);
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);
    await expect(page.getByRole("button", { name: "Add click" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Save to library" })).toBeVisible();
    await page.getByText("Advanced events and recording", { exact: true }).click();
    await expect(page.getByRole("button", { name: "Arm recording" })).toBeVisible();
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

test("editor records real left/middle/right clicks and synthetic back/forward without saving implicitly", async ({ page }) => {
  await page.goto("/e2e.html?offline");
  await page.getByRole("link", { name: "Macros", exact: true }).click();
  await page.getByRole("button", { name: "New macro", exact: true }).click();
  await page.getByLabel("Macro name", { exact: true }).fill("Five buttons");
  await page.getByText("Advanced events and recording", { exact: true }).click();
  const zone = page.getByRole("region", { name: "Mouse recording zone" });
  await zone.click({ button: "middle" }); // Passive input must not enter the draft.
  await page.getByRole("button", { name: "Arm recording" }).click();
  for (const button of ["left", "middle", "right"] as const) await zone.click({ button });
  // Playwright's mouse API has no back/forward buttons. These test DOM mapping,
  // not native browser history-navigation suppression.
  for (const button of [3, 4]) {
    await zone.dispatchEvent("mousedown", { button, bubbles: true, cancelable: true });
    await zone.dispatchEvent("mouseup", { button, bubbles: true, cancelable: true });
    await zone.dispatchEvent("auxclick", { button, bubbles: true, cancelable: true });
  }
  await page.getByRole("button", { name: "Stop recording" }).click();
  expect(await page.evaluate(() => window.__macroTest.calls)).toEqual([]);
  const types = ["mouse_left", "mouse_middle", "mouse_right", "mouse_back", "mouse_forward"];
  for (const [index, type] of types.entries()) {
    await expect(page.getByLabel(`Event ${index * 2 + 1} button`)).toHaveValue(type);
    await expect(page.getByLabel(`Event ${index * 2 + 2} action`)).toHaveValue("up");
  }
  await page.getByRole("button", { name: "Save to library" }).click();
  await expect(page.getByText("Saved to library. No device changes made.")).toBeVisible();
  expect(await page.evaluate(() => window.__macroTest.library()[0].events)).toEqual(
    types.flatMap((type) => ["down", "up"].map((action) => ({ type, action, delay_ms: 0 }))),
  );
  expect(await page.evaluate(() => window.__routingTest.calls.filter((call) => call.operation !== "SelectDevice"))).toEqual([]);
});

test("armed-zone real middle click prevents link navigation while passive middle click remains available", async ({ page, context }) => {
  await page.goto("/e2e.html?offline");
  await page.getByRole("link", { name: "Macros", exact: true }).click();
  await page.getByRole("button", { name: "New macro", exact: true }).click();
  const zone = page.getByRole("region", { name: "Mouse recording zone" });
  await page.getByText("Advanced events and recording", { exact: true }).click();
  const addLink = () => zone.evaluate((element) => {
    const link = document.createElement("a");
    link.href = "/e2e.html?offline&middle-link";
    link.textContent = "Middle navigation probe";
    element.append(link);
  });
  await page.getByRole("button", { name: "Arm recording" }).click();
  await addLink();
  const opened: string[] = [];
  const track = (popup: import("@playwright/test").Page) => { opened.push(popup.url()); };
  context.on("page", track);
  await page.getByRole("link", { name: "Middle navigation probe" }).click({ button: "middle" });
  await page.getByRole("button", { name: "Stop recording" }).click();
  await expect(page.getByLabel("Event 1 button")).toHaveValue("mouse_middle");
  expect(opened).toEqual([]);
  context.off("page", track);
  // A positive passive control verifies that actual Chromium middle-link
  // navigation is supported here, unlike synthetic back/forward dispatch.
  await addLink();
  const popupPromise = context.waitForEvent("page");
  await page.getByRole("link", { name: "Middle navigation probe" }).last().click({ button: "middle" });
  const popup = await popupPromise;
  await popup.waitForLoadState();
  expect(popup.url()).toContain("middle-link");
  await popup.close();
  await expect(page.getByLabel("Event 2 action")).toHaveValue("up");
  await expect(page.getByLabel("Event 3 button")).toHaveCount(0);
});

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
    await expect(page.getByLabel("Import macro JSON")).toBeVisible();
    await expect(page.getByRole("button", { name: "Export saved macro" })).toBeEnabled();
    await page.getByText("Advanced events and recording", { exact: true }).focus();
    await page.keyboard.press("Space");
    await expect(page.getByLabel("Event 10 action")).toHaveValue("up");
    await page.getByRole("button", { name: "Move event 10 up" }).click();
    await expect(page.getByLabel("Event 9 action")).toHaveValue("up");
    await page.getByRole("button", { name: "Save to library" }).click();
    await expect(page.getByText("Saved to library. No device changes made.")).toBeVisible();
    expect(await page.evaluate(() => window.__macroTest.library()[0].events.slice(-2).map((event) => event.action))).toEqual(["up", "down"]);
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
