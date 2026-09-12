export type PowerSettingsPanelProps = {
  value: number;
  appliedValue?: number;
  firmwareStatus?: string;
  persistenceStatus?: string;
  retryAvailable?: boolean;
  disabled?: boolean;
  onChange: (value: number) => void;
  onRetry?: () => void;
};

export function PowerSettingsPanel({
  value,
  appliedValue,
  firmwareStatus,
  persistenceStatus,
  retryAvailable,
  disabled = false,
  onChange,
  onRetry,
}: PowerSettingsPanelProps) {
  return (
    <div className="group">
      <h2>Key response time</h2>
      <p className="hint">Lower values reduce click response latency.</p>
      <div className="range-row">
        <span>2 ms</span>
        <input
          type="range"
          aria-label="Key response time"
          min="2"
          max="32"
          step="2"
          value={value}
          disabled={disabled}
          onChange={(event) => onChange(Number(event.target.value))}
        />
        <span>32 ms</span>
      </div>
      <div className="power-settings-state status" role="status" aria-label="Key response time status">
        {firmwareStatus === "pending"
          ? "Applying…"
          : firmwareStatus === "failed"
          ? "Key response time change failed"
          : `Applied ${appliedValue ?? value} ms`}
        {persistenceStatus === "failed" && <span> Key response time was not saved.</span>}
        {retryAvailable && (
          <button type="button" className="button" style={{ marginLeft: 8 }} onClick={onRetry}>
            Retry key response persistence
          </button>
        )}
      </div>
    </div>
  );
}
