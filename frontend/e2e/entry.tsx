import { createRoot } from "react-dom/client";
import { App } from "../src/App";
import type { DesktopService, DPIConfig, RemapConfig, RemapSnapshot, RemapAction, Macro, MacroEvent, MacroPlaybackMode } from "../src/desktop-contract";
import "../src/styles.css";
import { deviceAdmission } from "../src/macros/device-admission";
import { x6KeyCode } from "../src/macros/x6keys";

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
const remaps = new Map<string, RemapSnapshot>(ids.map(({ ID }) => [ID.Serial, { Pending: remapFactory(), Applied: remapFactory(), Factory: remapFactory(), Actions: actions, MacroPending: null, MacroApplied: null, MacroProgress: { Assignment: 0, Upload: 0 }, Revision: 0, Firmware: "idle", Persistence: "idle", RetryAvailable: false, Error: { Code: "" } } satisfies RemapSnapshot]));
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
// Admission metadata only, not a mock firmware/serializer or playback proof.
const admittedUploads: Array<{ button: number; source: MacroEvent[]; rawCodes: number[] }> = [];
let macroID = 0;
const readMacro = (id: string) => {
  const macro = macros.get(id);
  if (!macro) throw new Error("Macro not found");
  return structuredClone(macro);
};
const updateScenario = new URLSearchParams(window.location.search).get("updates");
const updateCalls: Array<{ operation: string; args: unknown[] }> = [];
let updateAttempts = 0;
const updateInfo = { Version: "2.0.0", AppAvailable: true, AppInstalled: false, AppError: "",
  PolicyState: "outdated", PolicyInstalled: false, PolicyError: "", RelaunchError: "" };
const profileStates = new Map(ids.map(({ ID }) => [ID.Serial, {
  Profiles: [{ ID: 'automatic', Name: 'Automatic profile', CreatedAt: '2026-01-01T12:00:00Z' }],
  ActiveID: 'automatic', Notice: 'Automatic profile created at 2026-01-01T12:00:00Z from saved local settings. No mouse changes.', Error: { Code: '' }, Applied: false,
}]));
let profileCounter = 0;
const profileResult = () => structuredClone(profileStates.get(selected.ID.Serial)!);
const changeProfiles = (update: (state: ReturnType<typeof profileResult>) => void) => {
  const state = profileResult(); state.Error.Code = ''; state.Applied = false;
  update(state); profileStates.set(selected.ID.Serial, state); return profileResult();
};
// Browser-only stand-in for the watcher settings store; the backend validates for real.
type FakeRule = { Executable: string; Folder: string; Name: string; ProfileID: string };
let watcher: { Enabled: boolean; Rules: FakeRule[] } = {
  Enabled: true, Rules: [{ Executable: 'cs2', Folder: '', Name: '', ProfileID: 'automatic' }],
};
const gameCatalog = [
  { Name: 'Terraria', Source: 'Steam', Folder: '/home/user/.steam/steamapps/common/Terraria', Executable: '', IsGame: true },
  { Name: 'Rocket League', Source: 'Heroic', Folder: '/home/user/Games/Heroic/RocketLeague', Executable: '', IsGame: true },
  { Name: 'xemu', Source: 'App', Folder: '', Executable: 'xemu', IsGame: true },
  { Name: 'Firefox', Source: 'App', Folder: '', Executable: 'firefox', IsGame: false },
];
const cancelled = { Path: '', Name: '', Cancelled: true, Error: { Code: '' } };
const watcherResult = () => ({ ...structuredClone(watcher), Error: { Code: '' } });
const service = {
  GetWatcherSettings: async () => watcherResult(),
  GetLauncherPath: async () => ({ Path: '/home/user/My Apps/x6.AppImage', Error: { Code: '' } }),
  ListInstalledGames: async () => ({ Games: structuredClone(gameCatalog), Error: { Code: '' } }),
  PickGameFolder: async () => cancelled,
  SetWatcherSettings: async (enabled: boolean, rules: FakeRule[]) => {
    watcher = { Enabled: enabled, Rules: structuredClone(rules) };
    return watcherResult();
  },
  GetProfiles: async () => profileResult(),
  CreateProfile: async (name: string) => changeProfiles(state => {
    const id = `profile-${++profileCounter}`;
    state.Profiles.push({ ID: id, Name: name, CreatedAt: new Date().toISOString() }); state.ActiveID = id;
  }),
  RenameProfile: async (id: string, name: string) => changeProfiles(state => { state.Profiles.find(p => p.ID === id)!.Name = name; }),
  DuplicateProfile: async (_id: string, name: string) => changeProfiles(state => { state.Profiles.push({ ID: `profile-${++profileCounter}`, Name: name, CreatedAt: new Date().toISOString() }); }),
  DeleteProfile: async (id: string) => changeProfiles(state => {
    if (state.ActiveID === id || state.Profiles.length === 1) state.Error.Code = 'profile_delete_refused';
    else state.Profiles = state.Profiles.filter(p => p.ID !== id);
  }),
  SwitchProfile: async (id: string) => changeProfiles(state => {
    if (failNext) { failNext = false; state.Error.Code = 'partial_failure'; }
    else { state.ActiveID = id; state.Applied = true; }
  }),
  DismissProfileNotice: async () => changeProfiles(state => { state.Notice = ''; }),
  CheckUnifiedUpdates: updateScenario ? async (...args: unknown[]) => {
    updateCalls.push({ operation: "check", args });
    return { ...updateInfo, AppAvailable: updateScenario !== "usb-only" && updateScenario !== "current",
      PolicyState: updateScenario === "app-only" || updateScenario === "current" ? "current" : "outdated" };
  } : undefined,
  ApplyUnifiedUpdate: updateScenario ? async (...args: unknown[]) => {
    updateCalls.push({ operation: "apply", args });
    updateAttempts++;
    await new Promise(resolve => setTimeout(resolve, 150));
    if (updateScenario === "denial" && updateAttempts === 1) return { ...updateInfo, PolicyError: "authorization denied", AppError: "download failed" };
    if (updateScenario === "partial") return { ...updateInfo, AppInstalled: true, PolicyInstalled: true, PolicyError: "reload failed", RelaunchError: "launch failed" };
    return { ...updateInfo, AppAvailable: updateScenario !== "usb-only", AppInstalled: updateScenario !== "usb-only", PolicyInstalled: true };
  } : undefined,
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
  StageMacroAssignment: async (id: string, button: number, repeat: number): Promise<RemapSnapshot> =>
    service.StageMacroAssignmentWithMode(id, button, repeat, 0),
  StageMacroAssignmentWithMode: async (id: string, button: number, repeat: number, mode: MacroPlaybackMode): Promise<RemapSnapshot> => {
    const macro = readMacro(id);
    if (offline || !Number.isInteger(button) || button < 1 || button > 7 ||
        !deviceAdmission(macro.events, repeat, mode).eligible) throw new Error("Invalid macro assignment");
    validateRemap(remapSnapshot().Pending);
    record("StageMacroAssignment");
    const current = remapSnapshot();
    const draft = { ID: id, Name: macro.name, Button: button, Repeat: repeat, Mode: mode, Events: structuredClone(macro.events) };
    const drafts = { ...(current.MacroDrafts ?? {}), [button]: draft };
    remaps.set(selected.ID.Serial, { ...current, MacroPending: draft, MacroDrafts: drafts, Revision: current.Revision + 1, Error: { Code: "" } });
    return remapSnapshot();
  },
  StageRemap: async (config: RemapConfig) => {
    validateRemap(config); record("StageRemap");
    const current = remapSnapshot();
    remaps.set(selected.ID.Serial, { ...current, Pending: structuredClone(config), Revision: current.Revision + 1, Error: { Code: "" } });
    return remapSnapshot();
  },
  ClearButtonMacroAssignment: async (button: number) => {
    record("ClearButtonMacroAssignment");
    const current = remapSnapshot();
    const drafts = { ...(current.MacroDrafts ?? {}) };
    delete drafts[button];
    const remaining = Object.values(drafts);
    const macroPending = remaining.length > 0 ? remaining[0] : null;
    remaps.set(selected.ID.Serial, { ...current, MacroPending: macroPending, MacroDrafts: drafts, Revision: current.Revision + 1, Error: { Code: "" } });
    return remapSnapshot();
  },
  ClearMacroAssignment: async () => {
    record("ClearMacroAssignment");
    const current = remapSnapshot();
    remaps.set(selected.ID.Serial, { ...current, MacroPending: null, MacroDrafts: {}, Revision: current.Revision + 1, Error: { Code: "" } });
    return remapSnapshot();
  },
  DiscardRemap: async () => {
    record("DiscardRemap");
    const current = remapSnapshot();
    remaps.set(selected.ID.Serial, { ...current, Pending: structuredClone(current.Applied), MacroPending: null, MacroDrafts: {}, Revision: current.Revision + 1, Error: { Code: "" } });
    return remapSnapshot();
  },
  ApplyRemap: async (config: RemapConfig) => {
    validateRemap(config);
    const current = remapSnapshot();
    const draft = current.MacroPending;
    const drafts = current.MacroDrafts ?? (draft ? { [draft.Button]: draft } : {});
    const allDrafts = Object.values(drafts);
    const seen = new Set<number>();
    const admitted = !offline && allDrafts.every((d) => {
      if (!Number.isInteger(d.Button) || d.Button < 1 || d.Button > 7 || seen.has(d.Button)) return false;
      seen.add(d.Button);
      return deviceAdmission(d.Events, d.Repeat, d.Mode ?? 0).eligible &&
        JSON.stringify(macros.get(d.ID)) === JSON.stringify({ id: d.ID, name: d.Name, events: d.Events });
    });
    if (!admitted) {
      // Invalid attempts leave draft/revision/progress untouched and never count I/O.
      remaps.set(selected.ID.Serial, { ...current, Firmware: "failed", Error: { Code: "invalid_configuration" } });
      return remapSnapshot();
    }
    calls.push({ operation: "ApplyRemap", destination: { ...selected.ID }, config: structuredClone(config) });
    const failed = failNext;
    failNext = false;
    const hasMacros = allDrafts.length > 0;
    for (const d of allDrafts) admittedUploads.push({ button: d.Button, source: structuredClone(d.Events), rawCodes: d.Events.map(event =>
      event.type === "keyboard" ? x6KeyCode(event.key_code!)! : 0xf1 + ["mouse_left", "mouse_right", "mouse_middle", "mouse_back", "mouse_forward"].indexOf(event.type)) });
    remaps.set(selected.ID.Serial, {
      ...current, Pending: structuredClone(config), Revision: current.Revision + 1,
      Applied: failed ? current.Applied : structuredClone(config),
      MacroApplied: failed ? current.MacroApplied : structuredClone(draft ?? null),
      MacroAppliedDrafts: failed ? current.MacroAppliedDrafts : structuredClone(drafts),
      MacroProgress: hasMacros ? { Assignment: 2, Upload: failed ? 1 : 2 } : current.MacroProgress,
      Firmware: failed ? "failed" : "success", Persistence: failed ? "" : hasMacros ? "not_supported" : "success",
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
    const operation = Reflect.get(target, key);
    // Opt-in latency makes pending feedback observable; default routing fixtures are unchanged.
    if (new URLSearchParams(window.location.search).has("feedback") && typeof operation === "function" &&
        ["ApplyDPI", "ApplyPollingRate", "ApplyRemap", "CreateMacro", "UpdateMacro", "DeleteMacro", "ReadMacro"].includes(String(key))) {
      return async (...args: unknown[]) => {
        await new Promise(resolve => setTimeout(resolve, 400));
        return operation(...args);
      };
    }
    return operation;
  },
}) as unknown as DesktopService;
Object.assign(window, { __updatesTest: { calls: updateCalls }, __macroTest: { calls: macroCalls, library: () => [...macros.values()].map((macro) => structuredClone(macro)), uploads: () => structuredClone(admittedUploads), stage: service.StageMacroAssignmentWithMode }, __routingTest: {
  calls, failNextApply: () => { failNext = true; }, selectDevice: service.SelectDevice,
  dpiSnapshot: () => structuredClone(currentDPI), remapSnapshot: (serial: string) => structuredClone(remaps.get(serial)),
} });
createRoot(document.getElementById("root")!).render(<App service={guarded} />);
