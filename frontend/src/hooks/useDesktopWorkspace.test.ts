import "@testing-library/jest-dom/vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useDesktopWorkspace } from "./useDesktopWorkspace";
import type { DesktopService, PollingConfigurationEvent, StatusEvent } from "../desktop-contract";

afterEach(() => vi.restoreAllMocks());

const binding = { ID: { VendorID: 0x1d57, ProductID: 0xfa60, Serial: "alpha" }, Path: "/dev/hidraw0", InventoryRevision: 1 };
const dpi = { DPI: [800, 1200], ActiveStage: 1, StageMask: 3, LiftDistance: 1 };
const snapshot = (overrides = {}) => ({ Connection: "dongle", Battery: 84, Applied: dpi, Pending: dpi, Factory: dpi, Revision: 0, Error: { Code: "" }, ...overrides });
const polling = (overrides = {}) => ({ Desired: 1000, Applied: 1000, Factory: 1000, Revision: 0, ...overrides });
const lighting = (overrides = {}) => ({ Pending: { Mode: 0x10 as const, TemplateID: "fixed-green" }, Applied: null, Effects: [], Revision: 0, Firmware: "", Error: { Code: "" }, ...overrides });

function serviceFor(overrides: Partial<DesktopService> = {}) {
  const statusListeners: Array<(event: StatusEvent) => void> = [];
  const configurationListeners: Array<(event: { Binding: typeof binding; Snapshot: ReturnType<typeof snapshot> }) => void> = [];
  const pollingListeners: Array<(event: PollingConfigurationEvent) => void> = [];
  const unsubscribeStatus = vi.fn();
  const unsubscribeConfiguration = vi.fn();
  const unsubscribePolling = vi.fn();
  const service: DesktopService = {
    GetSnapshot: vi.fn().mockResolvedValue(snapshot()),
    GetPollingSnapshot: vi.fn().mockResolvedValue(polling()),
    GetLightingSnapshot: vi.fn().mockResolvedValue(lighting()),
    RefreshStatus: vi.fn().mockResolvedValue(snapshot()),
    RefreshInventory: vi.fn().mockResolvedValue({ Devices: [binding], Selected: binding, Error: { Code: "" } }),
    SelectDevice: vi.fn().mockResolvedValue({ Devices: [binding], Selected: binding, Error: { Code: "" } }),
    StageDPI: vi.fn().mockImplementation(async (next) => snapshot({ Pending: next })),
    StagePollingRate: vi.fn().mockImplementation(async (rate) => polling({ Desired: rate })),
    StageLighting: vi.fn().mockResolvedValue(lighting()),
    ApplyLighting: vi.fn().mockResolvedValue(lighting()),
    ResetToFactory: vi.fn().mockResolvedValue(snapshot()),
    RetryPersistence: vi.fn().mockResolvedValue(snapshot()),
    RetryPollingPersistence: vi.fn().mockResolvedValue(polling()),
    OnStatusEvent: vi.fn().mockImplementation((callback) => { statusListeners.push(callback); return unsubscribeStatus; }),
    OnConfiguration: vi.fn().mockImplementation((callback) => { configurationListeners.push(callback); return unsubscribeConfiguration; }),
    OnPollingConfiguration: vi.fn().mockImplementation((callback) => { pollingListeners.push(callback); return unsubscribePolling; }),
    ...overrides,
  };
  return { service, statusListeners, configurationListeners, pollingListeners, unsubscribeStatus, unsubscribeConfiguration, unsubscribePolling };
}

describe("useDesktopWorkspace", () => {
  it("loads all workspace data, filters mismatched events, stages actions, and cleans up subscriptions", async () => {
    const harness = serviceFor();
    const { result, unmount } = renderHook(() => useDesktopWorkspace(harness.service));

    await waitFor(() => expect(result.current.model.snapshot?.Battery).toBe(84));
    expect(harness.service.RefreshStatus).toHaveBeenCalledOnce();
    expect(harness.service.GetPollingSnapshot).toHaveBeenCalledOnce();
    expect(harness.service.GetLightingSnapshot).toHaveBeenCalledOnce();
    expect(harness.service.RefreshInventory).toHaveBeenCalledOnce();

    await act(async () => harness.statusListeners[0]({ ...binding, Battery: 90 }));
    expect(result.current.model.snapshot?.Battery).toBe(90);
    await act(async () => harness.statusListeners[0]({ ...binding, ID: { ...binding.ID, Serial: "bravo" }, Battery: 10 }));
    expect(result.current.model.snapshot?.Battery).toBe(90);

    await act(async () => result.current.actions.stageDPI(0, 1600));
    expect(harness.service.StageDPI).toHaveBeenCalledWith(expect.objectContaining({ DPI: [1600, 1200] }));
    expect(result.current.model.notice).toBe("Synchronization queued. It will apply after one second of inactivity.");

    unmount();
    expect(harness.unsubscribeStatus).toHaveBeenCalledOnce();
    expect(harness.unsubscribeConfiguration).toHaveBeenCalledOnce();
    expect(harness.unsubscribePolling).toHaveBeenCalledOnce();
  });

  it("reports exact polling and lighting notices for their distinct actions", async () => {
    const harness = serviceFor();
    const { result } = renderHook(() => useDesktopWorkspace(harness.service));
    await waitFor(() => expect(result.current.model.snapshot).toBeDefined());

    await act(async () => result.current.actions.stagePollingRate(500));
    expect(result.current.model.notice).toBe("Polling synchronization queued. It will apply after one second of inactivity.");
    await act(async () => result.current.actions.stageLighting({ Mode: 0x10, TemplateID: "fixed-green" }));
    expect(result.current.model.notice).toBe("Lighting selection staged. Apply lighting to send it to the device.");
  });
});
