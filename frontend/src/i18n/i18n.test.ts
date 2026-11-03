import { vi } from "vitest";
import { beforeEach, describe, expect, it } from "vitest";
import { detectLanguage, i18n, setLanguage } from "./index";
import en from "./locales/en.json";
import es from "./locales/es.json";

function keys(value: Record<string, unknown>, prefix = ""): string[] {
  return Object.entries(value).flatMap(([key, child]) =>
    typeof child === "object" && child !== null
      ? keys(child as Record<string, unknown>, `${prefix}${key}.`)
      : [`${prefix}${key}`],
  );
}

// Languages need different plural categories (one/many/other); compare the base keys only.
const withoutPlural = (list: string[]) => [...new Set(list.map((key) => key.replace(/_(zero|one|two|few|many|other)$/, "")))].sort();

describe("i18n", () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  it("defaults to English when the system language is unsupported", () => {
    vi.spyOn(window.navigator, "language", "get").mockReturnValue("fr-FR");
    expect(detectLanguage()).toBe("en");
  });

  it("follows the system language when it is supported", () => {
    vi.spyOn(window.navigator, "language", "get").mockReturnValue("es-AR");
    expect(detectLanguage()).toBe("es");
  });

  it("prefers the stored choice over the system language", () => {
    vi.spyOn(window.navigator, "language", "get").mockReturnValue("es-AR");
    window.localStorage.setItem("x6.language", "en");
    expect(detectLanguage()).toBe("en");
  });

  it("persists the choice and switches the active language", async () => {
    await setLanguage("es");
    expect(window.localStorage.getItem("x6.language")).toBe("es");
    expect(i18n.language).toBe("es");
    expect(document.documentElement.lang).toBe("es");
    await setLanguage("en");
    expect(i18n.language).toBe("en");
  });

  it("keeps Spanish complete against the English base", () => {
    expect(withoutPlural(keys(es))).toEqual(withoutPlural(keys(en)));
  });
});
