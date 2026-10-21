import "@testing-library/jest-dom/vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useDesktopWorkspace } from "./useDesktopWorkspace";
import type { DesktopService, PollingConfigurationEvent, RemapConfig, RemapAction, RemapSnapshot, RemapConfigurationEvent, StatusEvent } from "../desktop-contract";

afterEach(() => vi.restoreAllMocks());

const binding = { ID: { VendorID: 0x1d57, ProductID: 0xfa60, Serial: "alpha" }, Path: "/dev/hidraw0", InventoryRevision: 1 };
const dpi = { DPI: [800, 1200], ActiveStage: 1, StageMask: 3, LiftDistance: 1 };
const snapshot = (overrides = {}) => ({ Connection: "dongle", Battery: 84, Applied: dpi, Pending: dpi, Factory: dpi, Revision: 0, Error: { Code: "" }, ...overrides });
const polling = (overrides = {}) => ({ Desired: 1000, Applied: 1000, Factory: 1000, Revision: 0, ...overrides });
const debounce = (overrides = {}) => ({ Desired: 8, Applied: 8, Factory: 8, Revision: 0, Firmware: "", Persistence: "", RetryAvailable: false, Error: { Code: "" }, ...overrides });
const normalSleep = (overrides = {}) => ({ Pending: 0.5, Applied: 0.5, Revision: 0, Firmware: "", Persistence: "", RetryAvailable: false, Error: { Code: "" }, ...overrides });
const lighting = (overrides = {}) => ({ Pending: { Mode: 0x10 as const, TemplateID: "fixed-green" }, Applied: null, Effects: [], Revision: 0, Firmware: "", Error: { Code: "" }, ...overrides });
const buttons: RemapConfig = { Buttons: ["left", "right", "middle", "forward", "backward", "dpi_plus", "dpi_minus"].map((Action, index) => ({ Button: index + 1, Action: Action as RemapAction, PreservedDefault: "" })) };
const remap = (overrides: Partial<RemapSnapshot> = {}): RemapSnapshot => ({ Pending: buttons, Applied: buttons, Factory: buttons, Actions: ["off", "left", "right", "middle", "forward", "backward", "double_click", "fire"], Revision: 0, Firmware: "", Persistence: "", RetryAvailable: false, Error: { Code: "" }, ...overrides });

function serviceFor(overrides: Partial<DesktopService> = {}) {
  const statusListeners: Array<(event: StatusEvent) => void> = [];
  const configurationListeners: Array<(event: { Binding: typeof binding; Snapshot: ReturnType<typeof snapshot> }) => void> = [];
  const pollingListeners: Array<(event: PollingConfigurationEvent) => void> = [];
  const remapListeners: Array<(event: RemapConfigurationEvent) => void> = [];
  const unsubscribeStatus = vi.fn();
  const unsubscribeConfiguration = vi.fn();
  const unsubscribePolling = vi.fn(); const unsubscribeRemap = vi.fn();
  const service: DesktopService = {
    ListMacros: vi.fn().mockResolvedValue([]), ReadMacro: vi.fn(), CreateMacro: vi.fn(), UpdateMacro: vi.fn(), DeleteMacro: vi.fn(),
    GetSnapshot: vi.fn().mockResolvedValue(snapshot()),
    GetPollingSnapshot: vi.fn().mockResolvedValue(polling()),
    GetDebounceSnapshot: vi.fn().mockResolvedValue(debounce()),
		GetLightingSnapshot: vi.fn().mockResolvedValue(lighting()),
    GetNormalSleepSnapshot: vi.fn().mockResolvedValue(normalSleep()),
		GetRemapSnapshot: vi.fn().mockResolvedValue(remap()),
    RefreshStatus: vi.fn().mockResolvedValue(snapshot()),
    RefreshInventory: vi.fn().mockResolvedValue({ Devices: [binding], Selected: binding, Error: { Code: "" } }),
    SelectDevice: vi.fn().mockResolvedValue({ Devices: [binding], Selected: binding, Error: { Code: "" } }),
    StageDPI: vi.fn().mockImplementation(async (next) => snapshot({ Pending: next })),
    ApplyDPI: vi.fn().mockResolvedValue(snapshot({ Firmware: "success" })),
    StagePollingRate: vi.fn().mockImplementation(async (rate) => polling({ Desired: rate })),
    ApplyPollingRate: vi.fn().mockResolvedValue(polling({ Firmware: "success" })),
    StageDebounce: vi.fn().mockImplementation(async (value) => debounce({ Desired: value })),
    ApplyDebounce: vi.fn().mockResolvedValue(debounce({ Firmware: "success" })),
    RetryDebouncePersistence: vi.fn().mockResolvedValue(debounce()),
    StageNormalSleep: vi.fn().mockImplementation(async (value) => normalSleep({ Pending: value })),
    ApplyNormalSleep: vi.fn().mockResolvedValue(normalSleep({ Firmware: "success" })),
    RetryNormalSleepPersistence: vi.fn().mockResolvedValue(normalSleep()),
		StageLighting: vi.fn().mockResolvedValue(lighting()),
		ApplyLighting: vi.fn().mockResolvedValue(lighting()),
    StageRemap: vi.fn().mockImplementation(async (Pending) => remap({ Pending })),
    StageMacroAssignmentWithMode: vi.fn().mockResolvedValue(remap()),
    StageMacroAssignment: vi.fn().mockResolvedValue(remap()),
    ClearButtonMacroAssignment: vi.fn().mockResolvedValue(remap()),
    ClearMacroAssignment: vi.fn().mockResolvedValue(remap()),
    DiscardRemap: vi.fn().mockResolvedValue(remap()),
    GetMacroAssignmentSnapshot: vi.fn().mockResolvedValue(remap()),
    ApplyRemap: vi.fn().mockResolvedValue(remap()),
		RetryRemapPersistence: vi.fn().mockResolvedValue(remap()),
    ResetToFactory: vi.fn().mockResolvedValue({ Lanes: [], Cleanup: { Lane: "cleanup", State: "success", Code: "" }, Error: { Code: "" }, RetryAvailable: false }),
    RetryPersistence: vi.fn().mockResolvedValue(snapshot()),
    RetryPollingPersistence: vi.fn().mockResolvedValue(polling()),
    OnStatusEvent: vi.fn().mockImplementation((callback) => { statusListeners.push(callback); return unsubscribeStatus; }),
    OnConfiguration: vi.fn().mockImplementation((callback) => { configurationListeners.push(callback); return unsubscribeConfiguration; }),
    OnPollingConfiguration: vi.fn().mockImplementation((callback) => { pollingListeners.push(callback); return unsubscribePolling; }),
		OnRemapConfiguration: vi.fn().mockImplementation((callback) => { remapListeners.push(callback); return unsubscribeRemap; }),
    ...overrides,
  };
  if (overrides.GetMacroAssignmentSnapshot && !overrides.GetRemapSnapshot) service.GetRemapSnapshot = overrides.GetMacroAssignmentSnapshot;
  return { service, statusListeners, configurationListeners, pollingListeners, remapListeners, unsubscribeStatus, unsubscribeConfiguration, unsubscribePolling, unsubscribeRemap };
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error: Error) => void;
  const promise = new Promise<T>((resolvePromise, rejectPromise) => { resolve = resolvePromise; reject = rejectPromise; });
  return { promise, resolve, reject };
}

const macro = { ID: "click", Name: "Click", Button: 6, Repeat: 2, Events: [{ type: "mouse_left" as const, action: "down" as const, delay_ms: 0 }, { type: "mouse_left" as const, action: "up" as const, delay_ms: 0 }] };
const macro7 = { ID: "click7", Name: "Click7", Button: 7, Repeat: 3, Events: [{ type: "mouse_right" as const, action: "down" as const, delay_ms: 0 }, { type: "mouse_right" as const, action: "up" as const, delay_ms: 0 }] };

const unified = (patch = {}) => ({ Version: "2.0", AppAvailable: true, AppInstalled: false, AppError: "", PolicyState: "outdated", PolicyInstalled: false, PolicyError: "", RelaunchError: "", ...patch });

describe("unified update lifecycle", () => {
  it.each([
    {}, { AppAvailable: false }, { PolicyState: "current" },
    { AppAvailable: false, PolicyState: "current" },
    { PolicyState: "custom" }, { PolicyState: "unavailable" },
    { AppError: "network", PolicyError: "policy unavailable" },
  ])("checks once without applying and retains exact outcomes %j", async (patch) => {
    const info = unified(patch);
    const h = serviceFor({ CheckUnifiedUpdates: vi.fn().mockResolvedValue(info), ApplyUnifiedUpdate: vi.fn(), CheckForUpdate: vi.fn().mockResolvedValue(null), ApplyVerifiedUpdate: vi.fn() });
    const { result, rerender } = renderHook(() => useDesktopWorkspace(h.service));
    await waitFor(() => expect(result.current.model.unifiedUpdate).toEqual(info));
    rerender();
    await act(async () => result.current.actions.selectStage(0));
    expect(h.service.CheckUnifiedUpdates).toHaveBeenCalledExactlyOnceWith();
    expect(h.service.CheckForUpdate).not.toHaveBeenCalled();
    expect(h.service.ApplyUnifiedUpdate).not.toHaveBeenCalled();
    expect(h.service.ApplyVerifiedUpdate).not.toHaveBeenCalled();
    expect(result.current.model.updateChecking).toBe(false);
    expect(result.current.model.updateApplying).toBe(false);
    expect(result.current.model.update).toBeUndefined();
  });

  it("serializes check/apply synchronously and retains partial, rejected and null outcomes", async () => {
    const check = deferred<ReturnType<typeof unified> | null>();
    const apply = deferred<ReturnType<typeof unified> | null>();
    const partial = unified({ AppInstalled: true, PolicyError: "denied", RelaunchError: "launch failed" });
    const h = serviceFor({ CheckUnifiedUpdates: vi.fn().mockReturnValueOnce(check.promise).mockResolvedValue(unified()), ApplyUnifiedUpdate: vi.fn().mockReturnValueOnce(apply.promise).mockRejectedValueOnce(new Error("unknown effects")).mockResolvedValueOnce(null) });
    const { result } = renderHook(() => useDesktopWorkspace(h.service));
    act(() => { result.current.actions.checkUpdates!(); result.current.actions.applyUpdate(); });
    expect(h.service.CheckUnifiedUpdates).toHaveBeenCalledTimes(1);
    expect(h.service.ApplyUnifiedUpdate).not.toHaveBeenCalled();
    expect(result.current.model.updateChecking).toBe(true);
    await act(async () => check.resolve(unified()));
    act(() => { result.current.actions.applyUpdate(); result.current.actions.applyUpdate(); result.current.actions.checkUpdates!(); });
    expect(h.service.ApplyUnifiedUpdate).toHaveBeenCalledExactlyOnceWith();
    expect(h.service.CheckUnifiedUpdates).toHaveBeenCalledTimes(1);
    expect(result.current.model.updateApplying).toBe(true);
    await act(async () => apply.resolve(partial));
    expect(result.current.model.unifiedUpdate).toEqual(partial);
    expect(result.current.model.updateApplying).toBe(false);
    await act(async () => result.current.actions.applyUpdate());
    expect(result.current.model.unifiedUpdate).toEqual(partial);
    expect(result.current.model.updateError).toBe("unknown effects");
    await act(async () => result.current.actions.applyUpdate());
    expect(result.current.model.unifiedUpdate).toEqual(partial);
    expect(result.current.model.updateError).toBe("No update result returned.");
    await act(async () => { result.current.actions.checkUpdates!(); result.current.actions.checkUpdates!(); });
    expect(h.service.CheckUnifiedUpdates).toHaveBeenCalledTimes(2);
    expect(result.current.model.updateError).toBe("");
  });

  it.each(["check", "apply"])("ignores stale %s completion after replacement and unmount", async (kind) => {
    const pending = deferred<ReturnType<typeof unified>>();
    const old = serviceFor({ CheckUnifiedUpdates: vi.fn().mockReturnValue(kind === "check" ? pending.promise : Promise.resolve(unified())), ApplyUnifiedUpdate: vi.fn().mockReturnValue(pending.promise) });
    const next = serviceFor({ CheckUnifiedUpdates: vi.fn().mockResolvedValue(unified({ Version: "3" })), ApplyUnifiedUpdate: vi.fn() });
    const { result, rerender, unmount } = renderHook(({ service }) => useDesktopWorkspace(service), { initialProps: { service: old.service } });
    if (kind === "apply") { await waitFor(() => expect(result.current.model.unifiedUpdate).toBeDefined()); act(() => result.current.actions.applyUpdate()); }
    rerender({ service: next.service });
    await waitFor(() => expect(result.current.model.unifiedUpdate?.Version).toBe("3"));
    await act(async () => pending.resolve(unified({ Version: "obsolete" })));
    expect(result.current.model.unifiedUpdate?.Version).toBe("3");
    expect(result.current.model.updateApplying).toBe(false);
    const late = deferred<ReturnType<typeof unified>>();
    vi.mocked(next.service.ApplyUnifiedUpdate!).mockReturnValue(late.promise);
    act(() => result.current.actions.applyUpdate());
    unmount();
    await act(async () => late.reject(new Error("unmounted")));
  });

  it("reports check rejection/null without erasing known outcomes and retries", async () => {
    const h = serviceFor({ CheckUnifiedUpdates: vi.fn().mockResolvedValueOnce(unified()).mockRejectedValueOnce(new Error("offline")).mockResolvedValueOnce(null), ApplyUnifiedUpdate: vi.fn() });
    const { result } = renderHook(() => useDesktopWorkspace(h.service));
    await waitFor(() => expect(result.current.model.unifiedUpdate).toEqual(unified()));
    await act(async () => result.current.actions.checkUpdates!());
    expect(result.current.model.updateError).toBe("offline");
    expect(result.current.model.unifiedUpdate).toEqual(unified());
    await act(async () => result.current.actions.checkUpdates!());
    expect(result.current.model.updateError).toBe("No update result returned.");
    expect(result.current.model.unifiedUpdate).toEqual(unified());
    expect(h.service.ApplyUnifiedUpdate).not.toHaveBeenCalled();
  });

  it.each(["check", "apply"])("falls back for a partial %s capability", async (method) => {
    const h = serviceFor({ CheckUnifiedUpdates: method === "check" ? vi.fn() : undefined, ApplyUnifiedUpdate: method === "apply" ? vi.fn() : undefined, CheckForUpdate: vi.fn().mockResolvedValue({ Version: "legacy" }), ApplyVerifiedUpdate: vi.fn().mockResolvedValue(undefined) });
    const { result } = renderHook(() => useDesktopWorkspace(h.service));
    await waitFor(() => expect(result.current.model.update?.Version).toBe("legacy"));
    await act(async () => result.current.actions.applyUpdate());
    expect(h.service.ApplyVerifiedUpdate).toHaveBeenCalledExactlyOnceWith();
    expect(result.current.model.unifiedUpdate).toBeUndefined();
    const optional = method === "check" ? h.service.CheckUnifiedUpdates : h.service.ApplyUnifiedUpdate;
    expect(optional).not.toHaveBeenCalled();
  });
});

describe("remap lifecycle", () => {
  it.each([0, 1, 2] as const)("stages mode %s locally via the additive action", async (mode) => {
    const harness = serviceFor({ StageMacroAssignmentWithMode: vi.fn().mockResolvedValue(remap({ MacroPending: { ...macro, Mode: mode } })) });
    const { result } = renderHook(() => useDesktopWorkspace(harness.service));
    await waitFor(() => expect(result.current.model.ready).toBe(true));
    await act(async () => result.current.actions.stageMacroAssignmentWithMode("click", 6, 1, mode));
    expect(harness.service.StageMacroAssignmentWithMode).toHaveBeenCalledWith("click", 6, 1, mode);
    expect(result.current.model.remap?.MacroPending?.Mode).toBe(mode);
    expect(harness.service.ApplyRemap).not.toHaveBeenCalled();
  });
  it("requires confirmed reselection after selection rejection and permits staging on recovery", async () => {
    const harness = serviceFor({ SelectDevice: vi.fn().mockRejectedValueOnce(new Error("selection lost")).mockResolvedValueOnce({ Devices: [binding], Selected: binding, Error: { Code: "" } }) });
    const { result } = renderHook(() => useDesktopWorkspace(harness.service));
    await waitFor(() => expect(result.current.model.remap).toBeDefined());
    await act(async () => result.current.actions.selectDevice("alpha"));
    expect(result.current.model.ready).toBe(false);
    expect(result.current.model.inventory?.Selected).toBeNull();
    expect(result.current.model.remapError).toBe("selection lost");
    expect(result.current.model.notice).toMatch(/select.*again/i);
    await act(async () => result.current.actions.stageRemap(2, "off"));
    expect(harness.service.StageRemap).not.toHaveBeenCalled();
    await act(async () => result.current.actions.selectDevice("alpha"));
    expect(result.current.model.ready).toBe(true);
    await act(async () => result.current.actions.stageRemap(2, "off"));
    expect(result.current.model.remap?.Pending.Buttons[1].Action).toBe("off");
  });

  it("isolates all device events during selection and ignores an obsolete selection rejection", async () => {
    const bravo = { ...binding, ID: { ...binding.ID, Serial: "bravo" }, Path: "/dev/hidraw1" };
    const third = { ...binding, ID: { ...binding.ID, Serial: "third" }, Path: "/dev/hidraw2" };
    const first = deferred<Awaited<ReturnType<DesktopService["SelectDevice"]>>>();
    const second = deferred<Awaited<ReturnType<DesktopService["SelectDevice"]>>>();
    const harness = serviceFor({
      RefreshInventory: vi.fn().mockResolvedValue({ Devices: [binding, bravo], Selected: binding, Error: { Code: "" } }),
      SelectDevice: vi.fn().mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise),
    });
    const { result } = renderHook(() => useDesktopWorkspace(harness.service));
    await waitFor(() => expect(result.current.model.remap).toBeDefined());
    await act(async () => result.current.actions.selectDevice("bravo"));
    const reads = vi.mocked(harness.service.GetPollingSnapshot).mock.calls.length;
    for (const source of [binding, bravo, third]) {
      await act(async () => {
        harness.statusListeners[0]({ ...source, Battery: 1 });
        harness.configurationListeners[0]({ Binding: source, Snapshot: snapshot({ Battery: 2 }) });
        harness.pollingListeners[0]({ Binding: source, Snapshot: polling({ Applied: 125 }) });
        harness.remapListeners[0]({ Binding: source, Snapshot: remap({ Revision: 99 }) });
      });
      expect(result.current.model.snapshot?.Battery).toBe(84);
      expect(result.current.model.polling?.Applied).toBe(1000);
      expect(result.current.model.remap).toBeUndefined();
    }
    expect(harness.service.GetPollingSnapshot).toHaveBeenCalledTimes(reads);
    await act(async () => result.current.actions.selectDevice("alpha"));
    await act(async () => second.resolve({ Devices: [binding, bravo], Selected: binding, Error: { Code: "" } }));
    await act(async () => first.reject(new Error("obsolete selection")));
    expect(result.current.model.ready).toBe(true);
    expect(result.current.model.remapError).toBe("");
    await act(async () => {
      harness.statusListeners[0]({ ...binding, Battery: 91 });
      harness.pollingListeners[0]({ Binding: binding, Snapshot: polling({ Applied: 500 }) });
      harness.remapListeners[0]({ Binding: binding, Snapshot: remap({ Revision: 5 }) });
    });
    expect(result.current.model.snapshot?.Battery).toBe(91);
    expect(result.current.model.polling?.Applied).toBe(500);
    expect(result.current.model.remap?.Revision).toBe(5);
    await act(async () => harness.configurationListeners[0]({ Binding: binding, Snapshot: snapshot({ Battery: 92 }) }));
    expect(result.current.model.snapshot?.Battery).toBe(92);
  });

  it("distinguishes confirmed firmware with failed local saving from transport failure", async () => {
    const harness = serviceFor({ ApplyRemap: vi.fn()
      .mockResolvedValueOnce(remap({ Firmware: "success", Persistence: "failed", RetryAvailable: true, Error: { Code: "persistence_failed" } }))
      .mockResolvedValueOnce(remap({ Firmware: "failed", Error: { Code: "write_failed" } })) });
    const { result } = renderHook(() => useDesktopWorkspace(harness.service));
    await waitFor(() => expect(result.current.model.remap).toBeDefined());
    await act(async () => result.current.actions.applyRemap());
    expect(result.current.model.notice).toMatch(/transport confirmed.*local.*failed.*unverified/i);
    expect(result.current.model.remapError).toBe("persistence_failed");
    expect(harness.service.RetryRemapPersistence).not.toHaveBeenCalled();
    await act(async () => result.current.actions.applyRemap());
    expect(result.current.model.notice).toMatch(/transport not confirmed.*unknown.*partial/i);
    expect(result.current.model.remapError).toBe("write_failed");
  });
  it("stages ordinary edits in the backend without applying hardware", async () => {
    const harness = serviceFor();
    const { result } = renderHook(() => useDesktopWorkspace(harness.service));
    await waitFor(() => expect(result.current.model.remap).toBeDefined());
    await act(async () => result.current.actions.stageRemap(2, "off"));
    expect(harness.service.StageRemap).toHaveBeenCalledOnce();
    expect(result.current.model.remap?.Pending.Buttons[1].Action).toBe("off");
    expect(harness.service.ApplyRemap).not.toHaveBeenCalled();
    expect(result.current.model.notice).toMatch(/staged locally/i);
  });

  it("preserves ordinary fields under one replaceable overlay and clears only its target", async () => {
    let state = remap();
    const harness = serviceFor({
      StageMacroAssignment: vi.fn().mockImplementation(async (ID, Button, Repeat) => (state = { ...state, MacroPending: { ...macro, ID, Button, Repeat } })),
      StageRemap: vi.fn().mockImplementation(async (Pending) => (state = { ...state, Pending })),
      ClearButtonMacroAssignment: vi.fn().mockImplementation(async () => (state = { ...state, MacroPending: null })),
    });
    const { result } = renderHook(() => useDesktopWorkspace(harness.service));
    await waitFor(() => expect(result.current.model.remap).toBeDefined());
    await act(async () => result.current.actions.stageMacroAssignment("click", 6, 2));
    await act(async () => result.current.actions.stageMacroAssignment("click", 7, 3));
    expect(result.current.model.remap?.MacroPending?.Button).toBe(7);
    expect(result.current.model.remap?.Pending).toEqual(buttons);
    await act(async () => result.current.actions.stageRemap(2, "off"));
    expect(harness.service.ClearMacroAssignment).not.toHaveBeenCalled();
    await act(async () => result.current.actions.stageRemap(7, "off"));
    expect(harness.service.ClearButtonMacroAssignment).toHaveBeenCalledWith(7);
    expect(result.current.model.remap?.MacroPending).toBeNull();
  });

  it.each(["explicit clear", "ordinary action"])("%s preserves the other staged macro", async (operation) => {
    let state = remap();
    const harness = serviceFor({
      StageMacroAssignment: vi.fn().mockImplementation(async (ID, Button, Repeat) => {
        const draft = { ...(Button === 7 ? macro7 : macro), ID, Button, Repeat };
        state = { ...state, MacroPending: draft, MacroDrafts: { ...(state.MacroDrafts ?? {}), [Button]: draft }, Revision: state.Revision + 1 };
        return state;
      }),
      ClearButtonMacroAssignment: vi.fn().mockImplementation(async (button: number) => {
        const drafts = { ...(state.MacroDrafts ?? {}) };
        delete drafts[button];
        const remaining = Object.values(drafts);
        const macroPending = remaining.length > 0 ? remaining[0] : null;
        state = { ...state, MacroPending: macroPending, MacroDrafts: drafts, Revision: state.Revision + 1 };
        return state;
      }),
      StageRemap: vi.fn().mockImplementation(async (Pending) => (state = { ...state, Pending })),
      ClearMacroAssignment: vi.fn().mockImplementation(async () => (state = { ...state, MacroPending: null, MacroDrafts: {} })),
    });
    const { result } = renderHook(() => useDesktopWorkspace(harness.service));
    await waitFor(() => expect(result.current.model.remap).toBeDefined());

    // Stage a macro on button 6
    await act(async () => result.current.actions.stageMacroAssignment("click", 6, 2));
    expect(harness.service.StageMacroAssignment).toHaveBeenCalledWith("click", 6, 2);
    expect(result.current.model.remap?.MacroPending?.Button).toBe(6);

    // Stage a macro on button 7
    await act(async () => result.current.actions.stageMacroAssignment("click7", 7, 3));
    expect(harness.service.StageMacroAssignment).toHaveBeenCalledWith("click7", 7, 3);
    expect(result.current.model.remap?.MacroPending?.Button).toBe(7);
    expect(result.current.model.remap?.MacroDrafts?.[6]?.ID).toBe("click");
    expect(result.current.model.remap?.MacroDrafts?.[7]?.ID).toBe("click7");

    // Clear button 6's macro
    await act(async () => operation === "explicit clear"
      ? result.current.actions.clearButtonMacroAssignment!(6)
      : result.current.actions.stageRemap(6, "off"));
    expect(harness.service.ClearButtonMacroAssignment).toHaveBeenCalledWith(6);

    // Button 7's macro should still be staged
    expect(result.current.model.remap?.MacroDrafts?.[6]).toBeUndefined();
    expect(result.current.model.remap?.MacroDrafts?.[7]?.ID).toBe("click7");
    expect(result.current.model.remap?.MacroPending?.Button).toBe(7);

    // Clearing button 6 should NOT have called ClearMacroAssignment (whole-map fallback)
    expect(harness.service.ClearMacroAssignment).not.toHaveBeenCalled();
  });

  it.each(["explicit clear", "ordinary action"])("%s never falls back to clearing every macro when per-button clear is unavailable", async (operation) => {
    let state = remap({ MacroPending: macro7, MacroDrafts: { 6: macro, 7: macro7 } });
    const harness = serviceFor({
      GetRemapSnapshot: vi.fn().mockResolvedValue(state),
      ClearButtonMacroAssignment: undefined,
      StageRemap: vi.fn().mockImplementation(async (Pending) => (state = { ...state, Pending })),
      ClearMacroAssignment: vi.fn().mockImplementation(async () => (state = { ...state, MacroPending: null, MacroDrafts: {} })),
    });
    const { result } = renderHook(() => useDesktopWorkspace(harness.service));
    await waitFor(() => expect(result.current.model.remap).toBeDefined());
    await act(async () => operation === "explicit clear"
      ? result.current.actions.clearButtonMacroAssignment!(6)
      : result.current.actions.stageRemap(6, "off"));
    expect(harness.service.ClearMacroAssignment).not.toHaveBeenCalled();
    expect(result.current.model.remap?.MacroDrafts?.[7]).toEqual(macro7);
    expect(result.current.model.remapError).toBeTruthy();
  });

  it("blocks duplicate apply, retains drafts on rejection and recovers on discard", async () => {
    const applying = deferred<ReturnType<typeof remap>>();
    const harness = serviceFor({ ApplyRemap: vi.fn().mockReturnValue(applying.promise) });
    const { result } = renderHook(() => useDesktopWorkspace(harness.service));
    await waitFor(() => expect(result.current.model.remap).toBeDefined());
    await act(async () => result.current.actions.stageRemap(2, "off"));
    await act(async () => { result.current.actions.applyRemap(); result.current.actions.applyRemap(); });
    expect(harness.service.ApplyRemap).toHaveBeenCalledOnce();
    expect(result.current.model.remapBusy).toBe(true);
    await act(async () => applying.reject(new Error("lost transport")));
    expect(result.current.model.remapBusy).toBe(false);
    expect(result.current.model.remapError).toBe("lost transport");
    expect(result.current.model.remap?.Pending.Buttons[1].Action).toBe("off");
    expect(result.current.model.notice).toMatch(/unknown.*partial/i);
    await act(async () => result.current.actions.discardRemap());
    expect(harness.service.DiscardRemap).toHaveBeenCalledOnce();
    expect(result.current.model.remap?.Pending).toEqual(buttons);
    expect(result.current.model.remapError).toBe("");
  });

  it("retains confirmed ordinary staging when clearing its overlay rejects", async () => {
    const harness = serviceFor({
      GetRemapSnapshot: vi.fn().mockResolvedValue(remap({ MacroPending: macro })),
      StageRemap: vi.fn().mockImplementation(async (Pending) => remap({ Pending, MacroPending: macro })),
      ClearButtonMacroAssignment: vi.fn().mockRejectedValue(new Error("clear failed")),
    });
    const { result } = renderHook(() => useDesktopWorkspace(harness.service));
    await waitFor(() => expect(result.current.model.remap?.MacroPending).toEqual(macro));
    await act(async () => result.current.actions.stageRemap(6, "off"));
    expect(result.current.model.remap?.Pending.Buttons[5].Action).toBe("off");
    expect(result.current.model.remap?.MacroPending).toEqual(macro);
    expect(result.current.model.remapBusy).toBe(false);
    expect(result.current.model.remapError).toBe("clear failed");
  });

  it("rejects older refreshes after an event or a staged edit", async () => {
    const first = deferred<ReturnType<typeof remap>>();
    const second = deferred<ReturnType<typeof remap>>();
    const harness = serviceFor({ GetMacroAssignmentSnapshot: vi.fn().mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise) });
    const { result } = renderHook(() => useDesktopWorkspace(harness.service));
    await waitFor(() => expect(harness.service.GetMacroAssignmentSnapshot).toHaveBeenCalledTimes(2));
    await act(async () => harness.remapListeners[0]({ Binding: binding, Snapshot: remap({ Revision: 4 }) }));
    await act(async () => result.current.actions.stageRemap(2, "off"));
    await act(async () => { first.resolve(remap({ Revision: 1 })); second.reject(new Error("stale load")); });
    expect(result.current.model.remap?.Pending.Buttons[1].Action).toBe("off");
    expect(result.current.model.remapError).toBe("");
  });

  it.each(["resolve", "reject"] as const)("rejects stale %s through a device switch and ABA return", async (outcome) => {
    const staged = deferred<ReturnType<typeof remap>>();
    const bravo = { ...binding, ID: { ...binding.ID, Serial: "bravo" }, Path: "/dev/hidraw1" };
    const harness = serviceFor({
      RefreshInventory: vi.fn().mockResolvedValue({ Devices: [binding, bravo], Selected: binding, Error: { Code: "" } }),
      SelectDevice: vi.fn().mockImplementation(async (id) => ({ Devices: [binding, bravo], Selected: id.Serial === "alpha" ? binding : bravo, Error: { Code: "" } })),
      StageMacroAssignment: vi.fn().mockReturnValue(staged.promise),
    });
    const { result } = renderHook(() => useDesktopWorkspace(harness.service));
    await waitFor(() => expect(result.current.model.remap).toBeDefined());
    await act(async () => result.current.actions.stageMacroAssignment("click", 6, 2));
    await act(async () => result.current.actions.selectDevice("bravo"));
    await act(async () => result.current.actions.selectDevice("alpha"));
    await act(async () => outcome === "resolve" ? staged.resolve(remap({ MacroPending: macro })) : staged.reject(new Error("stale mutation")));
    expect(result.current.model.remap?.MacroPending).toBeUndefined();
    expect(result.current.model.remapError).toBe("");
    expect(result.current.model.remapBusy).toBe(false);
    expect(result.current.model.notice).toBe("");
  });

  it("ignores service replacement and unmounted completions without issuing a later clear", async () => {
    const stage = deferred<ReturnType<typeof remap>>();
    const old = serviceFor({ StageRemap: vi.fn().mockReturnValue(stage.promise), GetMacroAssignmentSnapshot: vi.fn().mockResolvedValue(remap({ MacroPending: macro })) });
    const next = serviceFor();
    const { result, rerender, unmount } = renderHook(({ service }) => useDesktopWorkspace(service), { initialProps: { service: old.service } });
    await waitFor(() => expect(result.current.model.remap?.MacroPending).toEqual(macro));
    await act(async () => result.current.actions.stageRemap(6, "off"));
    rerender({ service: next.service });
    await waitFor(() => expect(result.current.model.remap).toBeDefined());
    await act(async () => stage.resolve(remap({ MacroPending: macro })));
    expect(old.service.ClearMacroAssignment).not.toHaveBeenCalled();
    expect(result.current.model.remap?.MacroPending).toBeUndefined();
    const discard = deferred<ReturnType<typeof remap>>();
    vi.mocked(next.service.DiscardRemap).mockReturnValue(discard.promise);
    await act(async () => result.current.actions.discardRemap());
    unmount();
    await act(async () => discard.reject(new Error("after unmount")));
  });

  it("preserves a draft on stage/discard rejection and permits a later confirmed apply", async () => {
    const harness = serviceFor({
      StageMacroAssignment: vi.fn().mockRejectedValueOnce(new Error("invalid macro")).mockResolvedValueOnce(remap({ MacroPending: macro })),
      DiscardRemap: vi.fn().mockRejectedValueOnce(new Error("discard failed")),
      ApplyRemap: vi.fn().mockResolvedValue(remap({ Firmware: "success", MacroApplied: macro, MacroPending: macro, Persistence: "not_supported", MacroProgress: { Assignment: 2, Upload: 2 } })),
    });
    const { result } = renderHook(() => useDesktopWorkspace(harness.service));
    await waitFor(() => expect(result.current.model.remap).toBeDefined());
    await act(async () => result.current.actions.stageMacroAssignment("click", 6, 2));
    expect(result.current.model.remapError).toBe("invalid macro");
    expect(result.current.model.remapBusy).toBe(false);
    await act(async () => result.current.actions.stageMacroAssignment("click", 6, 2));
    await act(async () => result.current.actions.discardRemap());
    expect(result.current.model.remap?.MacroPending).toEqual(macro);
    expect(result.current.model.remapError).toBe("discard failed");
    await act(async () => result.current.actions.applyRemap());
    expect(harness.service.ApplyRemap).toHaveBeenCalledWith(buttons);
    expect(result.current.model.remap?.MacroApplied).toEqual(macro);
    expect(result.current.model.notice).toMatch(/transport confirmed.*unverified/i);
    expect(result.current.model.remapError).toBe("");
  });

  it("reports partial statuses without claiming success or playback", async () => {
    const harness = serviceFor({ ApplyRemap: vi.fn().mockResolvedValue(remap({ Firmware: "failed", MacroPending: macro, MacroProgress: { Assignment: 2, Upload: 1 }, Error: { Code: "upload_failed" } })) });
    const { result } = renderHook(() => useDesktopWorkspace(harness.service));
    await waitFor(() => expect(result.current.model.remap).toBeDefined());
    await act(async () => result.current.actions.applyRemap());
    expect(result.current.model.remap?.MacroProgress).toEqual({ Assignment: 2, Upload: 1 });
    expect(result.current.model.notice).toMatch(/unknown.*partial/i);
    expect(result.current.model.remapError).toBe("upload_failed");
  });
});

describe("useDesktopWorkspace", () => {
  it("loads the installed application version independently of available updates", async () => {
    const harness = serviceFor({
      GetApplicationVersion: vi.fn().mockResolvedValue("1.2.0-rc.5"),
      CheckForUpdate: vi.fn().mockResolvedValue({ Version: "1.2.0" }),
    });
    const { result } = renderHook(() => useDesktopWorkspace(harness.service));
    await waitFor(() => expect(result.current.model.applicationVersion).toBe("1.2.0-rc.5"));
    expect(result.current.model.update?.Version).toBe("1.2.0");
  });

  it("loads all workspace data, filters mismatched events, stages actions, and cleans up subscriptions", async () => {
    const harness = serviceFor();
    const { result, unmount } = renderHook(() => useDesktopWorkspace(harness.service));

    await waitFor(() => expect(result.current.model.snapshot?.Battery).toBe(84));
    expect(harness.service.RefreshStatus).toHaveBeenCalledTimes(2);
    expect(harness.service.GetPollingSnapshot).toHaveBeenCalledTimes(2);
    expect(harness.service.GetLightingSnapshot).toHaveBeenCalledTimes(2);
    expect(harness.service.RefreshInventory).toHaveBeenCalledOnce();

    await act(async () => harness.statusListeners[0]({ ...binding, Battery: 90 }));
    expect(result.current.model.snapshot?.Battery).toBe(90);
    await act(async () => harness.statusListeners[0]({ ...binding, ID: { ...binding.ID, Serial: "bravo" }, Battery: 10 }));
    expect(result.current.model.snapshot?.Battery).toBe(90);

    await act(async () => result.current.actions.stageDPI(0, 1600));
    expect(harness.service.StageDPI).toHaveBeenCalledWith(expect.objectContaining({ DPI: [1600, 1200] }));
    expect(harness.service.ApplyDPI).toHaveBeenCalledOnce();
    expect(result.current.model.notice).toBe("DPI applied.");

    unmount();
    expect(harness.unsubscribeStatus).toHaveBeenCalledOnce();
    expect(harness.unsubscribeConfiguration).toHaveBeenCalledOnce();
    expect(harness.unsubscribePolling).toHaveBeenCalledOnce();
		expect(harness.unsubscribeRemap).toHaveBeenCalledOnce();
  });

  it("keeps the verified update available after launch failure so approval can retry", async () => {
    const apply = vi.fn().mockRejectedValueOnce(new Error("launch failed")).mockResolvedValueOnce(undefined);
    const harness = serviceFor({
      CheckForUpdate: vi.fn().mockResolvedValue({ Version: "1.2.0" }),
      ApplyVerifiedUpdate: apply,
    });
    const { result } = renderHook(() => useDesktopWorkspace(harness.service));
    await waitFor(() => expect(result.current.model.update?.Version).toBe("1.2.0"));

    await act(async () => result.current.actions.applyUpdate());
    await waitFor(() => expect(result.current.model.updateError).toBe("launch failed"));
    expect(result.current.model.updateApplying).toBe(false);
    expect(result.current.model.update?.Version).toBe("1.2.0");

    await act(async () => result.current.actions.applyUpdate());
    await waitFor(() => expect(apply).toHaveBeenCalledTimes(2));
    expect(result.current.model.updateError).toBe("");
  });

  it("clears only the explicitly replaced DPI marker when staging remap", async () => {
    const pending: RemapConfig = { Buttons: [
      { Button: 1, Action: "left", PreservedDefault: "" }, { Button: 2, Action: "right", PreservedDefault: "" },
      { Button: 3, Action: "middle", PreservedDefault: "" }, { Button: 4, Action: "forward", PreservedDefault: "" },
      { Button: 5, Action: "backward", PreservedDefault: "" }, { Button: 6, Action: null, PreservedDefault: "DPI+" },
      { Button: 7, Action: null, PreservedDefault: "DPI-" },
    ] };
    const harness = serviceFor({ GetMacroAssignmentSnapshot: vi.fn().mockResolvedValue(remap({ Pending: pending, Applied: pending })) });
    const { result } = renderHook(() => useDesktopWorkspace(harness.service));
    await waitFor(() => expect(result.current.model.remap?.Pending.Buttons).toHaveLength(7));

    await act(async () => result.current.actions.stageRemap(6, "dpi_cycle"));

    expect(result.current.model.remap?.Pending.Buttons[5]).toEqual({ Button: 6, Action: "dpi_cycle", PreservedDefault: "" });
    expect(result.current.model.remap?.Pending.Buttons[6]).toEqual({ Button: 7, Action: null, PreservedDefault: "DPI-" });
  });

  it("hydrates both settings, applies them independently, and refreshes both after a configuration event", async () => {
    const configurationListeners: Array<(event: { Binding: typeof binding; Snapshot: ReturnType<typeof snapshot> }) => void> = [];
    const harness = serviceFor({ OnConfiguration: vi.fn().mockImplementation((callback) => { configurationListeners.push(callback); return vi.fn(); }) });
    const { result } = renderHook(() => useDesktopWorkspace(harness.service));
    await waitFor(() => expect(result.current.model.debounce?.Applied).toBe(8));

    await act(async () => result.current.actions.stageDebounce(12));
    await act(async () => result.current.actions.stageNormalSleep(15.5));
    expect(harness.service.ApplyDebounce).toHaveBeenCalledOnce();
    expect(harness.service.ApplyNormalSleep).toHaveBeenCalledOnce();
    await act(async () => configurationListeners[0]({ Binding: binding, Snapshot: snapshot() }));
    expect(harness.service.GetDebounceSnapshot).toHaveBeenCalled();
    expect(harness.service.GetNormalSleepSnapshot).toHaveBeenCalled();
  });

  it("reports exact polling and lighting notices for their distinct actions", async () => {
    const harness = serviceFor();
    const { result } = renderHook(() => useDesktopWorkspace(harness.service));
    await waitFor(() => expect(result.current.model.snapshot).toBeDefined());

    await act(async () => result.current.actions.stagePollingRate(500));
        await waitFor(() => expect(harness.service.ApplyPollingRate).toHaveBeenCalledOnce());
    expect(harness.service.ApplyPollingRate).toHaveBeenCalledOnce();
    expect(result.current.model.notice).toBe("Polling applied.");
    await act(async () => result.current.actions.stageLighting({ Mode: 0x10, TemplateID: "fixed-green" }));
    expect(harness.service.ApplyLighting).toHaveBeenCalledOnce();
  });

  it("keeps the blocking state until every overlapping automatic apply settles", async () => {
  const dpiStage = deferred<ReturnType<typeof snapshot>>();
  const dpiApply = deferred<ReturnType<typeof snapshot>>();
  const pollingApply = deferred<ReturnType<typeof polling>>();
  const harness = serviceFor({
    StageDPI: vi.fn().mockReturnValue(dpiStage.promise),
    ApplyDPI: vi.fn().mockReturnValue(dpiApply.promise),
    ApplyPollingRate: vi.fn().mockReturnValue(pollingApply.promise),
  });
  const { result } = renderHook(() => useDesktopWorkspace(harness.service));
  await waitFor(() => expect(result.current.model.snapshot).toBeDefined());

  await act(async () => {
    result.current.actions.stageDPI(0, 1600);
    result.current.actions.stagePollingRate(500);
    await Promise.resolve();
  });
  expect(result.current.model.automaticApplyBusy).toBe(true);

  await waitFor(() => expect(harness.service.ApplyPollingRate).toHaveBeenCalledOnce());

      await act(async () => pollingApply.resolve(polling({ Desired: 500, Applied: 500 })));
  expect(result.current.model.automaticApplyBusy).toBe(true);

  await act(async () => dpiStage.resolve(snapshot({ Pending: { ...dpi, DPI: [1600, 1200] } })));
  expect(harness.service.ApplyDPI).toHaveBeenCalledOnce();
  expect(result.current.model.automaticApplyBusy).toBe(true);

  await act(async () => dpiApply.resolve(snapshot({ Firmware: "success" })));
  expect(result.current.model.automaticApplyBusy).toBe(false);
});

it("applies DPI edits directly after staging", async () => {
    const harness = serviceFor();
    const { result } = renderHook(() => useDesktopWorkspace(harness.service));
    await waitFor(() => expect(result.current.model.snapshot).toBeDefined());

    await act(async () => result.current.actions.stageDPI(0, 1600));

    expect(result.current.model.notice).toBe("DPI applied.");
    expect(harness.service.ApplyDPI).toHaveBeenCalledOnce();
  });
});
