import type { ReactNode } from "react";
import type { DeviceInventory } from "../../desktop-contract";

export type DeviceStatusPanelProps = {
  connectionType?: string;
  battery?: number | null;
  serial?: string;
  inventory?: DeviceInventory | null;
  errorCode?: string;
  feedbackFor?: (code: string) => string;
  onSelectDevice?: (serial: string) => void;
  children?: ReactNode;
};

export function DeviceStatusPanel({
  connectionType,
  battery,
  serial,
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
            <strong>Interface</strong>
            <span>Active physical interface</span>
          </label>
          <b>{connectionType || "unavailable"}</b>
        </div>

        <div className="row">
          <label>
            <strong>Battery charge</strong>
            <span>Remaining battery capacity</span>
          </label>
          <b>{battery != null ? `${battery}%` : "unavailable"}</b>
        </div>

        <div className="row">
          <label>
            <strong>Serial identifier</strong>
            <span>Hardware identity</span>
          </label>
          <b>{serial || "unavailable"}</b>
        </div>
      </div>
    </article>
  );
}
