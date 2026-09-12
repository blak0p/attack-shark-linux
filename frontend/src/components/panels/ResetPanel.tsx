import type { ReactNode } from "react";

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
  return (
    <article id="reset" className="card" aria-labelledby="reset-title">
      <div className="card-head">
        <div>
          <h2 id="reset-title">Reset to factory</h2>
          <p className="hint" style={{ margin: "3px 0 0" }}>
            This resets DPI, polling, and button remapping on the selected device.
          </p>
        </div>
        {children ?? (
          <button
            type="button"
            className="button"
            disabled={!ready}
            onClick={onRequestReset}
          >
            Reset to factory
          </button>
        )}
      </div>

      {confirmation && (
        <div
          role="alertdialog"
          aria-label="Confirm factory reset"
          className="group"
          style={{ marginTop: 14 }}
        >
          <p style={{ margin: 0 }}>
            This resets DPI, polling, and button remapping on the selected device.
          </p>
          <div className="actions">
            <button type="button" className="button" onClick={onCancel}>
              Cancel factory reset
            </button>
            <button type="button" className="button primary" onClick={onConfirm}>
              Confirm factory reset
            </button>
          </div>
        </div>
      )}
    </article>
  );
}
