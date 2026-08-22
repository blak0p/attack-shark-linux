import { describe, expect, it, vi } from "vitest";

const bindings = vi.hoisted(() => ({
  GetSnapshot: vi.fn(),
  RefreshStatus: vi.fn(),
  StageDPI: vi.fn(),
  GetPollingSnapshot: vi.fn(),
  StagePollingRate: vi.fn(),
  RetryPollingPersistence: vi.fn(),
  ResetToFactory: vi.fn(),
  RetryPersistence: vi.fn(),
  RefreshInventory: vi.fn(),
  SelectDevice: vi.fn(),
}));

const runtime = vi.hoisted(() => ({ Events: { On: vi.fn().mockReturnValue(() => {}) } }));

vi.mock("../bindings/github.com/blak0p/attack-shark-linux/internal/desktop/service", () => bindings);
vi.mock("@wailsio/runtime", () => runtime);

import { desktopService } from "./wails-service";

describe("desktopService", () => {
  it("forwards each UI operation to its generated Wails binding", () => {
    const config = { DPI: [1600], ActiveStage: 0, StageMask: 1, LiftDistance: 1 };

    expect(desktopService.GetSnapshot).toBe(bindings.GetSnapshot);
    expect(desktopService.RefreshStatus).toBe(bindings.RefreshStatus);
    expect(desktopService.StageDPI).toBe(bindings.StageDPI);
    expect(desktopService.RetryPersistence).toBe(bindings.RetryPersistence);
    desktopService.StageDPI(config);
    expect(bindings.StageDPI).toHaveBeenCalledWith(config);
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
