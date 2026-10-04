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
