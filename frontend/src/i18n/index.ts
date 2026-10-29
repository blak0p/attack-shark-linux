import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import en from "./locales/en.json";
import es from "./locales/es.json";

export const supportedLanguages = ["en", "es"] as const;
export type Language = (typeof supportedLanguages)[number];

const storageKey = "x6.language";

function isSupported(value: string | null | undefined): value is Language {
  return supportedLanguages.includes(value as Language);
}

export function detectLanguage(): Language {
  if (typeof window === "undefined") return "en";
  try {
    const stored = window.localStorage.getItem(storageKey);
    if (isSupported(stored)) return stored;
  } catch {
    // Storage can be unavailable; fall back to the system language.
  }
  const system = window.navigator.language?.split("-")[0].toLowerCase();
  return isSupported(system) ? system : "en";
}

export async function setLanguage(language: Language): Promise<void> {
  try {
    window.localStorage.setItem(storageKey, language);
  } catch {
    // The choice still applies for this session.
  }
  await i18n.changeLanguage(language);
  if (typeof document !== "undefined") document.documentElement.lang = language;
}

void i18n.use(initReactI18next).init({
  resources: { en: { translation: en }, es: { translation: es } },
  lng: detectLanguage(),
  fallbackLng: "en",
  supportedLngs: supportedLanguages,
  interpolation: { escapeValue: false },
  initAsync: false,
});
if (typeof document !== "undefined") document.documentElement.lang = i18n.language;

export { i18n };
