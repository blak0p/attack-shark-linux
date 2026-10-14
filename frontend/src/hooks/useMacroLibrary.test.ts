import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import type { Macro, MacroLibraryService } from "../desktop-contract";
import { exportMacroFile } from "../macros/macro-file";
import { useMacroLibrary } from "./useMacroLibrary";

afterEach(cleanup);

it("rejects recording in the same tick as a read, delete request or import starts", async () => {
  let readDone!: (macro: Macro) => void;
  const service: MacroLibraryService = { ListMacros: vi.fn().mockResolvedValue([]), ReadMacro: vi.fn().mockImplementation(() => new Promise<Macro>(resolve => { readDone = resolve; })), CreateMacro: vi.fn(), UpdateMacro: vi.fn(), DeleteMacro: vi.fn() };
  const { result } = renderHook(() => useMacroLibrary(service));
  await waitFor(() => expect(result.current.loaded).toBe(true));
  act(() => result.current.newMacro());
  act(() => result.current.rename("Named"));
  act(() => { result.current.select("selected"); expect(result.current.beginRecording()).toBeUndefined(); result.current.save(); });
  expect(service.CreateMacro).not.toHaveBeenCalled();
  await act(async () => readDone({ id: "selected", name: "Selected", events: [] }));
  act(() => { result.current.requestDelete(); expect(result.current.beginRecording()).toBeUndefined(); });
  act(() => result.current.cancelDelete());
  let textDone!: (text: string) => void;
  let pending!: Promise<void>;
  act(() => { pending = result.current.importFile(() => new Promise(resolve => { textDone = resolve; })); expect(result.current.beginRecording()).toBeUndefined(); });
  await act(async () => { textDone("{}"); await pending; });
  act(() => { result.current.reload(); expect(result.current.beginRecording()).toBeUndefined(); });
  await waitFor(() => expect(result.current.loading).toBe(false));
});

it("retains latest-read ordering while blocking take start during pending operations", async () => {
  let resolve!: (macro: Macro) => void;
  const old = new Promise<Macro>(done => { resolve = done; });
  const service: MacroLibraryService = { ListMacros: vi.fn().mockResolvedValue([]), ReadMacro: vi.fn().mockImplementation(id => id === "old" ? old : Promise.resolve({ id, name: id, events: [] })), CreateMacro: vi.fn(), UpdateMacro: vi.fn(), DeleteMacro: vi.fn() };
  const { result } = renderHook(() => useMacroLibrary(service));
  expect(result.current.beginRecording()).toBeUndefined();
  await waitFor(() => expect(result.current.loaded).toBe(true));
  act(() => result.current.select("old"));
  expect(result.current.beginRecording()).toBeUndefined();
  act(() => result.current.select("latest"));
  await waitFor(() => expect(result.current.draft?.id).toBe("latest"));
  await act(async () => resolve({ id: "old", name: "old", events: [] }));
  expect(result.current.draft?.id).toBe("latest");
  act(() => result.current.requestDelete());
  expect(result.current.beginRecording()).toBeUndefined();
  act(() => result.current.save());
  expect(service.UpdateMacro).not.toHaveBeenCalled();
});

it("locks synchronous library operations during a take and rejects stale delivery after service change", async () => {
  const makeService = (): MacroLibraryService => ({ ListMacros: vi.fn().mockResolvedValue([]), ReadMacro: vi.fn(), CreateMacro: vi.fn(), UpdateMacro: vi.fn(), DeleteMacro: vi.fn() });
  const first = makeService(), second = makeService();
  const { result, rerender } = renderHook(({ service }) => useMacroLibrary(service), { initialProps: { service: first } });
  await waitFor(() => expect(result.current.loaded).toBe(true));
  act(() => result.current.newMacro());
  act(() => result.current.rename("Take"));
  let token: symbol | undefined;
  act(() => { token = result.current.beginRecording(); });
  expect(token).toBeDefined();
  act(() => { result.current.save(); result.current.select("other"); result.current.newMacro(); result.current.rename("Wrong"); result.current.requestDelete(); });
  await act(async () => result.current.importFile(async () => "{}"));
  expect(first.CreateMacro).not.toHaveBeenCalled();
  expect(first.ReadMacro).not.toHaveBeenCalled();
  expect(result.current.draft?.name).toBe("Take");
  expect(result.current.confirmation).toBe(false);
  rerender({ service: second });
  await waitFor(() => expect(result.current.loaded).toBe(true));
  act(() => result.current.newMacro());
  act(() => { result.current.finishRecording(token!, [{ type: "keyboard", key_code: "KeyA", action: "down", delay_ms: 0 }]); });
  expect(result.current.draft?.events).toEqual([]);
});

it("preserves keyboard copies across import, edits, reorder and save while blocking stale identities", async () => {
  const original = { id: "original", name: "Keyboard", events: [
    { type: "keyboard", action: "down", delay_ms: 123, key_code: " UnknownPhysicalKey " },
    { type: "keyboard", action: "up", delay_ms: 456, key_code: "KeyA" },
  ] } as Macro;
  const before = structuredClone(original);
  const service: MacroLibraryService = {
    ListMacros: vi.fn().mockResolvedValue([]), ReadMacro: vi.fn().mockResolvedValue(original),
    CreateMacro: vi.fn().mockImplementation(async (name, events) => ({ id: "copy", name, events })),
    UpdateMacro: vi.fn().mockImplementation(async (id, name, events) => ({ id, name, events })),
    DeleteMacro: vi.fn(),
  };
  const { result } = renderHook(() => useMacroLibrary(service));
  await waitFor(() => expect(result.current.loaded).toBe(true));
  await act(async () => result.current.importFile(async () => JSON.stringify({ version: 1, name: original.name, events: original.events })));
  expect(service.CreateMacro).toHaveBeenCalledWith(original.name, original.events);
  act(() => { result.current.rename("Edited"); result.current.moveEvent(0, 1); });
  act(() => result.current.save());
  await waitFor(() => expect(result.current.notice).toContain("Saved"));
  expect(service.UpdateMacro).toHaveBeenCalledWith("copy", "Edited", [...original.events].reverse());
  expect(JSON.parse(exportMacroFile(result.current.savedMacro!)).events).toEqual([...original.events].reverse());
  act(() => result.current.updateEvent(0, { key_code: " \t" }));
  expect(result.current.validationError).toMatch(/key_code/);
  act(() => result.current.save());
  expect(service.UpdateMacro).toHaveBeenCalledTimes(1);
  act(() => result.current.updateEvent(0, { type: "mouse_left", key_code: "KeyA" }));
  expect(result.current.validationError).toMatch(/key_code/);
  act(() => result.current.save());
  expect(service.UpdateMacro).toHaveBeenCalledTimes(1);
  expect(original).toEqual(before);
});

it.each(["legacy", "imported"])("preserves %s delay data through selection, edits, reorder, save and export", async (source) => {
  const original: Macro = { id: "existing", name: "Preserved", events: [
    { type: "mouse_left", action: "down", delay_ms: 123 },
    { type: "mouse_right", action: "up", delay_ms: 456789 },
  ] };
  const service: MacroLibraryService = {
    ListMacros: vi.fn().mockResolvedValue([original]),
    ReadMacro: vi.fn().mockResolvedValue(original),
    CreateMacro: vi.fn().mockImplementation(async (name, events) => ({ id: "imported", name, events })),
    UpdateMacro: vi.fn().mockImplementation(async (id, name, events) => ({ id, name, events })),
    DeleteMacro: vi.fn().mockResolvedValue(undefined),
  };
  const { result } = renderHook(() => useMacroLibrary(service));
  await waitFor(() => expect(result.current.loaded).toBe(true));
  if (source === "imported") {
    await act(async () => result.current.importFile(async () => exportMacroFile(original)));
    expect(service.CreateMacro).toHaveBeenCalledWith(original.name, original.events);
  } else {
    act(() => result.current.select(original.id));
    await waitFor(() => expect(result.current.draft?.id).toBe(original.id));
  }
  expect(result.current.draft?.events).toEqual(original.events);
  const id = result.current.draft!.id;
  act(() => {
    result.current.rename("Renamed");
    result.current.updateEvent(0, { type: "mouse_right", action: "up" });
    result.current.updateEvent(1, { type: "mouse_left", action: "down" });
    result.current.moveEvent(0, 1);
  });
  const expected = [
    { type: "mouse_left", action: "down", delay_ms: 456789 },
    { type: "mouse_right", action: "up", delay_ms: 123 },
  ];
  expect(result.current.draft?.events).toEqual(expected);
  expect(original.events.map((event) => event.delay_ms)).toEqual([123, 456789]);
  act(() => result.current.save());
  await waitFor(() => expect(result.current.notice).toBe("Saved to library. No device changes made."));
  expect(service.UpdateMacro).toHaveBeenCalledWith(id, "Renamed", expected);
  expect(JSON.parse(exportMacroFile(result.current.savedMacro!))).toEqual({
    version: 1, name: "Renamed", events: expected,
  });
});
