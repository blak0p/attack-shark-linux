import { useEffect, useRef, useState, type ReactNode } from "react";
import { createPortal } from 'react-dom';

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
      feedbackHost.setAttribute('aria-label', 'Operation feedback');
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

export function ProgressIndicator({ label = "Working…" }: { label?: string }) {
  return <><span className="pending-spinner" aria-hidden="true" /><span>{label}</span></>;
}

// Presentation only. A snapshot is not an operation: seed initial versions silently.
export function OperationFeedback({ scope, busy = false, quiet = false, results }: {
  scope: string;
  busy?: boolean;
  // Another operation announces its own outcome; settings it refreshes must not confirm a second time.
  quiet?: boolean;
  results: FeedbackResult[];
}) {
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
      id: ++serial.current, scope, message: prior?.results.get(completed.id)?.saveFailed ? "Saved" : completed.success!,
    });
  }, [scope, signature]);

  useEffect(() => {
    if (!confirmation || busy) return;
    const timer = window.setTimeout(() => setConfirmation(undefined), 3000);
    return () => window.clearTimeout(timer);
  }, [confirmation, busy]);

  const message = confirmation?.scope === scope ? confirmation.message : undefined;
  return <FeedbackPortal>
    {(busy || message) && <div key={busy ? "pending" : confirmation?.id} className="feedback-message" role="status" aria-live="polite" aria-label="Operation status">
      {busy ? <ProgressIndicator /> : <><span aria-hidden="true">✓</span><span>{message}</span></>}
    </div>}
    {failures.map(result => <div key={result.id} role="alert" className="feedback-message feedback-error">
      <span><b>{result.label}</b>: {result.error}</span>
      {result.retry && <button type="button" className="button" disabled={busy} onClick={result.retry}>Retry</button>}
    </div>)}
  </FeedbackPortal>;
}

export function settingsResult(id: string, label: string, snapshot: {
  Revision: number; Firmware?: string; Persistence?: string; Error: { Code: string }; RetryAvailable?: boolean; Applied?: unknown;
} | null | undefined, feedbackFor: (code: string) => string, retry?: () => void): FeedbackResult {
  const error = snapshot?.Firmware === "failed"
    ? feedbackFor(snapshot.Error.Code || "application_failed")
    : snapshot?.Persistence === "failed"
      ? "Applied to mouse; local save failed."
      : snapshot?.Error.Code ? feedbackFor(snapshot.Error.Code) : undefined;
  return {
    id, label,
    version: JSON.stringify([snapshot?.Applied, snapshot?.Firmware, snapshot?.Persistence, snapshot?.Error.Code]),
    success: snapshot?.Firmware === "success" ? "Applied" : undefined,
    initialized: !!snapshot,
    saveFailed: snapshot?.Firmware === "success" && snapshot?.Persistence === "failed",
    error, retry: snapshot?.RetryAvailable ? retry : undefined,
  };
}
