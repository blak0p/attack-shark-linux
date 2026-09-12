import type { NormalSleepSnapshot } from "../../desktop-contract";

type SleepPanelProps = {
  snapshot: NormalSleepSnapshot;
  ready: boolean;
  onStage: (minutes: number) => void;
  onRetry: () => void;
};

export function SleepPanel({ snapshot, ready, onStage, onRetry }: SleepPanelProps) {
  return <fieldset className="polling-control" disabled={!ready}>
    <legend>Normal sleep</legend>
    <label className="lighting-effect-select">
      <span>Sleep after</span>
      <input aria-label="Normal sleep duration" type="range" min="0.5" max="60" step="0.5" value={snapshot.Pending} disabled={!ready} onChange={(event) => onStage(Number(event.target.value))} />
      <output>{snapshot.Pending} minutes</output>
    </label>
    <div role="status" aria-label="Normal sleep status">
      {snapshot.Firmware === "pending" ? "Applying…" : snapshot.Firmware === "failed" ? "Normal sleep application failed" : `Applied ${snapshot.Applied} minutes`}
      {snapshot.Persistence === "failed" && <span> Sleep preference was not saved.</span>}
      {snapshot.RetryAvailable && <button type="button" onClick={onRetry}>Retry sleep persistence</button>}
    </div>
  </fieldset>;
}
