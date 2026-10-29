import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

export type ResetPanelProps = {
  confirmation: boolean;
  ready?: boolean;
  onRequestReset?: () => void;
  onConfirm: () => void;
  onCancel: () => void;
  children?: ReactNode;
};

export function ResetPanel({
  confirmation,
  ready = true,
  onRequestReset,
  onConfirm,
  onCancel,
  children,
}: ResetPanelProps) {
  const { t } = useTranslation();
  return (
    <article id="reset" className="card" aria-labelledby="reset-title">
      <div className="card-head">
        <div>
          <h2 id="reset-title">{t("reset.title")}</h2>
          <p className="hint" style={{ margin: "3px 0 0" }}>
            {t("reset.hint")}
          </p>
        </div>
        {children ?? (
          <button
            type="button"
            className="button"
            disabled={!ready}
            onClick={onRequestReset}
          >
            {t("reset.title")}
          </button>
        )}
      </div>

      {confirmation && (
        <div
          role="alertdialog"
          aria-label={t("reset.confirmLabel")}
          className="group"
          style={{ marginTop: 14 }}
        >
          <p>{t("reset.warning")}</p>
          <details className="group"><summary>{t("reset.details")}</summary>
          <p style={{ margin: 0 }}>
            {t("reset.detailsBody")}
          </p>
          </details>
          <div className="actions">
            <button type="button" className="button" onClick={onCancel}>
              {t("reset.cancel")}
            </button>
            <button type="button" className="button primary" onClick={onConfirm}>
              {t("reset.confirm")}
            </button>
          </div>
        </div>
      )}
    </article>
  );
}
