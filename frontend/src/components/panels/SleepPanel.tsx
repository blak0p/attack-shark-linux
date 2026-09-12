import type { NormalSleepSnapshot } from "../../desktop-contract";

type SleepPanelProps = {
  snapshot: NormalSleepSnapshot;
  ready: boolean;
  onStage: (minutes: number) => void;
  onRetry: () => void;
};

export function SleepPanel({ snapshot, ready, onStage, onRetry }: SleepPanelProps) {
  return (
    <article id="normal-sleep" className="card" aria-labelledby="sleep-title">
      <h2 id="sleep-title">Normal sleep</h2>
      <p className="hint">
        Sleep after: {snapshot.Pending} {snapshot.Pending === 1 ? "minute" : "minutes"}
      </p>
      <div className="range-row">
        <span>0.5</span>
        <input
          type="range"
          aria-label="Normal sleep duration"
          min="0.5"
          max="60"
          step="0.5"
          value={snapshot.Pending}
          disabled={!ready}
          onChange={(event) => onStage(Number(event.target.value))}
        />
        <span>60 minutes</span>
      </div>
      <div className="status" role="status" aria-label="Normal sleep status">
        {snapshot.Firmware === "pending"
          ? "Applying…"
          : snapshot.Firmware === "failed"
          ? "Normal sleep application failed"
          : `Applied ${snapshot.Applied} minutes`}
        {snapshot.Persistence === "failed" && <span> Sleep preference was not saved.</span>}
        {snapshot.RetryAvailable && (
          <button type="button" className="button" style={{ marginLeft: 8 }} onClick={onRetry}>
            Retry sleep persistence
          </button>
        )}
      </div>
    </article>
  );
}
