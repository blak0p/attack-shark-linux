import type { Macro } from "../desktop-contract";

// Browser file resource budget only; not an event count or firmware limit.
export const MAX_MACRO_FILE_BYTES = 1024 * 1024;
type MacroCopy = Pick<Macro, "name" | "events">;
const fail = (message: string): never => { throw new Error(message); };
const objectWithKeys = (value: unknown, keys: string[], label: string): Record<string, unknown> => {
  if (!value || typeof value !== "object" || Array.isArray(value)) return fail(`Invalid ${label} object.`);
  const object = value as Record<string, unknown>;
  if (Object.keys(object).length !== keys.length || keys.some((key) => !Object.hasOwn(object, key))) {
    return fail(`Invalid ${label} fields: expected only ${keys.join(", ")}.`);
  }
  return object;
};
const validate = (value: unknown): MacroCopy => {
  const file = objectWithKeys(value, ["version", "name", "events"], "macro file");
  if (file.version !== 1) return fail("Unsupported macro file version; expected version 1.");
  if (typeof file.name !== "string" || !file.name.trim()) return fail("Macro name must be a nonblank string.");
  if (!Array.isArray(file.events)) return fail("Macro events must be an ordered array.");
  const events = file.events.map((value, index) => {
    const event = objectWithKeys(value, ["type", "action", "delay_ms"], `event ${index + 1}`);
    if (event.type !== "mouse_left" && event.type !== "mouse_right" && event.type !== "mouse_middle" && event.type !== "mouse_back" && event.type !== "mouse_forward") return fail(`Event ${index + 1} has an unsupported type.`);
    if (event.action !== "down" && event.action !== "up") return fail(`Event ${index + 1} has an unsupported action.`);
    if (typeof event.delay_ms !== "number" || !Number.isSafeInteger(event.delay_ms) || event.delay_ms < 0) {
      return fail(`Event ${index + 1} delay_ms must be a safe nonnegative integer of local milliseconds.`);
    }
    return { type: event.type, action: event.action, delay_ms: event.delay_ms };
  });
  return { name: file.name, events };
};
export function importMacroFile(text: string): MacroCopy {
  if (text.length > MAX_MACRO_FILE_BYTES || new TextEncoder().encode(text).length > MAX_MACRO_FILE_BYTES) {
    return fail("Macro JSON file is too large (local file safety budget: 1 MiB).");
  }
  let value: unknown;
  try { value = JSON.parse(text); } catch { return fail("Invalid macro JSON: malformed or trailing data."); }
  return validate(value);
}
export function exportMacroFile(macro: Macro): string {
  const copy = validate({ version: 1, name: macro.name, events: macro.events });
  const text = JSON.stringify({ version: 1, ...copy }, null, 2) + "\n";
  // Keep exports within the same resource budget so they can be imported again.
  importMacroFile(text);
  return text;
}
