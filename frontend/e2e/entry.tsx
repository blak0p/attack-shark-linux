import { createRoot } from "react-dom/client";
import { App } from "../src/App";
import type { DesktopService, DPIConfig } from "../src/desktop-contract";
import "../src/styles.css";

// Browser-only service: no Wails runtime, bindings, or physical HID is exercised.
const ids = ["alpha", "beta"].map((Serial, index) => ({
  ID: { VendorID: 0x1d57, ProductID: 0xfa60, Serial },
  Profile: "attack-shark-x6", ProfileID: "attack-shark-x6",
  Path: `/dev/mock${index}`, Eligible: true, InventoryRevision: 1, SessionOnly: false,
}));
const config = () => ({
  DPI: [800, 1200, 1600, 2400, 3200, 6400, 12800, 26000], ActiveStage: 4,
  StageMask: 255, LiftDistance: 1, Colors: Array.from({ length: 8 }, () => [0, 255, 0]),
});
const snapshot = () => ({
  Connection: "dongle", Battery: 84, Applied: config(), Pending: config(), Factory: config(),
  Revision: 0, Error: { Code: "" },
});
const polling = (rate = 1000, failed = false) => ({
  Desired: rate, Applied: failed ? 1000 : rate, Persisted: 1000, Factory: 1000,
  Revision: 1, Error: { Code: failed ? "permission_denied" : "" },
  Firmware: failed ? "failed" : "success", Persistence: "success",
});
type DeviceID = (typeof ids)[number]["ID"];
const calls: Array<{ operation: string; destination: DeviceID; requested?: DeviceID; value?: number }> = [];
let selected = ids[0];
let currentPolling = polling();
let currentDPI = snapshot();
let failNext = false;
const record = (operation: string, value?: number, requested?: DeviceID) => {
  calls.push({ operation, destination: { ...selected.ID }, ...(requested ? { requested: { ...requested } } : {}), ...(value === undefined ? {} : { value }) });
};
const inventory = () => ({ Devices: ids, Selected: selected, Error: { Code: "" } });
const service = {
  GetApplicationVersion: undefined, CheckForUpdate: undefined,
  RefreshStatus: async () => currentDPI, GetSnapshot: async () => currentDPI,
  RefreshInventory: async () => inventory(),
  SelectDevice: async (id: DeviceID) => {
    const next = ids.find((item) => item.ID.Serial === id.Serial && item.ID.VendorID === id.VendorID && item.ID.ProductID === id.ProductID);
    if (!next) throw new Error("Unknown mock device");
    selected = next;
    record("SelectDevice", undefined, id);
    return inventory();
  },
  StageDPI: async (next: DPIConfig) => {
    const stage = currentDPI.Pending.ActiveStage - 1;
    const value = next.DPI[stage];
    if (stage !== 3 || !Number.isInteger(value) || value < 50 || value > 26000 || value % 50 !== 0 ||
        next.DPI.length !== 8 || next.DPI.some((dpi, index) => index !== stage && dpi !== currentDPI.Pending.DPI[index]) ||
        next.ActiveStage !== currentDPI.Pending.ActiveStage || next.StageMask !== currentDPI.Pending.StageMask ||
        next.LiftDistance !== currentDPI.Pending.LiftDistance || JSON.stringify(next.Colors) !== JSON.stringify(currentDPI.Pending.Colors)) {
      throw new Error("Unexpected DPI configuration");
    }
    record("StageDPI", value);
    currentDPI = { ...currentDPI, Pending: structuredClone(next), Firmware: "pending" };
    return currentDPI;
  },
  ApplyDPI: async () => {
    const value = currentDPI.Pending.DPI[currentDPI.Pending.ActiveStage - 1];
    record("ApplyDPI", value);
    currentDPI = { ...currentDPI, Applied: structuredClone(currentDPI.Pending), Firmware: "success" };
    return currentDPI;
  },
  GetPollingSnapshot: async () => currentPolling,
  StagePollingRate: async (rate: number) => {
    record("StagePollingRate", rate);
    currentPolling = { ...polling(rate), Firmware: "pending" };
    return currentPolling;
  },
  ApplyPollingRate: async () => {
    record("ApplyPollingRate");
    currentPolling = polling(currentPolling.Desired, failNext);
    failNext = false;
    return currentPolling;
  },
  GetDebounceSnapshot: async () => ({ Desired: 8, Applied: 8, Persisted: 8, Factory: 8, Revision: 0, Error: { Code: "" } }),
  GetLightingSnapshot: async () => ({ Pending: { Mode: 0, TemplateID: "off" }, Applied: null, Effects: [], Revision: 0, Error: { Code: "" } }),
  GetNormalSleepSnapshot: async () => ({ Pending: 0.5, Applied: 0.5, Persisted: 0.5, Revision: 0, Error: { Code: "" } }),
  GetRemapSnapshot: async () => ({ Pending: { Buttons: [] }, Applied: { Buttons: [] }, Factory: { Buttons: [] }, Actions: [], Revision: 0, Error: { Code: "" } }),
  OnStatusEvent: () => () => {}, OnConfiguration: () => () => {},
  OnPollingConfiguration: () => () => {}, OnRemapConfiguration: () => () => {},
};
// Unused operations fail loudly if the UI unexpectedly invokes them.
const guarded = new Proxy(service, {
  get(target, key) {
    if (typeof key === "string" && !(key in target)) return () => { throw new Error(`Unexpected mock operation: ${key}`); };
    return Reflect.get(target, key);
  },
}) as unknown as DesktopService;
Object.assign(window, { __routingTest: {
  calls, failNextApply: () => { failNext = true; }, selectDevice: service.SelectDevice,
  dpiSnapshot: () => structuredClone(currentDPI),
} });
createRoot(document.getElementById("root")!).render(<App service={guarded} />);
