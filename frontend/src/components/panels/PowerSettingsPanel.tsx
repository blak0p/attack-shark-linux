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
  return (
    <div className="group">
      <div className="card-head">
        <h2>Key response time</h2>
        <span aria-label="Key response time value">{value} ms</span>
      </div>
      <div className="range-row">
        <span>2 ms</span>
        <input
          type="range"
          aria-label="Key response time"
          min="2"
          max="25"
          step="1"
          value={value}
          disabled={disabled}
          onChange={(event) => onChange(Number(event.target.value))}
        />
        <span>25 ms</span>
      </div>
      {!feedbackManaged && <div className="power-settings-state status" role="status" aria-label="Key response time status">
        {firmwareStatus === "pending"
          ? "Applying…"
          : firmwareStatus === "failed"
          ? <>
              Key response time change failed
              {errorCode && feedbackFor && <>: <span role="alert">{feedbackFor(errorCode)}</span></>}
            </>
          : `Applied ${appliedValue ?? value} ms`}
        {persistenceStatus === "failed" && <span> Key response time was not saved.</span>}
        {retryAvailable && (
          <button type="button" className="button" style={{ marginLeft: 8 }} onClick={onRetry}>
            Retry key response persistence
          </button>
        )}
      </div>}
    </div>
  );
}
