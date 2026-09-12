type PowerSettingsPanelProps = {
  value: number;
  disabled?: boolean;
  onChange: (value: number) => void;
};

export function PowerSettingsPanel({ value, disabled = false, onChange }: PowerSettingsPanelProps) {
  return <fieldset className="polling-control" disabled={disabled}>
    <legend>Key response time</legend>
    <p className="polling-guidance">Lower values reduce click response latency.</p>
    <label className="lighting-effect-select">
      <span>Response time</span>
      <input type="range" aria-label="Key response time" min="2" max="32" step="2" value={value} disabled={disabled} onChange={(event) => onChange(Number(event.target.value))} />
      <output>{value} ms</output>
    </label>
  </fieldset>;
}
