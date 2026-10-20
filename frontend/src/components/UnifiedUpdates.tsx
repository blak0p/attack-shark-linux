import type { UnifiedUpdateInfo } from "../desktop-contract";

const policyMessages: Record<string, string> = {
  current: "USB permissions are current.",
  missing: "USB permissions are missing.",
  outdated: "An official USB permissions update is available.",
  custom: "Custom USB permissions preserved; no automatic overwrite.",
  unavailable: "USB permissions update unavailable for this release.",
  error: "USB permissions could not be inspected.",
};

const policyEligible = (info: UnifiedUpdateInfo) => !info.PolicyInstalled &&
  (info.PolicyState === "missing" || info.PolicyState === "outdated");
const actionable = (info: UnifiedUpdateInfo) => policyEligible(info) ||
  (info.AppAvailable && !info.AppInstalled) || info.AppInstalled;

export function UnifiedUpdateNotice({ info, error, onReview }: {
  info?: UnifiedUpdateInfo; error: string; onReview(): void;
}) {
  if (!error && (!info || (!actionable(info) && !info.AppError && !info.PolicyError && !info.RelaunchError))) return null;
  return <aside className="update-banner" role="status" aria-live="polite">
    <span>AppImage and USB permissions: review update status.</span>
    <button type="button" className="button" onClick={onReview}>Review updates</button>
  </aside>;
}

export function UnifiedUpdates({ info, checking, applying, error, onCheck, onApply }: {
  info?: UnifiedUpdateInfo; checking: boolean; applying: boolean; error: string;
  onCheck(): void; onApply(): void;
}) {
  return <article className="card unified-updates" aria-labelledby="updates-title">
    <h2 id="updates-title" tabIndex={-1}>Updates</h2>
    {checking && <p role="status">Checking for updates…</p>}
    {!info && !checking && <p>Update status is unknown. Check before applying.</p>}
    {error && <p role="alert">Update request failed: {error}</p>}
    {info && <>
      <h3>AppImage</h3>
      <p>{info.AppInstalled
        ? info.RelaunchError ? "AppImage replaced; relaunch failed." : info.PolicyError
          ? "AppImage replaced; restart may be deferred. Use Relaunch app explicitly."
          : "AppImage replaced; restart requested, not confirmed."
        : info.AppAvailable ? `Version ${info.Version} is available.`
          : info.AppError ? "AppImage update status could not be confirmed." : "No newer AppImage available."}</p>
      {info.AppError && <p role="alert">AppImage: {info.AppError}</p>}
      <h3>USB permissions</h3>
      <p>{info.PolicyInstalled
        ? info.PolicyError ? "USB policy written; reload or activation failed." : "USB policy written and reload completed; reconnect the mouse if needed."
        : policyMessages[info.PolicyState] ?? "USB permissions status is unknown."}</p>
      {info.PolicyError && <p role="alert">USB permissions: {info.PolicyError}</p>}
      {info.RelaunchError && <p role="alert">Relaunch: {info.RelaunchError}</p>}
      {info.PolicyError && <p><a href="https://github.com/blak0p/attack-shark-linux/blob/main/docs/linux-usb-prerequisites.md#one-time-authenticated-usb-permissions-setup" target="_blank" rel="noreferrer">Trusted USB permissions setup guidance</a>. Setup is separate and user-led; the app never installs the helper.</p>}
    </>}
    <p>Only missing or recognized official outdated USB policies are eligible. Custom rules are preserved. USB changes use one system authorization dialog; this app never collects a password. AppImage replacement stays user-local.</p>
    <div className="update-actions">
      <button type="button" className="button" disabled={checking || applying} onClick={onCheck}>Check for updates</button>
      {info && actionable(info) && <button type="button" className="button primary" disabled={checking || applying} onClick={onApply}>
        {info.AppInstalled && !policyEligible(info) ? "Relaunch app" : "Apply updates"}
      </button>}
    </div>
    {info && actionable(info) && <p>Apply updates retries unfinished eligible operations only. A written policy is not written again; a reload error still needs inspection. Relaunch app requests a restart, not a second installation.</p>}
  </article>;
}
