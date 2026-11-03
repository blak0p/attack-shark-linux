import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import type { PollingSnapshot } from "../../desktop-contract";

export type PollingPanelProps = {
  feedbackManaged?: boolean;
  snapshot?: PollingSnapshot | null;
  ready?: boolean;
  onStagePollingRate?: (rate: number) => void;
  onRetry?: () => void;
  errorCode?: string;
  feedbackFor?: (code: string) => string;
  children?: ReactNode;
};

export function PollingPanel({
  feedbackManaged = false,
  snapshot,
  ready = true,
  onStagePollingRate,
  onRetry,
  errorCode,
  feedbackFor,
  children,
}: PollingPanelProps) {
  const { t } = useTranslation();
  if (children) {
    return (
      <section id="polling" className="card panel-section">
        {children}
      </section>
    );
  }

  if (!snapshot) return null;

  const rates = [125, 250, 500, 1000];

  return (
    <article id="polling" className="card" aria-labelledby="polling-title">
      <h2 id="polling-title">{t("polling.title")}</h2>
      <div className="segmented" role="radiogroup" aria-label={t("polling.title")}>
        {rates.map((rate) => {
          const isSelected = snapshot.Desired === rate;
          const displayLabel = rate >= 1000 ? "1,000 Hz" : `${rate} Hz`;
          return (
            <button
              key={rate}
              type="button"
              role="radio"
              aria-checked={isSelected}
              aria-label={`${rate} Hz`}
              className={isSelected ? "selected" : ""}
              disabled={!ready}
              onClick={() => onStagePollingRate?.(rate)}
            >
              {displayLabel}
            </button>
          );
        })}
      </div>
      {!feedbackManaged && <div className="status" role="status" aria-label={t("polling.status")}>
        {snapshot.Firmware === "pending" ? (
          <>
            <span className="pending-spinner" aria-hidden="true" />
            {t("common.applying")}
          </>
        ) : snapshot.Firmware === "failed" ? (
          <>
            {t("polling.failed")}
            {errorCode && feedbackFor && <>: <span role="alert">{feedbackFor(errorCode)}</span></>}
          </>
        ) : (
          <>{t("polling.applied", { rate: snapshot.Applied })}</>
        )}
        {snapshot.Persistence === "failed" && <span> {t("polling.notSaved")}</span>}
        {snapshot.RetryAvailable && (
          <button type="button" className="button" style={{ marginLeft: 8 }} onClick={onRetry}>
            {t("polling.retry")}
          </button>
        )}
      </div>}
    </article>
  );
}
