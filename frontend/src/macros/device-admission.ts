import type { MacroEvent } from "../desktop-contract";

export type DeviceAdmission = { eligible: boolean; reason: string };
const mouseTypes = ["mouse_left", "mouse_right", "mouse_middle", "mouse_back", "mouse_forward"];
import { i18n } from "../i18n";
import { x6KeyCode } from "./x6keys";

// Mirrors Go's local admission and forward codec limits, not physical playback.
// Local storage is deliberately broader; this pure gate never edits events.
export function deviceAdmission(events: readonly MacroEvent[], repeat = 1, mode = 0): DeviceAdmission {
  const reject = (reason: string): DeviceAdmission => ({ eligible: false, reason });
  if (!Number.isInteger(repeat) || repeat < 1 || repeat > 255) return reject(i18n.t("admission.repeat"));
  if (!Number.isInteger(mode) || mode < 0 || mode > 2) return reject(i18n.t("admission.mode"));
  if (events.length < 1 || events.length > 46) return reject(i18n.t("admission.eventCount"));
  const held = new Map<number, string>();
  let extensions = 0;
  for (const [index, event] of events.entries()) {
    let identity: string;
    let raw: number;
    if (event.type === "keyboard") {
      const code = x6KeyCode(event.key_code ?? "");
      if (code === undefined) return reject(i18n.t("admission.keyboardIdentity"));
      identity = event.key_code!;
      raw = code;
    } else {
      if (!mouseTypes.includes(event.type) || (event.key_code !== undefined && event.key_code !== "")) return reject(i18n.t("admission.mouseOnly"));
      identity = event.type;
      raw = 0xf1 + mouseTypes.indexOf(event.type);
    }
    if (!Number.isSafeInteger(event.delay_ms) || event.delay_ms < 0 || event.delay_ms > 65535 ||
      (event.delay_ms >= 1271 && Math.floor(event.delay_ms / 200) > 255)) return reject(i18n.t("admission.delay"));
    if (event.action === "down") {
      if (held.has(raw)) return reject(i18n.t("admission.repeatedDown"));
      held.set(raw, identity);
    } else if (event.action === "up") {
      if (held.get(raw) !== identity) return reject(i18n.t("admission.unmatchedUp"));
      held.delete(raw);
    } else return reject(i18n.t("admission.action"));
    // The terminal event is validated above, then encoded with delay 10.
    if (index < events.length - 1 && event.delay_ms >= 1271) extensions++;
  }
  if (held.size) return reject(i18n.t("admission.incompleteChord"));
  if (extensions > 10) return reject(i18n.t("admission.extensions"));
  if (events.length + extensions > 50) return reject(i18n.t("admission.pairs"));
  return { eligible: true, reason: i18n.t("admission.eligible") };
}
