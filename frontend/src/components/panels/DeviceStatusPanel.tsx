import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import type { InstallSource, Inventory } from "../../desktop-contract";

export type DeviceStatusPanelProps = {
  connectionType?: string;
  applicationVersion?: string;
  installSource?: InstallSource;
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
  installSource,
  battery,
  serial,
  errorCode,
  feedbackFor,
  children,
}: DeviceStatusPanelProps) {
  const { t } = useTranslation();
  const managedBy = installSource?.PackageManager;
  // Branch builds (-git packages) are identified by branch and commit, not a version.
  const build = installSource?.Branch && installSource.Commit
    ? `${installSource.Branch} · ${installSource.Commit}`
    : applicationVersion?.trim();
  const version = build && installSource?.Distribution ? `${build} · ${installSource.Distribution}` : build;
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
          <b>{version || t("status.devBuild")}</b>
        </div>
        {managedBy && (
          <div className="row">
            <label>
              <strong>{t("status.updates")}</strong>
            </label>
            <b>{t("status.updatesManagedBy", { manager: managedBy })}</b>
          </div>
        )}
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
