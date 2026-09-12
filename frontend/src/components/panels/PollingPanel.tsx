import type { ReactNode } from "react";
import type { PollingSnapshot } from "../../desktop-contract";

export type PollingPanelProps = {
  snapshot?: PollingSnapshot | null;
  ready?: boolean;
  onStagePollingRate?: (rate: number) => void;
  onRetry?: () => void;
  children?: ReactNode;
};

export function PollingPanel({
  snapshot,
  ready = true,
  onStagePollingRate,
  onRetry,
  children,
}: PollingPanelProps) {
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
      <h2 id="polling-title">Polling rate</h2>
      <p className="hint">Higher rates favor responsiveness; lower rates favor battery life.</p>
      <div className="segmented" role="radiogroup" aria-label="Polling rate">
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
      <div className="status" role="status" aria-label="Polling status">
        {snapshot.Firmware === "pending" ? (
          <>
            <span className="pending-spinner" aria-hidden="true" />
            Applying…
          </>
        ) : snapshot.Firmware === "failed" ? (
          <>Polling change failed</>
        ) : (
          <>Applied {snapshot.Applied} Hz</>
        )}
        {snapshot.Persistence === "failed" && <span> Polling preference was not saved.</span>}
        {snapshot.RetryAvailable && (
          <button type="button" className="button" style={{ marginLeft: 8 }} onClick={onRetry}>
            Retry polling persistence
          </button>
        )}
      </div>
    </article>
  );
}
