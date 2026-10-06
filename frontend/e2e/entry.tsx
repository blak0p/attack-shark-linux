import { createRoot } from "react-dom/client";
import { App } from "../src/App";
import type { DesktopService, DPIConfig, RemapConfig, RemapSnapshot, RemapAction, Macro, MacroEvent } from "../src/desktop-contract";
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
const calls: Array<{ operation: string; destination: DeviceID; requested?: DeviceID; value?: number; config?: RemapConfig }> = [];
const remapFactory = (): RemapConfig => ({ Buttons: [
  { Button: 1, Action: "left", PreservedDefault: "" },
  { Button: 2, Action: "right", PreservedDefault: "" },
  { Button: 3, Action: "middle", PreservedDefault: "" },
  { Button: 4, Action: "forward", PreservedDefault: "" },
  { Button: 5, Action: "backward", PreservedDefault: "" },
  { Button: 6, Action: null, PreservedDefault: "DPI+" },
  { Button: 7, Action: null, PreservedDefault: "DPI-" },
] });
const actions: RemapAction[] = ["off", "left", "right", "middle", "forward", "backward", "double_click", "fire", "browser_calculator", "browser_email", "browser_forward", "browser_backward", "browser_stop", "browser_my_computer", "browser_refresh", "browser_home", "browser_search"];
const remaps = new Map<string, RemapSnapshot>(ids.map(({ ID }) => [ID.Serial, { Pending: remapFactory(), Applied: remapFactory(), Factory: remapFactory(), Actions: actions, Revision: 0, Firmware: "idle", Persistence: "idle", RetryAvailable: false, Error: { Code: "" } } satisfies RemapSnapshot]));
const remapSnapshot = () => structuredClone(remaps.get(selected.ID.Serial)!);
const validateRemap = (config: RemapConfig) => {
  if (config.Buttons.length !== 7 || config.Buttons.some((button, index) =>
    button.Button !== index + 1 || (button.Action !== null ? !actions.includes(button.Action) :
      button.PreservedDefault !== (button.Button === 6 ? "DPI+" : button.Button === 7 ? "DPI-" : "" ) || button.Button < 6))) {
    throw new Error("Invalid mock remap draft");
  }
};
let selected = ids[0];
let currentPolling = polling();
let currentDPI = snapshot();
let failNext = false;
const record = (operation: string, value?: number, requested?: DeviceID) => {
  calls.push({ operation, destination: { ...selected.ID }, ...(requested ? { requested: { ...requested } } : {}), ...(value === undefined ? {} : { value }) });
};
const offline = new URLSearchParams(window.location.search).has("offline");
if (offline) currentDPI = { ...currentDPI, Error: { Code: "device_disconnected" } };
const inventory = () => ({ Devices: offline ? [] : ids, Selected: offline ? null : selected, Error: { Code: "" } });
const macros = new Map<string, Macro>();
const macroCalls: string[] = [];
let macroID = 0;
const readMacro = (id: string) => {
  const macro = macros.get(id);
  if (!macro) throw new Error("Macro not found");
  return structuredClone(macro);
};
const service = {
  ListMacros: async () => [...macros.values()].map((macro) => structuredClone(macro)),
  ReadMacro: async (id: string) => readMacro(id),
  CreateMacro: async (name: string, events: MacroEvent[]) => {
    const macro = { id: `macro-${++macroID}`, name, events: structuredClone(events) };
    macros.set(macro.id, macro); macroCalls.push("CreateMacro"); return structuredClone(macro);
  },
  UpdateMacro: async (id: string, name: string, events: MacroEvent[]) => {
    readMacro(id);
    const macro = { id, name, events: structuredClone(events) };
    macros.set(id, macro); macroCalls.push("UpdateMacro"); return structuredClone(macro);
  },
  DeleteMacro: async (id: string) => { readMacro(id); macros.delete(id); macroCalls.push("DeleteMacro"); },
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
  GetRemapSnapshot: async () => remapSnapshot(),
  GetMacroAssignmentSnapshot: async () => remapSnapshot(),
  StageMacroAssignment: async (id: string, button: number, repeat: number) => {
    const macro = readMacro(id);
    if (offline || !Number.isInteger(button) || button < 1 || button > 7 ||
        !Number.isInteger(repeat) || repeat < 1 || repeat > 255 || macro.events.length !== 2 ||
        !["mouse_left", "mouse_right"].includes(macro.events[0].type) ||
        macro.events[0].type !== macro.events[1].type || macro.events[0].action !== "down" ||
        macro.events[1].action !== "up" || macro.events.some((event) => event.delay_ms !== 0)) throw new Error("Invalid macro assignment");
    record("StageMacroAssignment");
    const current = remapSnapshot();
    remaps.set(selected.ID.Serial, { ...current, MacroPending: { ID: id, Name: macro.name, Button: button, Repeat: repeat, Events: macro.events }, Revision: current.Revision + 1, Error: { Code: "" } });
    return remapSnapshot();
  },
  StageRemap: async (config: RemapConfig) => {
    validateRemap(config); record("StageRemap");
    const current = remapSnapshot();
    remaps.set(selected.ID.Serial, { ...current, Pending: structuredClone(config), Revision: current.Revision + 1, Error: { Code: "" } });
    return remapSnapshot();
  },
  ClearMacroAssignment: async () => {
    record("ClearMacroAssignment");
    const current = remapSnapshot();
    remaps.set(selected.ID.Serial, { ...current, MacroPending: null, Revision: current.Revision + 1, Error: { Code: "" } });
    return remapSnapshot();
  },
  DiscardRemap: async () => {
    record("DiscardRemap");
    const current = remapSnapshot();
    remaps.set(selected.ID.Serial, { ...current, Pending: structuredClone(current.Applied), MacroPending: null, Revision: current.Revision + 1, Error: { Code: "" } });
    return remapSnapshot();
  },
  ApplyRemap: async (config: RemapConfig) => {
    validateRemap(config);
    calls.push({ operation: "ApplyRemap", destination: { ...selected.ID }, config: structuredClone(config) });
    const current = remapSnapshot();
    const draft = current.MacroPending;
    const libraryCurrent = !draft || JSON.stringify(macros.get(draft.ID)) === JSON.stringify({ id: draft.ID, name: draft.Name, events: draft.Events });
    if (!libraryCurrent) {
      remaps.set(selected.ID.Serial, { ...current, Pending: structuredClone(config), Revision: current.Revision + 1, Firmware: "failed", Persistence: "", Error: { Code: "invalid_configuration" } });
      return remapSnapshot();
    }
    const failed = failNext;
    failNext = false;
    remaps.set(selected.ID.Serial, {
      ...current, Pending: structuredClone(config), Revision: current.Revision + 1,
      Applied: failed ? current.Applied : structuredClone(config),
      MacroApplied: failed ? current.MacroApplied : structuredClone(draft ?? null),
      MacroProgress: draft ? { Assignment: 2, Upload: failed ? 1 : 2 } : current.MacroProgress,
      Firmware: failed ? "failed" : "success", Persistence: failed ? "" : draft ? "not_supported" : "success",
      Error: { Code: failed ? "apply_failed" : "" },
    });
    return remapSnapshot();
  },
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
Object.assign(window, { __macroTest: { calls: macroCalls, library: () => [...macros.values()].map((macro) => structuredClone(macro)) }, __routingTest: {
  calls, failNextApply: () => { failNext = true; }, selectDevice: service.SelectDevice,
  dpiSnapshot: () => structuredClone(currentDPI), remapSnapshot: (serial: string) => structuredClone(remaps.get(serial)),
} });
createRoot(document.getElementById("root")!).render(<App service={guarded} />);
