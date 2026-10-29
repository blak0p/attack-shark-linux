import type { WorkspaceModel } from "../desktop-contract";
import { useTranslation } from "react-i18next";
import { libraryNoticeKeys } from "../hooks/useMacroLibrary";
import { workspaceNoticeKeys } from "../hooks/useDesktopWorkspace";
import { OperationFeedback, settingsResult } from "./OperationFeedback";

export function WorkspaceFeedback({ model, library, feedbackFor, retries, initializing = false, profileBusy = false }: {
  model: WorkspaceModel;
  initializing?: boolean;
  profileBusy?: boolean;
  library: { loading: boolean; reading: boolean; busy: boolean; error: string; notice: string };
  feedbackFor(code: string): string;
  retries: { dpi(): void; polling(): void; debounce(): void; sleep(): void };
}) {
  const { t } = useTranslation();
  const { snapshot, polling, debounce, normalSleep, lighting, remap, notice } = model;
  const remapResult = settingsResult("remap", t("feedback.labels.buttons"), remap, feedbackFor);
  remapResult.version = JSON.stringify([remap?.Applied, remap?.MacroAppliedDrafts ?? remap?.MacroApplied, remap?.Firmware, remap?.Persistence, remap?.Error.Code]);
  // Staging is not an apply, even when the last firmware result was successful.
  const drafts = remap?.MacroDrafts ?? (remap?.MacroPending ? { [remap.MacroPending.Button]: remap.MacroPending } : {});
  const appliedDrafts = remap?.MacroAppliedDrafts ?? (remap?.MacroApplied ? { [remap.MacroApplied.Button]: remap.MacroApplied } : {});
  const draftsMatch = remap && JSON.stringify(remap.Pending) === JSON.stringify(remap.Applied)
    && JSON.stringify(drafts) === JSON.stringify(appliedDrafts);
  remapResult.success = draftsMatch && remap?.Firmware === "success" ? t("feedback.applied") : undefined;
  if (Object.keys(appliedDrafts).length > 0 && remap?.MacroProgress && (remap.MacroProgress.Assignment > 0 || remap.MacroProgress.Upload > 0) && remap.Firmware === "success" && draftsMatch) remapResult.success = t("feedback.sent");
  if (model.remapError) remapResult.error = feedbackFor(model.remapError);
  if (remap?.Firmware === "failed" && remap.MacroProgress) remapResult.error = `${remapResult.error ?? t("feedback.uploadFailed")} ${t("feedback.devicePartial")}`;
  const resetError = notice.startsWith(t(workspaceNoticeKeys.factoryResetFailedPrefix)) || notice.startsWith(t(workspaceNoticeKeys.deviceSelectionFailedPrefix));
  return <OperationFeedback
    scope={JSON.stringify(model.inventory?.Selected ?? null)}
    quiet={profileBusy}
    busy={initializing || model.automaticApplyBusy || model.remapBusy || library.busy || library.loading || library.reading || model.updateApplying
      || [snapshot, polling, debounce, normalSleep, lighting].some(value => value?.Firmware === "pending")}
    results={[
      settingsResult("dpi", t("feedback.labels.mouseSettings"), snapshot && { ...snapshot, Error: { Code: model.inventory?.Error.Code || snapshot.Error.Code } }, feedbackFor, retries.dpi),
      settingsResult("polling", t("feedback.labels.pollingRate"), polling, feedbackFor, retries.polling),
      settingsResult("debounce", t("feedback.labels.keyResponse"), debounce, feedbackFor, retries.debounce),
      settingsResult("sleep", t("feedback.labels.sleep"), normalSleep, feedbackFor, retries.sleep),
      settingsResult("lighting", t("feedback.labels.lighting"), lighting, feedbackFor),
      remapResult,
      { id: "library", label: t("feedback.labels.macros"), version: library.notice, error: library.error,
        success: library.notice === t(libraryNoticeKeys.saved) ? t("feedback.saved")
          : library.notice === t(libraryNoticeKeys.imported) ? t("feedback.imported")
          : library.notice === t(libraryNoticeKeys.deleted) ? t("feedback.deleted") : undefined },
      { id: "reset", label: t("feedback.labels.device"), version: notice, error: resetError ? notice : undefined,
        success: notice === t(workspaceNoticeKeys.factoryResetCompleted) ? t("feedback.reset") : undefined },
    ]}
  />;
}
