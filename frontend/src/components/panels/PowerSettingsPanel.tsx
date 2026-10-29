import { useTranslation } from "react-i18next";

export type PowerSettingsPanelProps = {
  feedbackManaged?: boolean;
  value: number;
  appliedValue?: number;
  firmwareStatus?: string;
  persistenceStatus?: string;
  retryAvailable?: boolean;
  errorCode?: string;
  feedbackFor?: (code: string) => string;
  disabled?: boolean;
  onChange: (value: number) => void;
  onRetry?: () => void;
};

export function PowerSettingsPanel({
  feedbackManaged = false,
  value,
  appliedValue,
  firmwareStatus,
  persistenceStatus,
  retryAvailable,
  errorCode,
  feedbackFor,
  disabled = false,
  onChange,
  onRetry,
}: PowerSettingsPanelProps) {
  const { t } = useTranslation();
  return (
    <div className="group">
      <div className="card-head">
        <h2>{t("power.title")}</h2>
        <span aria-label={t("power.value")}>{value} ms</span>
      </div>
      <div className="range-row">
        <span>2 ms</span>
        <input
          type="range"
          aria-label={t("power.title")}
          min="2"
          max="25"
          step="1"
          value={value}
          disabled={disabled}
          onChange={(event) => onChange(Number(event.target.value))}
        />
        <span>25 ms</span>
      </div>
      {!feedbackManaged && <div className="power-settings-state status" role="status" aria-label={t("power.status")}>
        {firmwareStatus === "pending"
          ? t("common.applying")
          : firmwareStatus === "failed"
          ? <>
              {t("power.failed")}
              {errorCode && feedbackFor && <>: <span role="alert">{feedbackFor(errorCode)}</span></>}
            </>
          : t("power.applied", { ms: appliedValue ?? value })}
        {persistenceStatus === "failed" && <span> {t("power.notSaved")}</span>}
        {retryAvailable && (
          <button type="button" className="button" style={{ marginLeft: 8 }} onClick={onRetry}>
            {t("power.retry")}
          </button>
        )}
      </div>}
    </div>
  );
}
