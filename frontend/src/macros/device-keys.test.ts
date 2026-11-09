import { describe, expect, it } from "vitest";
import { deviceKeyCode, setDeviceKeyCodes } from "./device-keys";
import { expected } from "./test-key-table";
import { deviceAdmission } from "./device-admission";
import type { MacroEvent } from "../desktop-contract";

const key = (key_code: string, action: "down" | "up"): MacroEvent => ({ type: "keyboard", key_code, action, delay_ms: 10 });

describe("official physical identity parity", () => {
  it("keeps the independently audited 104 identities delivered by the backend", () => {
    expect(Object.keys(expected)).toHaveLength(104);
  });
  it("admits nothing until the backend delivers a table, and replaces it atomically", () => {
    const events = [key("KeyA", "down"), key("KeyA", "up")];
    setDeviceKeyCodes({});
    expect(deviceKeyCode("KeyA")).toBeUndefined();
    expect(deviceAdmission(events).eligible).toBe(false);
    setDeviceKeyCodes({ KeyA: 4 });
    expect(deviceKeyCode("KeyA")).toBe(4);
    expect(deviceAdmission(events).eligible).toBe(true);
    setDeviceKeyCodes(expected);
  });
  it.each(Object.entries(expected))("maps and admits %s as raw %i", (identity, code) => {
    expect(deviceKeyCode(identity)).toBe(code);
    const events = [key(identity, "down"), key(identity, "up")];
    const before = structuredClone(events);
    expect(deviceAdmission(events).eligible).toBe(true);
    expect(events).toEqual(before);
  });
  it.each(["__proto__", "constructor", "toString", "Unknown", "keya", " KeyA", "KeyA ", "F13", "F24", "IntlBackslash", "MediaPlayPause", "BrowserBack", "Control", "Shift", "Alt", "NumpadSeparator", "OEM8", "Lang1", "Convert", "NonConvert"])("rejects exact unsupported identity %s atomically", identity => {
    const events = [key(identity, "down"), key(identity, "up")];
    const before = structuredClone(events);
    expect(deviceKeyCode(identity)).toBeUndefined();
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
