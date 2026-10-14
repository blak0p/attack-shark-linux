import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import type { Macro, MacroLibraryService } from "../desktop-contract";
import { exportMacroFile } from "../macros/macro-file";
import { useMacroLibrary } from "./useMacroLibrary";

afterEach(cleanup);

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
