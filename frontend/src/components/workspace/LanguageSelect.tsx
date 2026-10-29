import { useTranslation } from "react-i18next";
import { setLanguage, supportedLanguages, type Language } from "../../i18n";

export function LanguageSelect() {
  const { t, i18n } = useTranslation();
  const current = (supportedLanguages as readonly string[]).includes(i18n.language) ? i18n.language : "en";
  return (
    <select
      className="language-select"
      aria-label={t("language.label")}
      value={current}
      onChange={(event) => { void setLanguage(event.target.value as Language); }}
    >
      {supportedLanguages.map((language) => (
        <option key={language} value={language}>{t(`language.${language}`)}</option>
      ))}
    </select>
  );
}
