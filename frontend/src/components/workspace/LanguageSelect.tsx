import { useTranslation } from "react-i18next";
import { setLanguage, supportedLanguages, type Language } from "../../i18n";
import { GnomeSelect } from "../panels/GnomeSelect";

export function LanguageSelect() {
  const { t, i18n } = useTranslation();
  const current = (supportedLanguages as readonly string[]).includes(i18n.language) ? i18n.language : "en";
  return (
    <GnomeSelect
      aria-label={t("language.label")}
      value={current}
      options={supportedLanguages.map((language) => ({ value: language, label: t(`language.${language}`), direct: true }))}
      onChange={(value) => { void setLanguage(value as Language); }}
    />
  );
}
