import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import type { DesktopService } from "../desktop-contract";
import { useDesktopWorkspace } from "./useDesktopWorkspace";

afterEach(cleanup);

it.each(["action", "event", "refresh-event"])("keeps reset authoritative across same-binding remap %s", async (interference) => {
  const service = fixture();
  const binding = { ID: { Serial: "alpha", VendorID: 1, ProductID: 2 }, InventoryRevision: 0 };
  service.RefreshInventory.mockResolvedValue({ Selected: binding });
  const previous = { Revision: 1, Pending: { Buttons: [] }, MacroPending: { ID: "old" } };
  service.GetRemapSnapshot.mockResolvedValue(previous);
  let onRemap!: (event: unknown) => void;
  service.OnRemapConfiguration.mockImplementation((callback) => { onRemap = callback; return () => {}; });
  const reset = deferred<unknown>();
  service.ResetToFactory.mockReturnValue(reset.promise);
  const mutation = deferred<unknown>();
  const withMutation = { ...service, DiscardRemap: vi.fn().mockReturnValue(mutation.promise) };
  const { result } = renderHook(() => useDesktopWorkspace(withMutation as unknown as DesktopService));
  await waitFor(() => expect(service.GetRemapSnapshot).toHaveBeenCalledTimes(2));
  if (interference === "event") {
    act(() => result.current.actions.discardRemap());
    await waitFor(() => expect(withMutation.DiscardRemap).toHaveBeenCalledOnce());
  }
  const fresh = { Revision: 2, MacroPending: null };
  const authoritative = { marker: "reset" };
  const reads = [service.GetSnapshot, service.GetPollingSnapshot, service.GetDebounceSnapshot, service.GetLightingSnapshot, service.GetNormalSleepSnapshot];
  reads.forEach((read) => { read.mockClear(); read.mockResolvedValue(authoritative); });
  service.GetRemapSnapshot.mockClear().mockResolvedValue(fresh);
  act(() => { result.current.actions.requestReset(); result.current.actions.confirmReset(); });
  if (interference === "action") {
    await act(async () => { result.current.actions.discardRemap(); });
    act(() => result.current.actions.confirmReset());
    expect(withMutation.DiscardRemap).not.toHaveBeenCalled();
    expect(service.ResetToFactory).toHaveBeenCalledOnce();
  }
  if (interference === "event") {
    await act(async () => {
      mutation.resolve(previous);
      onRemap({ Binding: binding, Snapshot: previous });
    });
    expect(result.current.model.remapBusy).toBe(true);
    expect(result.current.model.remap).toEqual(previous);
  }
  const delayed = deferred<unknown>();
  if (interference === "refresh-event") reads.forEach((read) => read.mockReturnValue(delayed.promise));
  await act(async () => reset.resolve({ Cleanup: { State: "success" }, Error: { Code: "" } }));
  if (interference === "refresh-event") {
    await act(async () => {
      onRemap({ Binding: binding, Snapshot: previous });
      delayed.resolve(authoritative);
    });
  }
  await waitFor(() => expect(result.current.model.remap).toEqual(fresh));
  reads.forEach((read) => expect(read).toHaveBeenCalledOnce());
  const model = result.current.model;
  [model.snapshot, model.polling, model.debounce, model.lighting, model.normalSleep].forEach((snapshot) => expect(snapshot).toEqual(authoritative));
  expect(model.resetConfirmation).toBe(false);
  expect(model.notice).toBe("Factory reset completed.");
  expect(model.remapBusy).toBe(false);
});
const deferred = <T,>() => {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => { resolve = done; });
  return { promise, resolve };
};
function fixture() {
  const binding = { ID: { Serial: "alpha" } };
  const service = {
    RefreshInventory: vi.fn().mockResolvedValue({ Selected: binding }),
    RefreshStatus: vi.fn().mockResolvedValue({}),
    GetSnapshot: vi.fn().mockResolvedValue({}),
    GetPollingSnapshot: vi.fn().mockResolvedValue({}),
    GetDebounceSnapshot: vi.fn().mockResolvedValue({}),
    GetLightingSnapshot: vi.fn().mockResolvedValue({}),
    GetNormalSleepSnapshot: vi.fn().mockResolvedValue({}),
    GetRemapSnapshot: vi.fn().mockResolvedValue({}),
    OnStatusEvent: vi.fn().mockReturnValue(() => {}),
    OnConfiguration: vi.fn().mockReturnValue(() => {}),
    OnPollingConfiguration: vi.fn().mockReturnValue(() => {}),
    OnRemapConfiguration: vi.fn().mockReturnValue(() => {}),
    ResetToFactory: vi.fn().mockResolvedValue({ Cleanup: { State: "success" }, Error: { Code: "" } }),
  };
  return service;
}
it("invalidates pre-reset remap reads and refreshes every supported snapshot", async () => {
  const service = fixture();
  const stale = deferred<unknown>();
  service.GetRemapSnapshot.mockReturnValueOnce(stale.promise).mockReturnValueOnce(stale.promise);
  const { result } = renderHook(() => useDesktopWorkspace(service as unknown as DesktopService));
  await waitFor(() => expect(service.GetRemapSnapshot).toHaveBeenCalledTimes(2));
  const fresh = { Revision: 2, MacroPending: null };
  service.GetRemapSnapshot.mockResolvedValue(fresh);
  const reads = [service.GetDebounceSnapshot, service.GetLightingSnapshot, service.GetNormalSleepSnapshot];
  reads.forEach((read) => read.mockClear());
  act(() => result.current.actions.confirmReset());
  await waitFor(() => expect(result.current.model.remap).toEqual(fresh));
  reads.forEach((read) => expect(read).toHaveBeenCalledOnce());
  await act(async () => stale.resolve({ Revision: 1, MacroPending: { ID: "stale" } }));
  expect(result.current.model.remap).toEqual(fresh);
});
it("retains snapshots and reports a rejected reset without fresh reads", async () => {
  const service = fixture();
  service.ResetToFactory.mockRejectedValue(new Error("transport unavailable"));
  const { result } = renderHook(() => useDesktopWorkspace(service as unknown as DesktopService));
  await waitFor(() => expect(service.GetRemapSnapshot).toHaveBeenCalledTimes(2));
  const previous = result.current.model.remap;
  service.GetRemapSnapshot.mockClear();
  act(() => result.current.actions.confirmReset());
  await waitFor(() => expect(result.current.model.notice).toBe("Factory reset failed: transport unavailable."));
  expect(service.GetRemapSnapshot).not.toHaveBeenCalled();
  expect(service.GetSnapshot).not.toHaveBeenCalled();
  expect(result.current.model.remap).toEqual(previous);
});
it("ignores a reset completion after device selection invalidates its scope", async () => {
  const service = fixture();
  const reset = deferred<unknown>();
  service.ResetToFactory.mockReturnValue(reset.promise);
  service.RefreshInventory.mockResolvedValue({ Selected: { ID: { Serial: "alpha" } }, Devices: [{ ID: { Serial: "beta" } }] });
  const selectable = { ...service, SelectDevice: vi.fn().mockResolvedValue({ Selected: { ID: { Serial: "beta" } }, Devices: [] }), GetMacroAssignmentSnapshot: vi.fn().mockResolvedValue({ Revision: 3 }) };
  const { result } = renderHook(() => useDesktopWorkspace(selectable as unknown as DesktopService));
  await waitFor(() => expect(service.GetRemapSnapshot).toHaveBeenCalledTimes(2));
  act(() => result.current.actions.confirmReset());
  act(() => result.current.actions.selectDevice("beta"));
  await waitFor(() => expect(result.current.model.remap).toEqual({ Revision: 3 }));
  await act(async () => reset.resolve({ Cleanup: { State: "success" }, Error: { Code: "" } }));
  expect(service.GetSnapshot).not.toHaveBeenCalled();
  expect(result.current.model.notice).not.toBe("Factory reset completed.");
});
it("ignores a reset completion from a replaced service", async () => {
  const old = fixture();
  const reset = deferred<unknown>();
  old.ResetToFactory.mockReturnValue(reset.promise);
  const next = fixture();
  const { result, rerender } = renderHook(({ service }) => useDesktopWorkspace(service as unknown as DesktopService), { initialProps: { service: old } });
  await waitFor(() => expect(old.GetRemapSnapshot).toHaveBeenCalledTimes(2));
  act(() => result.current.actions.confirmReset());
  rerender({ service: next });
  await waitFor(() => expect(next.GetRemapSnapshot).toHaveBeenCalledTimes(2));
  old.GetSnapshot.mockClear();
  await act(async () => reset.resolve({ Cleanup: { State: "success" }, Error: { Code: "" } }));
  expect(old.GetSnapshot).not.toHaveBeenCalled();
  expect(result.current.model.notice).not.toBe("Factory reset completed.");
});
