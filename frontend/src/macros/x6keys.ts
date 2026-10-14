import officialKeys from "../../../internal/macros/x6keys.json";

// Same audited data as Go. Exact browser physical identity, never typed text.
// NumpadEnter aliases VK_RETURN inferentially; separator/OEM8 are not guessed.
export function x6KeyCode(identity: string): number | undefined {
  return Object.prototype.hasOwnProperty.call(officialKeys, identity)
    ? (officialKeys as Record<string, number>)[identity] : undefined;
}
