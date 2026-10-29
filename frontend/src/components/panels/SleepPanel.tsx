import { useTranslation } from "react-i18next";
import type { NormalSleepSnapshot } from "../../desktop-contract";

type SleepPanelProps = {
  feedbackManaged?: boolean;
  snapshot: NormalSleepSnapshot;
  ready: boolean;
  onStage: (minutes: number) => void;
  onRetry: () => void;
  errorCode?: string;
  feedbackFor?: (code: string) => string;
};

export function SleepPanel({ snapshot, ready, onStage, onRetry, errorCode, feedbackFor, feedbackManaged = false }: SleepPanelProps) {
  const { t } = useTranslation();
  return (
    <article id="normal-sleep" className="card" aria-labelledby="sleep-title">
      <h2 id="sleep-title">{t("sleep.title")}</h2>
      <p className="hint">
        {t("sleep.after", { count: snapshot.Pending })}
      </p>
      <div className="range-row">
        <span>0.5</span>
        <input
          type="range"
          aria-label={t("sleep.duration")}
          min="0.5"
          max="60"
          step="0.5"
          value={snapshot.Pending}
          disabled={!ready}
          onChange={(event) => onStage(Number(event.target.value))}
        />
        <span>{t("sleep.max")}</span>
      </div>
      {!feedbackManaged && <div className="status" role="status" aria-label={t("sleep.status")}>
        {snapshot.Firmware === "pending"
          ? t("common.applying")
          : snapshot.Firmware === "failed"
          ? <>
              {t("sleep.failed")}
              {errorCode && feedbackFor && <>: <span role="alert">{feedbackFor(errorCode)}</span></>}
            </>
          : t("sleep.applied", { minutes: snapshot.Applied })}
        {snapshot.Persistence === "failed" && <span> {t("sleep.notSaved")}</span>}
        {snapshot.RetryAvailable && (
          <button type="button" className="button" style={{ marginLeft: 8 }} onClick={onRetry}>
            {t("sleep.retry")}
          </button>
        )}
      </div>}
    </article>
  );
}
