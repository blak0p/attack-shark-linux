import { test, expect } from "@playwright/test";

type DeviceID = { VendorID: number; ProductID: number; Serial: string };
type Call = { operation: string; destination: DeviceID; requested?: DeviceID; value?: number };
const betaID: DeviceID = { VendorID: 0x1d57, ProductID: 0xfa60, Serial: "beta" };
declare global {
  interface Window { __routingTest: { calls: Call[]; failNextApply(): void; selectDevice(id: DeviceID): Promise<unknown>; dpiSnapshot(): { Applied: { DPI: number[] } } } }
}

test("mock rejects a mismatched product identity even when serial matches", async ({ page }) => {
  await page.goto("/e2e.html");
  await expect(page.getByRole("combobox", { name: "Mouse device" })).toHaveValue("alpha");
  await expect(page.evaluate((id) => window.__routingTest.selectDevice(id), { ...betaID, ProductID: 1 })).rejects.toThrow("Unknown mock device");
  await expect(page.getByRole("combobox", { name: "Mouse device" })).toHaveValue("alpha");
  expect(await page.evaluate(() => window.__routingTest.calls)).toEqual([]);
});

test("non-default mouse receives the requested polling operation", async ({ page }) => {
  await page.goto("/e2e.html");
  await page.getByRole("combobox", { name: "Mouse device" }).selectOption("beta");
  await expect(page.getByRole("combobox", { name: "Mouse device" })).toHaveValue("beta");
  await page.getByRole("radio", { name: "500 Hz" }).click();
  await expect(page.getByRole("status", { name: "Polling status" })).toContainText("Applied 500 Hz");
  await expect.poll(() => page.evaluate(() => window.__routingTest.calls)).toEqual([
    { operation: "SelectDevice", destination: betaID, requested: betaID },
    { operation: "StagePollingRate", destination: betaID, value: 500 },
    { operation: "ApplyPollingRate", destination: betaID },
  ]);
});

test("non-default mouse receives a validated DPI change through the UI", async ({ page }) => {
  await page.goto("/e2e.html");
  await page.getByRole("combobox", { name: "Mouse device" }).selectOption("beta");
  await expect(page.getByRole("combobox", { name: "Mouse device" })).toHaveValue("beta");
  await page.getByRole("slider", { name: "Stage 4 DPI" }).fill("3500");
  await expect(page.getByText("DPI applied.")).toBeVisible();
  await expect(page.getByRole("slider", { name: "Stage 4 DPI" })).toHaveValue("3500");
  expect(await page.evaluate(() => window.__routingTest.dpiSnapshot().Applied.DPI[3])).toBe(3500);
  await expect.poll(() => page.evaluate(() => window.__routingTest.calls)).toEqual([
    { operation: "SelectDevice", destination: betaID, requested: betaID },
    { operation: "StageDPI", destination: betaID, value: 3500 },
    { operation: "ApplyDPI", destination: betaID, value: 3500 },
  ]);
});

test("application failure is visible after targeting the non-default mouse", async ({ page }) => {
  await page.goto("/e2e.html");
  await page.getByRole("combobox", { name: "Mouse device" }).selectOption("beta");
  await expect(page.getByRole("combobox", { name: "Mouse device" })).toHaveValue("beta");
  await page.evaluate(() => window.__routingTest.failNextApply());
  await page.getByRole("radio", { name: "250 Hz" }).click();
  await expect(page.getByRole("status", { name: "Polling status" })).toContainText("Polling change failed");
  await expect(page.getByRole("alert")).toContainText("Permission denied");
  await expect.poll(() => page.evaluate(() => window.__routingTest.calls.slice(-2))).toEqual([
    { operation: "StagePollingRate", destination: betaID, value: 250 },
    { operation: "ApplyPollingRate", destination: betaID },
  ]);
});
