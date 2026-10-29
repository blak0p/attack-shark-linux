import type { Macro, MacroEvent } from "../desktop-contract";
import { i18n } from "../i18n";

// Browser file resource budget only; not an event count or firmware limit.
export const MAX_MACRO_FILE_BYTES = 1024 * 1024;
type MacroCopy = Pick<Macro, "name" | "events">;
const fail = (message: string): never => { throw new Error(message); };
const objectWithKeys = (value: unknown, keys: string[], label: string, optional: string[] = []): Record<string, unknown> => {
  if (!value || typeof value !== "object" || Array.isArray(value)) return fail(i18n.t("macroFile.invalidObject", { label }));
  const object = value as Record<string, unknown>;
  if (Object.keys(object).some((key) => !keys.includes(key) && !optional.includes(key)) || keys.some((key) => !Object.hasOwn(object, key))) {
    return fail(i18n.t("macroFile.invalidFields", { label, keys: keys.join(", ") }));
  }
  return object;
};
// Shared strict local boundary: copies data, preserves opaque identities, rejects unknown fields.
export function copyMacroEvent(value: unknown, label = i18n.t("macroFile.event")): MacroEvent {
  const event = objectWithKeys(value, ["type", "action", "delay_ms"], label, ["key_code"]);
  if (event.type !== "keyboard" && event.type !== "mouse_left" && event.type !== "mouse_right" && event.type !== "mouse_middle" && event.type !== "mouse_back" && event.type !== "mouse_forward") return fail(i18n.t("macroFile.unsupportedType", { label }));
  if (event.action !== "down" && event.action !== "up") return fail(i18n.t("macroFile.unsupportedAction", { label }));
  if (Object.hasOwn(event, "key_code") && typeof event.key_code !== "string") return fail(i18n.t("macroFile.keyCodeString", { label }));
  if (event.type === "keyboard" ? typeof event.key_code !== "string" || !event.key_code.trim() : event.key_code !== undefined && event.key_code !== "") {
    return fail(i18n.t("macroFile.keyCodeInvalid", { label }));
  }
  if (typeof event.delay_ms !== "number" || !Number.isSafeInteger(event.delay_ms) || event.delay_ms < 0) return fail(i18n.t("macroFile.delayInvalid", { label }));
  return { type: event.type, action: event.action, delay_ms: event.delay_ms,
    ...(Object.hasOwn(event, "key_code") ? { key_code: event.key_code as string } : {}) };
}
const validate = (value: unknown): MacroCopy => {
  const file = objectWithKeys(value, ["version", "name", "events"], i18n.t("macroFile.fileLabel"));
  if (file.version !== 1) return fail(i18n.t("macroFile.version"));
  if (typeof file.name !== "string" || !file.name.trim()) return fail(i18n.t("macroFile.nameBlank"));
  if (!Array.isArray(file.events)) return fail(i18n.t("macroFile.eventsArray"));
  const events = file.events.map((value, index) => copyMacroEvent(value, i18n.t("macroFile.eventN", { n: index + 1 })));
  return { name: file.name, events };
};
export function importMacroFile(text: string): MacroCopy {
  if (text.length > MAX_MACRO_FILE_BYTES || new TextEncoder().encode(text).length > MAX_MACRO_FILE_BYTES) {
    return fail(i18n.t("macroFile.tooLarge"));
  }
  let value: unknown;
  try { value = JSON.parse(text); } catch { return fail(i18n.t("macroFile.malformed")); }
  return validate(value);
}
export function exportMacroFile(macro: Macro): string {
  const copy = validate({ version: 1, name: macro.name, events: macro.events });
  const text = JSON.stringify({ version: 1, ...copy }, null, 2) + "\n";
  // Keep exports within the same resource budget so they can be imported again.
  importMacroFile(text);
  return text;
}
