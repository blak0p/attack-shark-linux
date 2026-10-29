import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import type { UnifiedUpdateInfo } from "../desktop-contract";

const policyStates = ["current", "missing", "outdated", "custom", "unavailable", "error"];

const policyEligible = (info: UnifiedUpdateInfo) => !info.PolicyInstalled &&
  (info.PolicyState === "missing" || info.PolicyState === "outdated");
const actionable = (info: UnifiedUpdateInfo) => policyEligible(info) ||
  (info.AppAvailable && !info.AppInstalled) || info.AppInstalled;

export function UnifiedUpdates({ info, checking, applying, error, onCheck, onApply }: {
  info?: UnifiedUpdateInfo; checking: boolean; applying: boolean; error: string;
  onCheck(): void; onApply(): void;
}) {
  const { t } = useTranslation();
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
  return <section className="update-window" aria-label={t("updates.title")}>
    <article className="card unified-updates" aria-labelledby="updates-title">
      <h2 id="updates-title" tabIndex={-1}>{t("updates.title")}</h2>
      <p>{appTarget && policyTarget ? t("updates.target.both") : appTarget ? t("updates.target.app") : policyTarget ? t("updates.target.policy") : t("updates.target.failed")}</p>
      {checking && <p role="status">{t("updates.checking")}</p>}
      {applying && <p role="status">{t("updates.applying")}</p>}
      {error && <p role="alert">{t("updates.requestFailed", { error })}</p>}
      {info && <>
        {info.AppInstalled && <p>{info.RelaunchError ? t("updates.app.relaunchFailed") : info.PolicyError
          ? t("updates.app.restartDeferred")
          : t("updates.app.restartRequested")}</p>}
        {info.AppError && <p role="alert">{t("updates.app.error", { error: info.AppError })}</p>}
        {info.PolicyInstalled && <p>{completed ? t("updates.policy.updated") : info.PolicyError ? t("updates.policy.reloadFailed")
          : t("updates.policy.reloaded")}</p>}
        {info.PolicyError && <p role="alert">{t("updates.policy.error", { error: info.PolicyError })}</p>}
        {info.RelaunchError && <p role="alert">{t("updates.relaunchError", { error: info.RelaunchError })}</p>}
        {info.PolicyError && <p><a href="https://github.com/blak0p/attack-shark-linux/blob/main/docs/linux-usb-prerequisites.md#one-time-authenticated-usb-permissions-setup" target="_blank" rel="noreferrer">{t("updates.guidance")}</a></p>}
      </>}
      <details>
        <summary>{t("updates.details")}</summary>
        {info && <>
          {info.AppAvailable && !info.AppInstalled && <p>{t("updates.app.available", { version: info.Version })}</p>}
          <p>{policyStates.includes(info.PolicyState) ? t(`updates.policyState.${info.PolicyState}`) : t("updates.policyState.unknown")}</p>
        </>}
        <p>{t("updates.explainPolicy")}</p>
        {info && actionable(info) && <p>{t("updates.explainApply")}</p>}
      </details>
      <div className="update-actions">
        {info && actionable(info) && <button type="button" className="button primary" disabled={checking || applying} onClick={onApply}>
          {info.AppInstalled && !policyEligible(info) ? t("updates.relaunch") : t("updates.apply")}
        </button>}
        <button type="button" className="button" disabled={checking || applying} onClick={onCheck}>{t("updates.check")}</button>
      </div>
    </article>
  </section>;
}
