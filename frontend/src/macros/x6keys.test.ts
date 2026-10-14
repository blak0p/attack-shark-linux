import { describe, expect, it } from "vitest";
import canonical from "../../../internal/macros/x6keys.json";
import { x6KeyCode } from "./x6keys";
import { deviceAdmission } from "./device-admission";
import type { MacroEvent } from "../desktop-contract";

// Independent ranges/rows from official converter evidence, not JSON-derived.
const expected: Record<string, number> = {};
for (let i = 0; i < 26; i++) expected[`Key${String.fromCharCode(65 + i)}`] = 4 + i;
for (let i = 1; i <= 9; i++) expected[`Digit${i}`] = 0x1d + i;
expected.Digit0 = 0x27;
["Enter", "Escape", "Backspace", "Tab", "Space", "Minus", "Equal", "BracketLeft", "BracketRight", "Backslash"].forEach((key, i) => expected[key] = 0x28 + i);
["Semicolon", "Quote", "Backquote", "Comma", "Period", "Slash", "CapsLock"].forEach((key, i) => expected[key] = 0x33 + i);
for (let i = 1; i <= 12; i++) expected[`F${i}`] = 0x39 + i;
["PrintScreen", "ScrollLock", "Pause", "Insert", "Home", "PageUp", "Delete", "End", "PageDown", "ArrowRight", "ArrowLeft", "ArrowDown", "ArrowUp", "NumLock", "NumpadDivide", "NumpadMultiply", "NumpadSubtract", "NumpadAdd"].forEach((key, i) => expected[key] = 0x46 + i);
for (let i = 1; i <= 9; i++) expected[`Numpad${i}`] = 0x58 + i;
Object.assign(expected, { Numpad0: 0x62, NumpadDecimal: 0x63, ContextMenu: 0x65, NumpadEnter: 0x28 });
["ControlLeft", "ShiftLeft", "AltLeft", "MetaLeft", "ControlRight", "ShiftRight", "AltRight", "MetaRight"].forEach((key, i) => expected[key] = 0xe0 + i);
const key = (key_code: string, action: "down" | "up"): MacroEvent => ({ type: "keyboard", key_code, action, delay_ms: 10 });

describe("official physical identity parity", () => {
  it("shares exactly the independently audited 104 identities with embedded Go JSON", () => {
    expect(Object.keys(expected)).toHaveLength(104);
    expect(canonical).toEqual(expected);
  });
  it.each(Object.entries(expected))("maps and admits %s as raw %i", (identity, code) => {
    expect(x6KeyCode(identity)).toBe(code);
    const events = [key(identity, "down"), key(identity, "up")];
    const before = structuredClone(events);
    expect(deviceAdmission(events).eligible).toBe(true);
    expect(events).toEqual(before);
  });
  it.each(["__proto__", "constructor", "toString", "Unknown", "keya", " KeyA", "KeyA ", "F13", "F24", "IntlBackslash", "MediaPlayPause", "BrowserBack", "Control", "Shift", "Alt", "NumpadSeparator", "OEM8", "Lang1", "Convert", "NonConvert"])("rejects exact unsupported identity %s atomically", identity => {
    const events = [key(identity, "down"), key(identity, "up")];
    const before = structuredClone(events);
    expect(x6KeyCode(identity)).toBeUndefined();
    expect(deviceAdmission(events).eligible).toBe(false);
    expect(events).toEqual(before);
  });
  it.each([
    { name: "sequential aliases", codes: [["Enter", "down"], ["Enter", "up"], ["NumpadEnter", "down"], ["NumpadEnter", "up"]], valid: true },
    { name: "simultaneous aliases", codes: [["Enter", "down"], ["NumpadEnter", "down"], ["Enter", "up"], ["NumpadEnter", "up"]], valid: false },
    { name: "mismatched alias release", codes: [["Enter", "down"], ["NumpadEnter", "up"]], valid: false },
    { name: "exact release", codes: [["NumpadEnter", "down"], ["NumpadEnter", "up"]], valid: true },
    { name: "repeated exact down", codes: [["Enter", "down"], ["Enter", "down"], ["Enter", "up"]], valid: false },
  ])("balances encoded identities: $name", ({ codes, valid }) => {
    const events = codes.map(([identity, action]) => key(identity, action as "down" | "up"));
    const before = structuredClone(events);
    expect(deviceAdmission(events).eligible).toBe(valid);
    expect(events).toEqual(before);
  });
});
