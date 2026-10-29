import { useEffect, useRef, useState, type ReactNode } from "react";
import { createPortal } from 'react-dom';
import { useTranslation } from "react-i18next";
import { i18n } from "../i18n";

let feedbackHost: HTMLDivElement | undefined;
let feedbackUsers = 0;
// Both workspace and profile notifications share one bottom-right stack.
export function FeedbackPortal({ children }: { children: ReactNode }) {
  const [host, setHost] = useState<HTMLDivElement>();
  useEffect(() => {
    if (!feedbackHost) {
      feedbackHost = document.createElement('div');
      feedbackHost.className = 'operation-feedback';
      feedbackHost.setAttribute('role', 'region');
      feedbackHost.setAttribute('aria-label', i18n.t('feedback.region'));
      feedbackHost.setAttribute('aria-live', 'polite');
      document.body.appendChild(feedbackHost);
    }
    feedbackUsers++;
    setHost(feedbackHost);
    return () => {
      if (--feedbackUsers === 0) { feedbackHost?.remove(); feedbackHost = undefined; }
    };
  }, []);
  return host ? createPortal(children, host) : null;
}

export type FeedbackResult = {
  id: string;
  label: string;
  version: string;
  success?: string;
  error?: string;
  retry?: () => void;
  initialized?: boolean;
  saveFailed?: boolean;
};

export function ProgressIndicator({ label }: { label?: string }) {
  const { t } = useTranslation();
  return <><span className="pending-spinner" aria-hidden="true" /><span>{label ?? t("feedback.working")}</span></>;
}

// Presentation only. A snapshot is not an operation: seed initial versions silently.
export function OperationFeedback({ scope, busy = false, quiet = false, results }: {
  scope: string;
  busy?: boolean;
  // Another operation announces its own outcome; settings it refreshes must not confirm a second time.
  quiet?: boolean;
  results: FeedbackResult[];
}) {
  const { t } = useTranslation();
  const previous = useRef<{ scope: string; results: Map<string, FeedbackResult> } | undefined>(undefined);
  const serial = useRef(0);
  const [confirmation, setConfirmation] = useState<{ id: number; scope: string; message: string }>();
  const signature = JSON.stringify(results.map(({ id, version, success, error, initialized }) => [id, version, success, error, initialized]));
  const [expiredFailures, setExpiredFailures] = useState<Set<string>>(new Set());
  const failureKey = (result: FeedbackResult) => JSON.stringify([scope, result.id, result.version, result.error]);
  const failures = results.filter(result => result.error && !expiredFailures.has(failureKey(result)));
  const failureSignature = JSON.stringify(results.filter(result => result.error).map(failureKey));
  useEffect(() => {
    const timers = results.filter(result => result.error && !expiredFailures.has(failureKey(result))).map(result => {
      const key = failureKey(result);
      return window.setTimeout(() => setExpiredFailures(current => new Set([...current, key])), 8000);
    });
    return () => timers.forEach(window.clearTimeout);
  }, [scope, failureSignature]);
  const wasBusy = useRef(false);
  useEffect(() => {
    if (busy && !wasBusy.current) setConfirmation(undefined);
    wasBusy.current = busy;
  }, [busy]);
  useEffect(() => {
    const prior = previous.current;
    const changed = prior?.scope === scope
      ? results.filter(result => prior.results.has(result.id) && prior.results.get(result.id)?.version !== result.version)
      : [];
    previous.current = { scope, results: new Map(results.filter(result => result.initialized !== false).map(result => [result.id, result])) };
    if (prior?.scope !== scope || changed.some(result => result.error)) setConfirmation(undefined);
    const completed = changed.filter(result => result.success && !result.error).at(-1);
    if (completed && !quiet) setConfirmation({
      id: ++serial.current, scope, message: prior?.results.get(completed.id)?.saveFailed ? t("feedback.saved") : completed.success!,
    });
  }, [scope, signature]);

  useEffect(() => {
    if (!confirmation || busy) return;
    const timer = window.setTimeout(() => setConfirmation(undefined), 3000);
    return () => window.clearTimeout(timer);
  }, [confirmation, busy]);

  const message = confirmation?.scope === scope ? confirmation.message : undefined;
  return <FeedbackPortal>
    {(busy || message) && <div key={busy ? "pending" : confirmation?.id} className="feedback-message" role="status" aria-live="polite" aria-label={t("feedback.status")}>
      {busy ? <ProgressIndicator /> : <><span aria-hidden="true">✓</span><span>{message}</span></>}
    </div>}
    {groupFailures(failures).map(group => <div key={group.map(result => result.id).join()} role="alert" className="feedback-message feedback-error">
      <span><b>{group.map(result => result.label).join(", ")}</b>: {group[0].error}</span>
      {group[0].retry && <button type="button" className="button" disabled={busy} onClick={group[0].retry}>{t("feedback.retry")}</button>}
    </div>)}
  </FeedbackPortal>;
}

// One failure often marks several settings at once (a shared hardware write); show it once.
// Retryable failures stay separate because each retry acts on its own setting.
function groupFailures(failures: FeedbackResult[]): FeedbackResult[][] {
  const groups: FeedbackResult[][] = [];
  for (const failure of failures) {
    const group = failure.retry ? undefined : groups.find(candidate => !candidate[0].retry && candidate[0].error === failure.error);
    if (group) group.push(failure); else groups.push([failure]);
  }
  return groups;
}

export function settingsResult(id: string, label: string, snapshot: {
  Revision: number; Firmware?: string; Persistence?: string; Error: { Code: string }; RetryAvailable?: boolean; Applied?: unknown;
} | null | undefined, feedbackFor: (code: string) => string, retry?: () => void): FeedbackResult {
  const error = snapshot?.Firmware === "failed"
    ? feedbackFor(snapshot.Error.Code || "application_failed")
    : snapshot?.Persistence === "failed"
      ? i18n.t("feedback.saveFailed")
      : snapshot?.Error.Code ? feedbackFor(snapshot.Error.Code) : undefined;
  return {
    id, label,
    version: JSON.stringify([snapshot?.Applied, snapshot?.Firmware, snapshot?.Persistence, snapshot?.Error.Code]),
    success: snapshot?.Firmware === "success" ? i18n.t("feedback.applied") : undefined,
    initialized: !!snapshot,
    saveFailed: snapshot?.Firmware === "success" && snapshot?.Persistence === "failed",
    error, retry: snapshot?.RetryAvailable ? retry : undefined,
  };
}
