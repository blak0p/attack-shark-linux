import { useEffect, useRef, useState } from "react";
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

export function UnifiedUpdates({ info, checking, applying, error, onCheck, onApply }: {
  info?: UnifiedUpdateInfo; checking: boolean; applying: boolean; error: string;
  onCheck(): void; onApply(): void;
}) {
  const failed = !!(error || info?.AppError || info?.PolicyError || info?.RelaunchError);
  const unfinished = !!info && actionable(info);
  const completed = !!info?.PolicyInstalled && !failed && !unfinished && !applying;
  const [dismissed, setDismissed] = useState(false);
  const deadline = useRef<number | null>(null);
  useEffect(() => {
    // Only genuinely unfinished work resets a completed notice, not fresh check objects.
    if (unfinished || failed || applying) {
      deadline.current = null;
      setDismissed(false);
      return;
    }
    if (!completed || checking || dismissed) return;
    deadline.current ??= Date.now() + 3000;
    const timer = setTimeout(() => setDismissed(true), Math.max(0, deadline.current - Date.now()));
    return () => clearTimeout(timer);
  }, [unfinished, failed, applying, completed, checking, dismissed]);
  if (!failed && !applying && (!info || (!unfinished && !info.PolicyInstalled) || (completed && dismissed && !checking))) return null;
  const appTarget = !!info && (info.AppAvailable || info.AppInstalled || !!info.AppError);
  const policyTarget = !!info && (policyEligible(info) || info.PolicyInstalled || !!info.PolicyError);
  return <section className="update-window" aria-label="Updates">
    <article className="card unified-updates" aria-labelledby="updates-title">
      <h2 id="updates-title" tabIndex={-1}>Updates</h2>
      <p>{appTarget && policyTarget ? "AppImage and USB permissions (udev)" : appTarget ? "AppImage" : policyTarget ? "USB permissions (udev)" : "Update check failed"}</p>
      {checking && <p role="status">Checking for updates…</p>}
      {applying && <p role="status">Applying updates…</p>}
      {error && <p role="alert">Update request failed: {error}</p>}
      {info && <>
        {info.AppInstalled && <p>{info.RelaunchError ? "AppImage replaced; relaunch failed." : info.PolicyError
          ? "AppImage replaced; restart may be deferred. Use Relaunch app explicitly."
          : "AppImage replaced; restart requested, not confirmed."}</p>}
        {info.AppError && <p role="alert">AppImage: {info.AppError}</p>}
        {info.PolicyInstalled && <p>{completed ? "USB permissions updated." : info.PolicyError ? "USB policy written; reload or activation failed."
          : "USB policy written and reload completed; reconnect the mouse if needed."}</p>}
        {info.PolicyError && <p role="alert">USB permissions: {info.PolicyError}</p>}
        {info.RelaunchError && <p role="alert">Relaunch: {info.RelaunchError}</p>}
        {info.PolicyError && <p><a href="https://github.com/blak0p/attack-shark-linux/blob/main/docs/linux-usb-prerequisites.md#one-time-authenticated-usb-permissions-setup" target="_blank" rel="noreferrer">Trusted USB permissions setup guidance</a></p>}
      </>}
      <details>
        <summary>Update details</summary>
        {info && <>
          {info.AppAvailable && !info.AppInstalled && <p>AppImage version {info.Version} is available.</p>}
          <p>{policyMessages[info.PolicyState] ?? "USB permissions status is unknown."}</p>
        </>}
        <p>Only missing or recognized official outdated USB policies are eligible. Custom rules are preserved. USB changes use one system authorization dialog; this app never collects a password. AppImage replacement stays user-local. Setup is separate and user-led; the app never installs the helper.</p>
        {info && actionable(info) && <p>Apply updates retries unfinished eligible operations only. A written policy is not written again; a reload error still needs inspection. Relaunch app requests a restart, not a second installation.</p>}
      </details>
      <div className="update-actions">
        {info && actionable(info) && <button type="button" className="button primary" disabled={checking || applying} onClick={onApply}>
          {info.AppInstalled && !policyEligible(info) ? "Relaunch app" : "Apply updates"}
        </button>}
        <button type="button" className="button" disabled={checking || applying} onClick={onCheck}>Check for updates</button>
      </div>
    </article>
  </section>;
}
