import type { ReactNode } from "react";
import type { Inventory } from "../../desktop-contract";

export type DeviceStatusPanelProps = {
  connectionType?: string;
  applicationVersion?: string;
  battery?: number | null;
  serial?: string;
  inventory?: Inventory | null;
  errorCode?: string;
  feedbackFor?: (code: string) => string;
  onSelectDevice?: (serial: string) => void;
  children?: ReactNode;
};

export function DeviceStatusPanel({
  connectionType,
  applicationVersion,
  battery,
  serial,
  errorCode,
  feedbackFor,
  children,
}: DeviceStatusPanelProps) {
  if (children) {
    return (
      <article className="card device-status">
        {children}
      </article>
    );
  }

  return (
    <article className="card device-status">
      <h2>Status</h2>
      <div className="rows">
        <div className="row">
          <label>
            <strong>Application version</strong>
          </label>
          <b>{applicationVersion?.trim() || "development build"}</b>
        </div>
        <div className="row">
          <label>
            <strong>Interface</strong>
          </label>
          <b>{connectionType || "unavailable"}</b>
        </div>

        <div className="row">
          <label>
            <strong>Battery charge</strong>
          </label>
          <b>{battery != null ? `${battery}%` : "unavailable"}</b>
        </div>

        <div className="row">
          <label>
            <strong>Serial identifier</strong>
          </label>
          <b>{serial || "unavailable"}</b>
        </div>
      </div>
      {(errorCode === "permission_denied" || errorCode === "device_disconnected") && feedbackFor && (
        <span role="alert">{feedbackFor(errorCode)}</span>
      )}
    </article>
  );
}
