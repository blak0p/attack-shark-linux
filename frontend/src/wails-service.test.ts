import { beforeEach, describe, expect, it, vi } from "vitest";

const bindings = vi.hoisted(() => ({
  ListMacros: vi.fn(), CreateMacro: vi.fn(), ReadMacro: vi.fn(), UpdateMacro: vi.fn(), DeleteMacro: vi.fn(),
  StageMacroAssignmentWithMode: vi.fn(), StageMacroAssignment: vi.fn(), StageRemap: vi.fn(), ClearButtonMacroAssignment: vi.fn(),
  ClearMacroAssignment: vi.fn(), DiscardRemap: vi.fn(), GetMacroAssignmentSnapshot: vi.fn(),
  GetApplicationVersion: vi.fn(),
  CheckForUpdate: vi.fn(),
  ApplyVerifiedUpdate: vi.fn(),
  GetSnapshot: vi.fn(),
  RefreshStatus: vi.fn(),
  StageDPI: vi.fn(),
  ApplyDPI: vi.fn(),
  GetPollingSnapshot: vi.fn(),
  GetDebounceSnapshot: vi.fn(),
  GetNormalSleepSnapshot: vi.fn(),
  StagePollingRate: vi.fn(),
  StageDebounce: vi.fn(),
  ApplyDebounce: vi.fn(),
  RetryDebouncePersistence: vi.fn(),
  StageNormalSleep: vi.fn(),
  ApplyNormalSleep: vi.fn(),
  RetryNormalSleepPersistence: vi.fn(),
  ApplyPollingRate: vi.fn(),
  GetLightingSnapshot: vi.fn(),
  StageLighting: vi.fn(),
	ApplyLighting: vi.fn(),
	GetRemapSnapshot: vi.fn(),
	ApplyRemap: vi.fn(),
	RetryRemapPersistence: vi.fn(),
  RetryPollingPersistence: vi.fn(),
  ResetToFactory: vi.fn(),
  RetryPersistence: vi.fn(),
  RefreshInventory: vi.fn(),
  SelectDevice: vi.fn(),
}));

const runtime = vi.hoisted(() => ({ Events: { On: vi.fn().mockReturnValue(() => {}) } }));

vi.mock("../../cmd/x6configurator/frontend/bindings/github.com/blak0p/attack-shark-linux/internal/desktop/service", () => bindings);
vi.mock("@wailsio/runtime", async (importOriginal) => ({
  ...await importOriginal<typeof import("@wailsio/runtime")>(), ...runtime,
}));

import { desktopService } from "./wails-service";
import { Macro as CanonicalMacro, EventType as CanonicalEventType } from "../bindings/github.com/blak0p/attack-shark-linux/internal/macros/models";
import { Macro as EmbeddedMacro, EventType as EmbeddedEventType } from "../../cmd/x6configurator/frontend/bindings/github.com/blak0p/attack-shark-linux/internal/macros/models";

it.each([CanonicalMacro, EmbeddedMacro])("binding models preserve keyboard keys and absent legacy mouse keys", (Model) => {
  const source = { id: "models", name: "Models", events: [
    { type: "keyboard", action: "down", delay_ms: 123, key_code: " UnknownPhysicalKey " },
    { type: "mouse_left", action: "up", delay_ms: 0 },
  ] };
  const result = Model.createFrom(JSON.stringify(source));
  expect(JSON.parse(JSON.stringify(result))).toEqual(source);
  expect(Object.hasOwn(result.events[1], "key_code")).toBe(false);
  expect(CanonicalEventType.Keyboard).toBe("keyboard");
  expect(EmbeddedEventType).toEqual(CanonicalEventType);
});

beforeEach(() => Object.values(bindings).forEach((binding) => binding.mockReset()));

describe("desktopService", () => {
  it("preserves opaque keyboard identity through all local CRUD boundaries with independent copies", async () => {
    const events = [{ type: "keyboard", action: "down", delay_ms: 123, key_code: " UnknownPhysicalKey " },
      { type: "keyboard", action: "up", delay_ms: 0, key_code: "KeyA" }];
    const macro = { id: "keyboard", name: "Keyboard", events };
    const before = structuredClone(macro);
    bindings.ListMacros.mockResolvedValueOnce([macro]);
    bindings.ReadMacro.mockResolvedValueOnce(macro);
    bindings.CreateMacro.mockResolvedValueOnce(macro);
    bindings.UpdateMacro.mockResolvedValueOnce(macro);
    const input = events as Parameters<typeof desktopService.CreateMacro>[1];
    const results = [(await desktopService.ListMacros())[0], await desktopService.ReadMacro(macro.id),
      await desktopService.CreateMacro(macro.name, input), await desktopService.UpdateMacro(macro.id, macro.name, input)];
    for (const result of results) {
      expect(result).toEqual(macro);
      expect(result.events[0]).not.toBe(events[0]);
      result.events[0].delay_ms = 9;
    }
    expect(bindings.CreateMacro).toHaveBeenLastCalledWith(macro.name, events);
    expect(bindings.UpdateMacro).toHaveBeenLastCalledWith(macro.id, macro.name, events);
    expect(bindings.CreateMacro.mock.calls.at(-1)?.[1][0]).not.toBe(events[0]);
    expect(macro).toEqual(before);
  });
  it.each([
    { type: "keyboard", key_code: "" }, { type: "keyboard", key_code: " \t" },
    { type: "keyboard" }, { type: "mouse_left", key_code: "KeyA" },
    { type: "mouse_right", key_code: " " },
  ])("rejects invalid local identity on read and before save: %j", async (identity) => {
    const events = [{ ...identity, action: "down", delay_ms: 0 }];
    bindings.ReadMacro.mockResolvedValueOnce({ id: "bad", name: "Bad", events });
    await expect(desktopService.ReadMacro("bad")).rejects.toThrow(/key_code/);
    const count = bindings.CreateMacro.mock.calls.length;
    await expect(desktopService.CreateMacro("Bad", events as Parameters<typeof desktopService.CreateMacro>[1])).rejects.toThrow(/key_code/);
    expect(bindings.CreateMacro.mock.calls).toHaveLength(count);
  });
  it.each([0, 1, 2] as const)("forwards numeric mode %s and normalizes legacy mode", async (mode) => {
    const draft = { ID: "click", Name: "Click", Button: 6, Repeat: 1, Mode: mode, Events: [] };
    bindings.StageMacroAssignmentWithMode.mockResolvedValueOnce({ MacroPending: draft, MacroDrafts: { 6: draft } });
    const result = await desktopService.StageMacroAssignmentWithMode("click", 6, 1, mode);
    expect(bindings.StageMacroAssignmentWithMode).toHaveBeenCalledWith("click", 6, 1, mode);
    expect(result.MacroDrafts?.[6]?.Mode).toBe(mode);
    bindings.GetMacroAssignmentSnapshot.mockResolvedValueOnce({ MacroPending: { ...draft, Mode: undefined } });
    expect((await desktopService.GetMacroAssignmentSnapshot()).MacroPending?.Mode).toBe(0);
  });
  it("forwards assignment drafts and preserves nullable overlay results and errors", async () => {
    const config = { Buttons: [] };
    const snapshot = { Pending: config, Applied: config, MacroPending: null, MacroApplied: null,
      MacroProgress: { Assignment: 0, Upload: 0 },
      Firmware: "failed", Persistence: "not_supported", Error: { Code: "apply_failed" } };
    bindings.StageMacroAssignment.mockResolvedValue(snapshot);
    bindings.StageRemap.mockResolvedValue(snapshot);
    bindings.ClearButtonMacroAssignment.mockResolvedValue(snapshot);
    bindings.ClearMacroAssignment.mockResolvedValue(snapshot);
    bindings.DiscardRemap.mockResolvedValue(snapshot);
    bindings.GetMacroAssignmentSnapshot.mockResolvedValue(snapshot);
    expect(await desktopService.StageMacroAssignment("stable", 6, 255)).toEqual(snapshot);
    expect(bindings.StageMacroAssignment).toHaveBeenCalledWith("stable", 6, 255);
    expect(await desktopService.StageRemap(config)).toEqual(snapshot);
    expect(bindings.StageRemap).toHaveBeenCalledWith(config);
    for (const name of ["ClearMacroAssignment", "DiscardRemap", "GetMacroAssignmentSnapshot"] as const) {
      expect(await desktopService[name]()).toEqual(snapshot);
      expect(bindings[name]).toHaveBeenCalledWith();
    }
    expect(await desktopService.ClearButtonMacroAssignment(6)).toEqual(snapshot);
    expect(bindings.ClearButtonMacroAssignment).toHaveBeenCalledWith(6);
    const failure = new Error("native service unavailable");
    bindings.StageMacroAssignment.mockRejectedValueOnce(failure);
    await expect(desktopService.StageMacroAssignment("stable", 6, 1)).rejects.toBe(failure);
  });
  it("preserves frozen assignment events and partial transport evidence on apply", async () => {
    const config = { Buttons: [] };
    const draft = { ID: "stable", Name: "Saved click", Button: 7, Repeat: 1, Mode: 0,
      Events: [{ type: "mouse_right", action: "down", delay_ms: 0 },
        { type: "mouse_right", action: "up", delay_ms: 0 }] };
    const snapshot = { Pending: config, Applied: config, MacroPending: draft, MacroApplied: null,
      MacroProgress: { Assignment: 2, Upload: 1 }, Firmware: "failed",
      Persistence: "not_supported", Error: { Code: "apply_failed" } };
    bindings.ApplyRemap.mockResolvedValueOnce(snapshot);
    expect(await desktopService.ApplyRemap(config)).toEqual(snapshot);
    expect(bindings.ApplyRemap).toHaveBeenCalledWith(config);
    const failure = new Error("transport unavailable");
    bindings.ApplyRemap.mockRejectedValueOnce(failure);
    await expect(desktopService.ApplyRemap(config)).rejects.toBe(failure);
  });
  it("converts every pending and applied map value without dropping events or other buttons", async () => {
    const events = [{ type: "mouse_right", action: "down", delay_ms: 0 },
      { type: "mouse_right", action: "up", delay_ms: 0 }];
    const six = { ID: "six", Name: "Six", Button: 6, Repeat: 2, Mode: 0, Events: events };
    const seven = { ID: "seven", Name: "Seven", Button: 7, Repeat: 255, Mode: 0, Events: events };
    const snapshot = { MacroPending: seven, MacroApplied: six,
      MacroDrafts: { 6: six, 7: seven }, MacroAppliedDrafts: { 6: six } };
    bindings.GetMacroAssignmentSnapshot.mockResolvedValueOnce(snapshot);
    const result = await desktopService.GetMacroAssignmentSnapshot();
    expect(result).toEqual(snapshot);
    expect(result.MacroDrafts?.[6]).not.toBe(six);
    expect(result.MacroDrafts?.[7]?.Events).not.toBe(events);
    expect(result.MacroAppliedDrafts?.[6]).not.toBe(six);
  });

  it("maps local macro CRUD to generated APIs with lowercase JSON fields and no device apply", async () => {
    const events = [{ type: "mouse_left" as const, action: "down" as const, delay_ms: 15 }];
    const macro = { id: "stable", name: "Clicks", events };
    bindings.ListMacros.mockResolvedValue([macro]);
    bindings.ReadMacro.mockResolvedValue(macro);
    bindings.CreateMacro.mockResolvedValue(macro);
    bindings.UpdateMacro.mockResolvedValue(macro);
    expect(await desktopService.ListMacros()).toEqual([macro]);
    expect(await desktopService.ReadMacro("stable")).toEqual(macro);
    expect(await desktopService.CreateMacro("Clicks", events)).toEqual(macro);
    expect(await desktopService.UpdateMacro("stable", "Clicks", events)).toEqual(macro);
    await desktopService.DeleteMacro("stable");
    expect(bindings.ReadMacro).toHaveBeenCalledWith("stable");
    expect(bindings.CreateMacro).toHaveBeenCalledWith("Clicks", events);
    expect(bindings.UpdateMacro).toHaveBeenCalledWith("stable", "Clicks", events);
    expect(bindings.DeleteMacro).toHaveBeenCalledWith("stable");
    expect(bindings.ApplyRemap).not.toHaveBeenCalled();
    expect(bindings.ApplyDPI).not.toHaveBeenCalled();
  });
  it.each(["mouse_middle", "mouse_back", "mouse_forward"] as const)(
    "preserves %s through local CRUD and assignment draft snapshots", async (type) => {
      const events = [
        { type, action: "down" as const, delay_ms: 15 },
        { type, action: "up" as const, delay_ms: 25 },
      ];
      const macro = { id: "stable", name: "Clicks", events };
      bindings.ListMacros.mockResolvedValueOnce([macro]);
      bindings.CreateMacro.mockResolvedValueOnce(macro);
      bindings.UpdateMacro.mockResolvedValueOnce(macro);
      bindings.ReadMacro.mockResolvedValueOnce(macro);
      expect(await desktopService.ListMacros()).toEqual([macro]);
      expect(await desktopService.CreateMacro("Clicks", events)).toEqual(macro);
      expect(await desktopService.UpdateMacro("stable", "Clicks", events)).toEqual(macro);
      expect(await desktopService.ReadMacro("stable")).toEqual(macro);
      expect(bindings.CreateMacro).toHaveBeenLastCalledWith("Clicks", events);
      expect(bindings.UpdateMacro).toHaveBeenLastCalledWith("stable", "Clicks", events);
      const draft = { ID: "stable", Name: "Clicks", Button: 7, Repeat: 1, Mode: 0, Events: events };
      const snapshot = { Pending: { Buttons: [] }, Applied: { Buttons: [] },
        MacroPending: draft, MacroApplied: draft };
      bindings.GetMacroAssignmentSnapshot.mockResolvedValueOnce(snapshot);
      expect(await desktopService.GetMacroAssignmentSnapshot()).toEqual(snapshot);
    },
  );

  it("rejects unknown event types in local CRUD responses and both draft states", async () => {
    const events = [{ type: "mouse_unknown", action: "down", delay_ms: 0 }];
    const macro = { id: "stable", name: "Unknown", events };
    bindings.ListMacros.mockResolvedValueOnce([macro]);
    bindings.CreateMacro.mockResolvedValueOnce(macro);
    bindings.UpdateMacro.mockResolvedValueOnce(macro);
    bindings.ReadMacro.mockResolvedValueOnce(macro);
    await expect(desktopService.ListMacros()).rejects.toThrow("Unsupported local macro event");
    await expect(desktopService.CreateMacro("Unknown", [])).rejects.toThrow("Unsupported local macro event");
    await expect(desktopService.UpdateMacro("stable", "Unknown", [])).rejects.toThrow("Unsupported local macro event");
    await expect(desktopService.ReadMacro("stable")).rejects.toThrow("Unsupported local macro event");
    const draft = { ID: "stable", Name: "Unknown", Button: 7, Repeat: 1, Events: events };
    for (const state of ["MacroPending", "MacroApplied"] as const) {
      bindings.GetMacroAssignmentSnapshot.mockResolvedValueOnce({
        Pending: { Buttons: [] }, Applied: { Buttons: [] },
        MacroPending: null, MacroApplied: null, [state]: draft,
      });
      await expect(desktopService.GetMacroAssignmentSnapshot()).rejects.toThrow("Unsupported local macro event");
    }
  });

  it("forwards each UI operation to its generated Wails binding", () => {
    const config = { DPI: [1600], ActiveStage: 0, StageMask: 1, LiftDistance: 1 };

    expect(desktopService.GetApplicationVersion).toBe(bindings.GetApplicationVersion);
    expect(desktopService.CheckForUpdate).toBe(bindings.CheckForUpdate);
    expect(desktopService.ApplyVerifiedUpdate).toBe(bindings.ApplyVerifiedUpdate);
    expect(desktopService.GetSnapshot).toBe(bindings.GetSnapshot);
    expect(desktopService.RefreshStatus).toBe(bindings.RefreshStatus);
    expect(desktopService.StageDPI).toBe(bindings.StageDPI);
    expect(desktopService.RetryPersistence).toBe(bindings.RetryPersistence);
    desktopService.StageDPI(config);
    expect(bindings.StageDPI).toHaveBeenCalledWith(config);
  });

  it("forwards key response time and normal sleep operations to generated Wails bindings", () => {
    expect(desktopService.GetDebounceSnapshot).toBe(bindings.GetDebounceSnapshot);
    expect(desktopService.GetNormalSleepSnapshot).toBe(bindings.GetNormalSleepSnapshot);
    desktopService.StageDebounce(12);
    desktopService.ApplyDebounce();
    desktopService.RetryDebouncePersistence();
    desktopService.StageNormalSleep(30.5);
    desktopService.ApplyNormalSleep();
    desktopService.RetryNormalSleepPersistence();
    expect(bindings.StageDebounce).toHaveBeenCalledWith(12);
    expect(bindings.ApplyDebounce).toHaveBeenCalledOnce();
    expect(bindings.RetryDebouncePersistence).toHaveBeenCalledOnce();
    expect(bindings.StageNormalSleep).toHaveBeenCalledWith(30.5);
    expect(bindings.ApplyNormalSleep).toHaveBeenCalledOnce();
    expect(bindings.RetryNormalSleepPersistence).toHaveBeenCalledOnce();
  });

  it("forwards polling selections and factory reset to generated Wails bindings", () => {
    expect(desktopService.GetPollingSnapshot).toBe(bindings.GetPollingSnapshot);
    expect(desktopService.StagePollingRate).toBe(bindings.StagePollingRate);
    expect(desktopService.RetryPollingPersistence).toBe(bindings.RetryPollingPersistence);
    expect(desktopService.ResetToFactory).toBe(bindings.ResetToFactory);

    desktopService.StagePollingRate(500);
    desktopService.RetryPollingPersistence();
    desktopService.ResetToFactory();

    expect(bindings.StagePollingRate).toHaveBeenCalledWith(500);
    expect(bindings.RetryPollingPersistence).toHaveBeenCalledOnce();
    expect(bindings.ResetToFactory).toHaveBeenCalledOnce();
  });

  it("forwards explicit DPI and polling confirmation to generated Wails bindings", () => {
    expect((desktopService as unknown as { ApplyDPI: unknown }).ApplyDPI).toBe(bindings.ApplyDPI);
    expect((desktopService as unknown as { ApplyPollingRate: unknown }).ApplyPollingRate).toBe(bindings.ApplyPollingRate);

    (desktopService as unknown as { ApplyDPI(): void }).ApplyDPI();
    (desktopService as unknown as { ApplyPollingRate(): void }).ApplyPollingRate();

    expect(bindings.ApplyDPI).toHaveBeenCalledOnce();
    expect(bindings.ApplyPollingRate).toHaveBeenCalledOnce();
  });

	it("forwards lighting reads, staging, and explicit application to generated Wails bindings", () => {
    const selection = { Mode: 0x20, TemplateID: "breathing-ff7f00" };

    expect(desktopService.GetLightingSnapshot).toBe(bindings.GetLightingSnapshot);
    expect(desktopService.StageLighting).toBe(bindings.StageLighting);
    expect(desktopService.ApplyLighting).toBe(bindings.ApplyLighting);
    desktopService.StageLighting(selection);
    desktopService.ApplyLighting();

    expect(bindings.StageLighting).toHaveBeenCalledWith(selection);
    expect(bindings.ApplyLighting).toHaveBeenCalledOnce();
	});

  it("forwards remap reads and explicit application with converted results", async () => {
    const config = { Buttons: [] };
    const snapshot = { Pending: config, Applied: config, MacroPending: null, MacroApplied: null };
    bindings.GetRemapSnapshot.mockResolvedValueOnce(snapshot);
    bindings.ApplyRemap.mockResolvedValueOnce(snapshot);
    expect(await desktopService.GetRemapSnapshot()).toEqual(snapshot);
    expect(bindings.GetRemapSnapshot).toHaveBeenCalledWith();
    expect(await desktopService.ApplyRemap(config)).toEqual(snapshot);
    expect(bindings.ApplyRemap).toHaveBeenCalledWith(config);
  });

  it("maps preserved defaults to Go zero actions without mutating the local draft", async () => {
    const config = { Buttons: [
      { Button: 1, Action: "left" as const, PreservedDefault: "" as const },
      { Button: 6, Action: null, PreservedDefault: "DPI+" as const },
      { Button: 7, Action: null, PreservedDefault: "DPI-" as const },
    ] };
    const snapshot = { Pending: config, Applied: config };
    bindings.StageRemap.mockResolvedValueOnce(snapshot);
    bindings.ApplyRemap.mockResolvedValueOnce(snapshot);
    for (const method of ["StageRemap", "ApplyRemap"] as const) {
      expect(await desktopService[method](config)).toMatchObject(snapshot);
      expect(bindings[method]).toHaveBeenLastCalledWith({ Buttons: [
        config.Buttons[0], { ...config.Buttons[1], Action: "" }, { ...config.Buttons[2], Action: "" },
      ] });
    }
    expect(config.Buttons[1].Action).toBeNull();
    const nativeConfig = JSON.parse('{"Buttons":[{"Button":6,"Action":"","PreservedDefault":"DPI+"}]}');
    bindings.StageRemap.mockResolvedValueOnce(snapshot);
    await desktopService.StageRemap(nativeConfig);
    expect(bindings.StageRemap).toHaveBeenLastCalledWith(nativeConfig);
    expect(nativeConfig.Buttons[0].Action).toBe("");
    for (const method of ["StageRemap", "ApplyRemap"] as const) {
      const calls = bindings[method].mock.calls.length;
      await expect(desktopService[method]({ Buttons: [{ Button: 1, Action: null, PreservedDefault: "" }] }))
        .rejects.toThrow("Unsupported preserved remap default");
      await expect(desktopService[method](JSON.parse('{"Buttons":[{"Button":1,"Action":"macro","PreservedDefault":""}]}')))
        .rejects.toThrow("Unsupported remap action");
      expect(bindings[method].mock.calls).toHaveLength(calls);
    }
  });

  it("subscribes to device-scoped status events and returns the Wails unsubscribe function", () => {
    const callback = vi.fn();
    const unsubscribe = desktopService.OnStatusEvent(callback);

    expect(runtime.Events.On).toHaveBeenCalledWith("mouse:status", expect.any(Function));
    const handler = runtime.Events.On.mock.calls[0][1];
    handler({ data: { Connection: "dongle", Battery: 90 } });
    expect(callback).toHaveBeenCalledWith({ Connection: "dongle", Battery: 90 });

    expect(unsubscribe).toBe(runtime.Events.On.mock.results[0].value);
  });

  it("subscribes to emitted configuration snapshots", () => {
    const callback = vi.fn();
    desktopService.OnConfiguration(callback);

    expect(runtime.Events.On).toHaveBeenCalledWith("mouse:configuration", expect.any(Function));
    runtime.Events.On.mock.calls.at(-1)[1]({ data: { Snapshot: { Firmware: "success" } } });
    expect(callback).toHaveBeenCalledWith({ Snapshot: { Firmware: "success" } });
  });

  it("subscribes to polling completion snapshots", () => {
    const callback = vi.fn();
    desktopService.OnPollingConfiguration(callback);

    expect(runtime.Events.On).toHaveBeenCalledWith("mouse:polling-configuration", expect.any(Function));
    runtime.Events.On.mock.calls.at(-1)[1]({ data: { Snapshot: { Firmware: "success", Persistence: "failed" } } });
    expect(callback).toHaveBeenCalledWith({ Snapshot: { Firmware: "success", Persistence: "failed" } });
  });

	it("subscribes to explicit remap completion snapshots", () => {
		const callback = vi.fn();
		desktopService.OnRemapConfiguration(callback);
		expect(runtime.Events.On).toHaveBeenCalledWith("mouse:remap-configuration", expect.any(Function));
	});

  it("delegates generated inventory bindings and subscribes to device-scoped status events", () => {
    const callback = vi.fn();

    expect((desktopService as unknown as { RefreshInventory: unknown }).RefreshInventory).toBe(bindings.RefreshInventory);
    expect((desktopService as unknown as { SelectDevice: unknown }).SelectDevice).toBe(bindings.SelectDevice);
    (desktopService as unknown as { SelectDevice(id: { VendorID: number; ProductID: number; Serial: string }): void }).SelectDevice({ VendorID: 0x1D57, ProductID: 0xFA60, Serial: "bravo" });
    expect(bindings.SelectDevice).toHaveBeenCalledWith({ VendorID: 0x1D57, ProductID: 0xFA60, Serial: "bravo" });

    desktopService.OnStatusEvent(callback);
    expect(runtime.Events.On).toHaveBeenLastCalledWith("mouse:status", expect.any(Function));
  });
});
