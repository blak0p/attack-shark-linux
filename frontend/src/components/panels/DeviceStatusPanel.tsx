import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
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
  const { t } = useTranslation();
  if (children) {
    return (
      <article className="card device-status">
        {children}
      </article>
    );
  }

  return (
    <article className="card device-status">
      <h2>{t("status.title")}</h2>
      <div className="rows">
        <div className="row">
          <label>
            <strong>{t("status.version")}</strong>
          </label>
          <b>{applicationVersion?.trim() || t("status.devBuild")}</b>
        </div>
        <div className="row">
          <label>
            <strong>{t("status.interface")}</strong>
          </label>
          <b>{connectionType || t("app.unavailable")}</b>
        </div>

        <div className="row">
          <label>
            <strong>{t("status.battery")}</strong>
          </label>
          <b>{battery != null ? `${battery}%` : t("app.unavailable")}</b>
        </div>

        <div className="row">
          <label>
            <strong>{t("status.serial")}</strong>
          </label>
          <b>{serial || t("app.unavailable")}</b>
        </div>
      </div>
      {(errorCode === "permission_denied" || errorCode === "device_disconnected") && feedbackFor && (
        <span role="alert">{feedbackFor(errorCode)}</span>
      )}
    </article>
  );
}
