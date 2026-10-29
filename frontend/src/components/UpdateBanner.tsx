import { useTranslation } from "react-i18next";
import type { UpdateInfo } from "../desktop-contract";

export function UpdateBanner({ update, applying, error, onApply }: { update: UpdateInfo; applying: boolean; error: string; onApply(): void }) {
  const { t } = useTranslation();
  return <aside className="update-banner" role="status" aria-live="polite">
    <span>{t("updateBanner.ready", { version: update.Version })}</span>
    <button type="button" className="button" disabled={applying} onClick={onApply}>
      {t("updateBanner.apply")}
    </button>
    {error && <span role="alert">{t("updateBanner.failed", { error })}</span>}
  </aside>;
}
