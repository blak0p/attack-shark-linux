import { expect, it } from "vitest";
import { exportMacroFile, importMacroFile, MAX_MACRO_FILE_BYTES } from "./macro-file";

const macro = { id: "local", name: "Clicks", events: [
  { type: "mouse_left" as const, action: "down" as const, delay_ms: 0 },
  { type: "mouse_right" as const, action: "up" as const, delay_ms: Number.MAX_SAFE_INTEGER },
] };
it("roundtrips one versioned macro in order without its local ID", () => {
  const text = exportMacroFile(macro);
  expect(JSON.parse(text)).toEqual({ version: 1, name: macro.name, events: macro.events });
  expect(importMacroFile(text)).toEqual({ name: macro.name, events: macro.events });
  expect(importMacroFile(JSON.stringify({ version: 1, name: "Empty", events: [] }))).toEqual({ name: "Empty", events: [] });
});
it.each(["mouse_middle", "mouse_back", "mouse_forward"] as const)("roundtrips stored %s events without changing delays", (type) => {
  const events = [{ type, action: "down" as const, delay_ms: 17 }, { type, action: "up" as const, delay_ms: 23 }];
  const text = JSON.stringify({ version: 1, name: "Extended", events });
  expect(importMacroFile(text)).toEqual({ name: "Extended", events });
  expect(importMacroFile(exportMacroFile({ id: "local", name: "Extended", events }))).toEqual({ name: "Extended", events });
});
it.each(["mouse_arbitrary", "mouse_backward", "", "keyboard"])("rejects arbitrary type %s on import and export", (type) => {
  const events = [{ type, action: "down" as const, delay_ms: 0 }];
  expect(() => importMacroFile(JSON.stringify({ version: 1, name: "Invalid", events }))).toThrow(/unsupported type/);
  expect(() => exportMacroFile({ id: "local", name: "Invalid", events } as unknown as Parameters<typeof exportMacroFile>[0])).toThrow(/unsupported type/);
});
it.each([
  "{", "{}", "null", "[]", JSON.stringify({ version: 2, name: "A", events: [] }),
  JSON.stringify({ version: 1, name: " ", events: [] }),
  JSON.stringify({ version: 1, name: 1, events: [] }),
  JSON.stringify({ version: 1, name: "A", events: null }),
  JSON.stringify({ version: 1, name: "A", events: [], id: "old" }),
  JSON.stringify({ version: 1, name: "A", events: [] }) + " trailing",
  ...[null, {}, { type: "keyboard", action: "down", delay_ms: 0 },
    { type: "mouse_left", action: "click", delay_ms: 0 },
    ...[-1, 0.5, Number.MAX_SAFE_INTEGER + 1, "0", null].map((delay_ms) => ({ type: "mouse_left", action: "up", delay_ms })),
    { type: "mouse_left", action: "down", delay_ms: 0, extra: true },
  ].map((event) => JSON.stringify({ version: 1, name: "A", events: [event] })),
])("rejects invalid input: %s", (text) => expect(() => importMacroFile(text)).toThrow(/macro|JSON|version|event|name/i));
it("rejects oversized input before parsing (a local file safety budget, not firmware)", () => {
  expect(() => importMacroFile(" ".repeat(MAX_MACRO_FILE_BYTES + 1))).toThrow(/too large/i);
});
