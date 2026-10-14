import { test, expect, type Page, type Locator } from "@playwright/test";

test("local input recording preserves policies through save and export without device calls", async ({ page }) => {
  await page.goto("/e2e.html");
  await page.getByRole("link", { name: "Macros", exact: true }).click();
  await expect(page.getByRole("button", { name: "New macro", exact: true })).toBeEnabled();
  await page.getByRole("button", { name: "New macro", exact: true }).click();
  await page.getByLabel("Macro name", { exact: true }).fill("Local recording");
  const surface = page.getByRole("region", { name: "Local input capture", exact: true });
  const policy = page.getByRole("button", { name: "Recording delay", exact: true });
  const choosePolicy = async (label: string) => {
    await policy.click();
    const menu = page.getByRole("menu", { name: "Recording delay categories", exact: true });
    await expect(menu).toBeVisible();
    const background = await menu.evaluate(element => getComputedStyle(element).backgroundColor);
    expect(background).not.toBe("transparent");
    expect(background).not.toBe("rgba(0, 0, 0, 0)");
    expect(await menu.evaluate(element => getComputedStyle(element).opacity)).toBe("1");
    await menu.getByRole("menuitem", { name: label, exact: true }).click();
    await expect(policy).toHaveText(new RegExp(label.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")));
  };
  await choosePolicy("Minimum · 0 local ms");
  await page.getByRole("button", { name: "Start recording", exact: true }).click();
  await expect(surface).toBeFocused();
  await expect(policy).toBeDisabled();
  await expect(page.getByRole("button", { name: "Save to library", exact: true })).toBeDisabled();
  await page.keyboard.down("ControlLeft");
  await page.keyboard.press("KeyA");
  await page.keyboard.up("ControlLeft");
  await page.keyboard.press("Tab");
  await expect(surface).toHaveText("⌨ / 🖱");
  await choosePolicy("Default · 10 ms");
  await page.getByRole("button", { name: "Start recording", exact: true }).click();
  await surface.click({ button: "right" });
  await page.getByRole("button", { name: "Stop recording", exact: true }).click();
  await choosePolicy("Recorded · between presses");
  await page.getByRole("button", { name: "Start recording", exact: true }).click();
  await page.keyboard.press("KeyB");
  await page.keyboard.press("KeyC");
  await page.getByRole("button", { name: "Stop recording", exact: true }).click();
  const recordingOptions = page.locator(".macro-record-options");
  await expect(recordingOptions).not.toHaveAttribute("open", "");
  await page.getByText("Recording options", { exact: true }).click();
  await expect(page.getByText(/46 source events, 50 encoded pairs, 10 long-delay extensions/)).toHaveCount(0);
  await choosePolicy("Default · 10 ms");
  const captureTab = page.getByRole("checkbox", { name: /Record Tab/ });
  await captureTab.check();
  await page.getByRole("button", { name: "Start recording", exact: true }).click();
  await expect(surface).toBeFocused();
  await expect(captureTab).toBeDisabled();
  await page.keyboard.press("Tab");
  await expect(surface).toHaveText("⌨ / 🖱");
  await expect(page.getByRole("button", { name: "Start recording", exact: true })).toBeFocused();
  await page.getByRole("button", { name: "Save to library", exact: true }).click();
  await expect(page.getByText("✓ Saved")).toBeVisible();
  const saved = await page.evaluate(() => window.__macroTest.library().find(macro => macro.name === "Local recording")!);
  expect(saved.events.slice(0, 6)).toEqual([
    { type: "keyboard", key_code: "ControlLeft", action: "down", delay_ms: 0 },
    { type: "keyboard", key_code: "KeyA", action: "down", delay_ms: 0 },
    { type: "keyboard", key_code: "KeyA", action: "up", delay_ms: 0 },
    { type: "keyboard", key_code: "ControlLeft", action: "up", delay_ms: 0 },
    { type: "mouse_right", action: "down", delay_ms: 10 },
    { type: "mouse_right", action: "up", delay_ms: 10 },
  ]);
  expect(saved.events.slice(6, 10).map(event => [event.key_code, event.action])).toEqual([["KeyB", "down"], ["KeyB", "up"], ["KeyC", "down"], ["KeyC", "up"]]);
  expect(saved.events[6].delay_ms).toBeGreaterThanOrEqual(0);
  expect(saved.events.slice(7, 10).map(event => event.delay_ms)).toEqual([0, 0, 0]);
  expect(saved.events.slice(10)).toEqual([
    { type: "keyboard", key_code: "Tab", action: "down", delay_ms: 10 },
    { type: "keyboard", key_code: "Tab", action: "up", delay_ms: 10 },
  ]);
  await page.getByText("Macro options", { exact: true }).click();
  const downloadPromise = page.waitForEvent("download");
  await page.getByRole("button", { name: "Export saved macro", exact: true }).click();
  const stream = await (await downloadPromise).createReadStream();
  const chunks: Buffer[] = [];
  for await (const chunk of stream!) chunks.push(Buffer.from(chunk));
  expect(JSON.parse(Buffer.concat(chunks).toString())).toEqual({ version: 1, name: saved.name, events: saved.events });
  expect(await page.evaluate(() => window.__macroTest.calls)).toEqual(["CreateMacro"]);
  expect(await page.evaluate(() => window.__routingTest.calls)).toEqual([]);
});

for (const width of [1280, 620]) {
  test(`T7 concise recorder groups keypresses at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 1000 });
    await page.goto("/e2e.html");
    await page.getByRole("link", { name: "Macros", exact: true }).click();
    await page.getByRole("button", { name: "New macro", exact: true }).click();
    await page.getByLabel("Macro name", { exact: true }).fill("Visual recording");
    const options = page.locator(".macro-record-options");
    await expect(options).not.toHaveAttribute("open", "");
    await expect(page.getByRole("checkbox", { name: "Record Tab", exact: true })).not.toBeVisible();
    const surface = page.getByRole("region", { name: "Local input capture", exact: true });
    await expect(surface).toHaveAccessibleDescription("Press keys or click here.");
    const record = page.getByRole("button", { name: "Start recording", exact: true });
    await expect(record).toHaveText("● Record");
    await record.click();
    await expect(page.locator(".macro-record-indicator.active")).toHaveText("● Recording");
    await page.keyboard.press("KeyE");
    await expect(surface).toHaveText("Recording · 1 press");
    await page.keyboard.press("KeyS");
    await expect(surface).toHaveText("Recording · 2 presses");
    await page.getByRole("button", { name: "Stop recording", exact: true }).click();
    await expect(page.getByRole("status")).toHaveText("2 presses");
    const timeline = page.getByRole("list", { name: "Ordered timeline", exact: true });
    await expect(timeline.locator("strong")).toHaveText(["Key E", "Key S"]);
    await expect(timeline.locator("small")).toHaveText(["↓ 10 ms / ↑ 10 ms", "↓ 10 ms / ↑ 10 ms"]);
    await page.getByRole("button", { name: "Save to library", exact: true }).click();
    await expect(page.getByText("✓ Saved")).toBeVisible();
    const saved = await page.evaluate(() => window.__macroTest.library()[0]);
    expect(saved.events).toEqual(["KeyE", "KeyS"].flatMap(key_code => [
      { type: "keyboard", key_code, action: "down", delay_ms: 10 },
      { type: "keyboard", key_code, action: "up", delay_ms: 10 },
    ]));
    await page.locator(".macro-recorder").scrollIntoViewIfNeeded();
    await page.screenshot({ path: testInfo.outputPath(`recorder-${width}.png`) });
    expect(await page.locator(".macro-recorder").evaluate(element => element.scrollWidth <= element.clientWidth)).toBe(true);
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);
    await page.getByText("Recording options", { exact: true }).click();
    await expect(page.getByRole("checkbox", { name: "Record Tab", exact: true })).toBeVisible();
    expect(await page.evaluate(() => window.__routingTest.calls)).toEqual([]);
  });
}

async function openMacro(page: Page, button: number) {
  await page.getByRole("button", { name: `Button ${button} action`, exact: true }).click();
  const macro = page.getByRole("menuitem", { name: "Macro", exact: true });
  await expect(macro).not.toHaveAttribute("aria-haspopup", "menu");
  await macro.click();
  const dialog = page.getByRole("dialog", { name: `Button ${button} macro assignment`, exact: true });
  await expect(dialog).toBeVisible();
  return dialog;
}

async function selectDialogOption(dialog: Locator, field: string, name: string) {
  await dialog.getByRole("button", { name: field, exact: true }).click();
  const menu = dialog.getByRole("menu", { name: `${field} categories`, exact: true });
  await expect(menu).toBeVisible();
  await expect(menu).toHaveCSS("background-color", "rgb(57, 58, 54)");
  // A body portal is inert behind a native modal; the popup must be in its top layer.
  expect(await menu.evaluate((element) => !!element.closest("dialog[open]"))).toBe(true);
  await menu.getByRole("menuitem", { name, exact: true }).click();
}
async function assignMacro(page: Page, button: number, name: string, repeat = 1) {
  const calls = await page.evaluate(() => window.__routingTest.calls);
  const dialog = await openMacro(page, button);
  await selectDialogOption(dialog, "Saved macro", name);
  await expect(dialog.getByLabel("Repeat (1–255)", { exact: true })).toBeVisible();
  await dialog.getByLabel("Repeat (1–255)", { exact: true }).fill(String(repeat));
  expect(await page.evaluate(() => window.__routingTest.calls)).toEqual(calls);
  await dialog.getByRole("button", { name: "Confirm", exact: true }).focus();
  expect(await page.evaluate(() => window.__routingTest.calls)).toEqual(calls);
  await page.keyboard.press("Enter");
  await expect(dialog).not.toBeVisible();
}
import type { Macro, MacroPlaybackMode, RemapSnapshot } from "../src/desktop-contract";

declare global {
  interface Window { __macroTest: { calls: string[]; library(): Macro[]; uploads(): Array<{ button: number; source: Macro["events"]; rawCodes: number[] }>; stage(id: string, button: number, repeat: number, mode: MacroPlaybackMode): Promise<RemapSnapshot> } }
}

test("T6 F/D/S ten-event recording stages and applies strict mock transport without claiming playback", async ({ page }) => {
  await page.goto("/e2e.html");
  await page.getByRole("link", { name: "Macros", exact: true }).click();
  await expect(page.getByRole("button", { name: "New macro", exact: true })).toBeEnabled();
  await page.getByRole("button", { name: "New macro", exact: true }).click();
  await page.getByLabel("Macro name", { exact: true }).fill("Supported recording");
  await page.getByRole("button", { name: "Start recording", exact: true }).click();
  const surface = page.getByRole("region", { name: "Local input capture", exact: true });
  await expect(surface).toBeFocused();
  for (const key of ["KeyF", "KeyD", "KeyS", "KeyF", "KeyD"]) await page.keyboard.press(key);
  await page.getByRole("button", { name: "Stop recording", exact: true }).click();
  await expect(page.getByText(/Ready to assign/)).toBeVisible();
  await page.getByRole("button", { name: "Save to library", exact: true }).click();
  await expect(page.getByText("✓ Saved")).toBeVisible();
  const recorded = await page.evaluate(() => window.__macroTest.library()[0]);
  expect(recorded.events).toEqual(["KeyF", "KeyD", "KeyS", "KeyF", "KeyD"].flatMap(key_code => [
    { type: "keyboard", key_code, action: "down", delay_ms: 10 },
    { type: "keyboard", key_code, action: "up", delay_ms: 10 },
  ]));
  const labels = await page.getByRole("list", { name: "Ordered timeline" }).locator("strong").allTextContents();
  expect(labels).toEqual(["Key F", "Key D", "Key S", "Key F", "Key D"]);
  await expect(page.getByRole("region", { name: "Sequence", exact: true }).getByText("5 presses", { exact: true })).toBeVisible();
  expect(labels.join(" ")).not.toMatch(/\b(down|up)\b/);
  await page.getByRole("button", { name: "New macro", exact: true }).click();
  await page.getByLabel("Macro name", { exact: true }).fill("Legacy click");
  await page.getByRole("button", { name: "Add click", exact: true }).click();
  await page.getByRole("button", { name: "Save to library", exact: true }).click();
  await expect(page.getByText("✓ Saved")).toBeVisible();
  const importedEvents = recorded.events.map(event => ({ ...event, delay_ms: 100 }));
  const imports = [
    { name: "Captured default100", events: importedEvents, eligible: true },
    { name: "Unsupported retained", events: importedEvents.map(event => ({ ...event, key_code: "F13" })), eligible: false },
    { name: "Padded retained", events: importedEvents.map(event => ({ ...event, key_code: " KeyA " })), eligible: false },
    { name: "Overflow retained", events: importedEvents.map(event => ({ ...event, delay_ms: 51200 })), eligible: false },
    { name: "Alias collision retained", events: [
      { type: "keyboard", key_code: "Enter", action: "down", delay_ms: 10 },
      { type: "keyboard", key_code: "NumpadEnter", action: "down", delay_ms: 10 },
      { type: "keyboard", key_code: "Enter", action: "up", delay_ms: 10 },
      { type: "keyboard", key_code: "NumpadEnter", action: "up", delay_ms: 10 },
    ], eligible: false },
    { name: "Oversize retained", events: Array.from({ length: 24 }, () => [
      { type: "keyboard", key_code: "KeyF", action: "down", delay_ms: 10 },
      { type: "keyboard", key_code: "KeyF", action: "up", delay_ms: 10 },
    ]).flat(), eligible: false },
  ];
  for (const item of imports) {
    const options = page.locator(".macro-options");
    if (!await options.evaluate(element => (element as HTMLDetailsElement).open)) await page.getByText("Macro options", { exact: true }).click();
    await expect(page.getByRole("button", { name: "Import JSON", exact: true })).toBeEnabled();
    await page.getByLabel("Import macro JSON", { exact: true }).setInputFiles({ name: "macro.json", mimeType: "application/json", buffer: Buffer.from(JSON.stringify({ version: 1, name: item.name, events: item.events })) });
    await expect(page.getByLabel("Macro name", { exact: true })).toHaveValue(item.name);
    await expect(page.getByText(item.eligible ? /Ready to assign/ : /Local only/)).toBeVisible();
  }
  const library = await page.evaluate(() => window.__macroTest.library());
  expect(library).toHaveLength(8);
  for (const item of imports) expect(library.find(macro => macro.name === item.name)?.events).toEqual(item.events);
  expect(await page.evaluate(() => window.__routingTest.calls)).toEqual([]);
  expect(await page.evaluate(() => window.__macroTest.uploads())).toEqual([]);
  // The browser-only mock must reject invalid direct requests as well as UI filtering.
  const before = await page.evaluate(() => window.__routingTest.remapSnapshot("alpha"));
  for (const macro of library.filter(macro => macro.name.endsWith("retained"))) {
    expect(await page.evaluate(async id => { try { await window.__macroTest.stage(id, 6, 1, 0); return true; } catch { return false; } }, macro.id)).toBe(false);
  }
  expect(await page.evaluate(() => window.__routingTest.remapSnapshot("alpha"))).toEqual(before);
  expect(await page.evaluate(() => window.__routingTest.calls)).toEqual([]);
  await page.getByRole("link", { name: "Button remapping", exact: true }).click();
  const dialog = await openMacro(page, 6);
  await dialog.getByRole("button", { name: "Saved macro", exact: true }).click();
  for (const item of imports.filter(item => !item.eligible)) await expect(dialog.getByRole("menuitem", { name: item.name, exact: true })).not.toBeVisible();
  await dialog.getByRole("menuitem", { name: "Supported recording", exact: true }).click();
  for (const invalid of ["0", "256"]) {
    await dialog.getByLabel("Repeat (1–255)", { exact: true }).fill(invalid);
    await expect(dialog.getByRole("button", { name: "Confirm", exact: true })).toBeDisabled();
  }
  expect(await page.evaluate(() => window.__routingTest.calls)).toEqual([]);
  await dialog.getByLabel("Repeat (1–255)", { exact: true }).fill("255");
  await dialog.getByRole("button", { name: "Confirm", exact: true }).click();
  await expect(dialog).not.toBeVisible();
  for (const [button, name, modeLabel] of [[7, "Captured default100", "Repeat until pressed again"], [4, "Legacy click", "Repeat while button is held"]] as const) {
    const assignment = await openMacro(page, button);
    await selectDialogOption(assignment, "Saved macro", name);
    await selectDialogOption(assignment, "Playback mode", modeLabel);
    await expect(assignment.getByLabel("Repeat (1–255)", { exact: true })).not.toBeVisible();
    await assignment.getByRole("button", { name: "Confirm", exact: true }).click();
    await expect(assignment).not.toBeVisible();
  }
  const staged = await page.evaluate(() => window.__routingTest.remapSnapshot("alpha"));
  expect(staged?.MacroDrafts?.[6]).toMatchObject({ Events: recorded.events, Repeat: 255, Mode: 0 });
  expect(staged?.MacroDrafts?.[7]).toMatchObject({ Events: importedEvents, Repeat: 1, Mode: 1 });
  expect(staged?.MacroDrafts?.[4]).toMatchObject({ Events: [{ type: "mouse_left", action: "down", delay_ms: 0 }, { type: "mouse_left", action: "up", delay_ms: 0 }], Repeat: 1, Mode: 2 });
  expect(await page.evaluate(() => window.__routingTest.calls.map(call => call.operation))).toEqual(["StageMacroAssignment", "StageMacroAssignment", "StageMacroAssignment"]);
  await page.getByRole("button", { name: "Apply remap", exact: true }).click();
  await expect(page.getByLabel("Button remapping status", { exact: true })).toContainText("transport confirmed");
  await expect(page.getByLabel("Button remapping status", { exact: true })).toContainText("Playback and device persistence unverified");
  const applied = await page.evaluate(() => window.__routingTest.remapSnapshot("alpha"));
  expect(applied?.MacroAppliedDrafts).toEqual(staged?.MacroDrafts);
  expect(applied?.MacroProgress).toEqual({ Assignment: 2, Upload: 2 });
  const uploads = await page.evaluate(() => window.__macroTest.uploads());
  expect(uploads.find(upload => upload.button === 6)).toEqual({ button: 6, source: recorded.events, rawCodes: [9,9,7,7,0x16,0x16,9,9,7,7] });
  expect(await page.evaluate(() => window.__routingTest.calls.map(call => call.operation))).toEqual(["StageMacroAssignment", "StageMacroAssignment", "StageMacroAssignment", "ApplyRemap"]);
  expect(await page.evaluate(() => window.__macroTest.library())).toEqual(library);
  // A later explicit attempt can be partial; it does not erase prior success.
  await page.evaluate(() => window.__routingTest.failNextApply());
  await page.getByRole("button", { name: "Apply remap", exact: true }).click();
  await expect(page.getByLabel("Button remapping status", { exact: true })).toContainText("partial or unknown");
  const partial = await page.evaluate(() => window.__routingTest.remapSnapshot("alpha"));
  expect(partial?.MacroProgress).toEqual({ Assignment: 2, Upload: 1 });
  expect(partial?.MacroAppliedDrafts).toEqual(applied?.MacroAppliedDrafts);
  expect(partial?.MacroDrafts).toEqual(staged?.MacroDrafts);
  expect(await page.evaluate(() => window.__macroTest.library())).toEqual(library);
});

for (const width of [1280, 620]) {
  test(`saved macro assignment guidance leads to button remapping at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 1000 });
    await page.goto("/e2e.html");
    await page.getByRole("link", { name: "Macros", exact: true }).click();
    await page.getByRole("button", { name: "New macro", exact: true }).click();
    await expect(page.getByText("To assign this macro to a mouse button, open Button remapping.")).toHaveCount(0);
    await expect(page.getByRole("button", { name: "Assign to button" })).not.toBeVisible();
    await page.getByLabel("Macro name", { exact: true }).fill("Saved shortcut");
    for (const name of ["Middle click", "Forward click"]) {
      await page.getByRole("button", { name, exact: true }).click();
      await page.getByRole("button", { name: "Add click" }).click();
    }
    await page.getByRole("button", { name: "Save to library" }).click();
    await expect(page.getByText("✓ Saved")).toBeVisible();
    const saved = await page.evaluate(() => window.__macroTest.library()[0]);
    await page.getByRole("link", { name: "Button remapping", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Button remapping", exact: true, level: 1 })).toBeVisible();
    expect(await page.evaluate(() => window.__macroTest.calls)).toEqual(["CreateMacro"]);
    for (let button = 1; button <= 7; button++) {
      const selector = page.getByRole("button", { name: `Button ${button} action`, exact: true });
      await selector.scrollIntoViewIfNeeded();
      await expect(selector).toBeVisible();
      const bounds = await selector.boundingBox();
      expect(bounds!.x).toBeGreaterThanOrEqual(0);
      expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(width);
      const draft = await page.getByLabel("Remap assignment summary").textContent();
      const dialog = await openMacro(page, button);
      const modalBounds = await dialog.boundingBox();
      expect(modalBounds!.x).toBeGreaterThanOrEqual(0);
      expect(modalBounds!.x + modalBounds!.width).toBeLessThanOrEqual(width);
      expect(modalBounds!.y).toBeGreaterThanOrEqual(0);
      expect(modalBounds!.y + modalBounds!.height).toBeLessThanOrEqual(1000);
      await expect(dialog.getByRole("button", { name: "Saved macro" })).toBeFocused();
      await selectDialogOption(dialog, "Saved macro", saved.name);
      await dialog.getByLabel("Repeat (1–255)").fill("255");
      const savedMacro = dialog.getByRole("button", { name: "Saved macro" });
      const confirm = dialog.getByRole("button", { name: "Confirm" });
      for (const reverse of [false, true]) {
        await (reverse ? savedMacro : confirm).focus();
        const key = reverse ? "Shift+Tab" : "Tab";
        await page.keyboard.press(key);
        const boundary = await dialog.evaluate((element) => ({
          inside: element.contains(document.activeElement),
          browserChrome: document.activeElement === document.body && !document.hasFocus(),
          modal: element.matches(":modal"),
        }));
        // Native Chromium dialogs may cycle through browser chrome, represented
        // by an unfocused document/body, but never through background controls.
        expect(boundary.modal).toBe(true);
        expect(boundary.inside || boundary.browserChrome).toBe(true);
        if (boundary.browserChrome) await page.keyboard.press(key);
        await expect(reverse ? confirm : savedMacro).toBeFocused();
      }
      await savedMacro.focus();
      await selector.evaluate((element) => element.focus());
      await expect(savedMacro).toBeFocused();
      await page.keyboard.press("Tab");
      await expect(dialog.getByRole("button", { name: "Playback mode", exact: true })).toBeFocused();
      await page.keyboard.press("Tab");
      await expect(dialog.getByLabel("Repeat (1–255)")).toBeFocused();
      await page.keyboard.press("Tab");
      await expect(dialog.getByRole("button", { name: "Cancel", exact: true })).toBeFocused();
      await page.keyboard.press("Tab");
      await expect(confirm).toBeFocused();
      expect(await page.evaluate(() => window.__routingTest.calls)).toEqual([]);
      if (button % 2) await page.keyboard.press("Escape");
      else await dialog.getByRole("button", { name: "Cancel", exact: true }).click();
      await expect(dialog).not.toBeVisible();
      await expect(selector).toBeFocused();
      expect(await page.getByLabel("Remap assignment summary").textContent()).toBe(draft);
      expect(await page.evaluate(() => window.__routingTest.calls)).toEqual([]);
    }
    expect(await page.locator("#remapping-card").evaluate((element) => element.scrollWidth <= element.clientWidth)).toBe(true);
    await assignMacro(page, 6, saved.name, 2);
    await expect(page.getByLabel("Remap assignment summary")).toContainText("Saved shortcut × 2");
    const staged = await page.evaluate(() => window.__routingTest.remapSnapshot("alpha")?.MacroPending);
    expect(staged).toMatchObject({ ID: saved.id, Name: saved.name, Events: saved.events, Button: 6, Repeat: 2 });
    expect(await page.evaluate(() => window.__routingTest.calls.map((call) => call.operation))).toEqual(["StageMacroAssignment"]);
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
  await expect(page.getByText("✓ Saved")).toBeVisible();
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
    await expect(page.getByText("✓ Saved")).toBeVisible();
    expect(await page.evaluate(() => window.__routingTest.calls.filter((call) => call.operation !== "SelectDevice"))).toEqual([]);
    await page.getByRole("link", { name: "Button remapping", exact: true }).click();
    await assignMacro(page, 6, type);
    const frozen = await page.evaluate(() => window.__routingTest.remapSnapshot("alpha")?.MacroPending);
    expect(frozen?.Button).toBe(6);
    expect(frozen?.Events.map((event) => event.type)).toEqual([type, type]);
    await page.getByRole("link", { name: "Macros", exact: true }).click();
    await page.getByLabel("Macro name", { exact: true }).fill("Changed saved name");
    await page.getByRole("button", { name: "Save to library" }).click();
    await expect(page.getByText("✓ Saved")).toBeVisible();
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

for (const mode of [1, 2] as const) {
  test(`macro playback mode ${mode} stages locally and restores on reopen`, async ({ page }) => {
    await page.goto("/e2e.html");
    await page.getByRole("link", { name: "Macros", exact: true }).click();
    await page.getByRole("button", { name: "New macro", exact: true }).click();
    await page.getByLabel("Macro name", { exact: true }).fill("Mode click");
    await page.getByRole("button", { name: "Left click", exact: true }).click();
    await page.getByRole("button", { name: "Add click" }).click();
    await page.getByRole("button", { name: "Save to library" }).click();
    await expect(page.getByText("✓ Saved")).toBeVisible();
    await page.getByRole("link", { name: "Button remapping", exact: true }).click();
    const dialog = await openMacro(page, 6);
    await expect(dialog.getByRole("button", { name: "Playback mode" })).toContainText("Repeat a set number of times");
    await selectDialogOption(dialog, "Saved macro", "Mode click");
    const label = mode === 1 ? "Repeat until pressed again" : "Repeat while button is held";
    await selectDialogOption(dialog, "Playback mode", label);
    await expect(dialog.getByLabel("Repeat (1–255)")).not.toBeVisible();
    expect(await page.evaluate(() => window.__routingTest.calls)).toEqual([]);
    await dialog.getByRole("button", { name: "Confirm", exact: true }).click();
    await expect(dialog).not.toBeVisible();
    await expect(page.getByLabel("Remap assignment summary")).toContainText(label);
    expect(await page.evaluate(() => window.__routingTest.remapSnapshot("alpha")?.MacroPending)).toMatchObject({ Mode: mode, Repeat: 1, Button: 6 });
    expect(await page.evaluate(() => window.__routingTest.calls.map((call) => call.operation))).toEqual(["StageMacroAssignment"]);
    const reopened = await openMacro(page, 6);
    await expect(reopened.getByRole("button", { name: "Playback mode" })).toContainText(label);
    await reopened.getByRole("button", { name: "Cancel", exact: true }).click();
    expect(await page.evaluate(() => window.__routingTest.calls.filter((call) => call.operation === "ApplyRemap"))).toEqual([]);
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
    // Balanced five-button takes are now eligible even offline; this is not playback.
    await expect(page.getByText(/Ready to assign/)).toBeVisible();
    await page.getByRole("button", { name: "Save to library" }).click();
    await expect(page.getByText("✓ Saved")).toBeVisible();
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
