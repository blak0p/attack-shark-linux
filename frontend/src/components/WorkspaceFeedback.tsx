import type { WorkspaceModel } from "../desktop-contract";
import { OperationFeedback, settingsResult } from "./OperationFeedback";

export function WorkspaceFeedback({ model, library, feedbackFor, retries, initializing = false }: {
  model: WorkspaceModel;
  initializing?: boolean;
  library: { loading: boolean; reading: boolean; busy: boolean; error: string; notice: string };
  feedbackFor(code: string): string;
  retries: { dpi(): void; polling(): void; debounce(): void; sleep(): void };
}) {
  const { snapshot, polling, debounce, normalSleep, lighting, remap, notice } = model;
  const remapResult = settingsResult("remap", "Buttons", remap, feedbackFor);
  remapResult.version = JSON.stringify([remap?.Applied, remap?.MacroAppliedDrafts ?? remap?.MacroApplied, remap?.Firmware, remap?.Persistence, remap?.Error.Code]);
  // Staging is not an apply, even when the last firmware result was successful.
  const drafts = remap?.MacroDrafts ?? (remap?.MacroPending ? { [remap.MacroPending.Button]: remap.MacroPending } : {});
  const appliedDrafts = remap?.MacroAppliedDrafts ?? (remap?.MacroApplied ? { [remap.MacroApplied.Button]: remap.MacroApplied } : {});
  const draftsMatch = remap && JSON.stringify(remap.Pending) === JSON.stringify(remap.Applied)
    && JSON.stringify(drafts) === JSON.stringify(appliedDrafts);
  remapResult.success = draftsMatch && remap?.Firmware === "success" ? "Applied" : undefined;
  if (Object.keys(appliedDrafts).length > 0 && remap?.MacroProgress && (remap.MacroProgress.Assignment > 0 || remap.MacroProgress.Upload > 0) && remap.Firmware === "success" && draftsMatch) remapResult.success = "Sent";
  if (model.remapError) remapResult.error = feedbackFor(model.remapError);
  if (remap?.Firmware === "failed" && remap.MacroProgress) remapResult.error = `${remapResult.error ?? "Upload failed."} Device state may be partial or unknown.`;
  const resetError = notice.startsWith("Factory reset failed:") || notice.startsWith("Device selection failed;");
  return <OperationFeedback
    scope={JSON.stringify(model.inventory?.Selected ?? null)}
    busy={initializing || model.automaticApplyBusy || model.remapBusy || library.busy || library.loading || library.reading || model.updateApplying
      || [snapshot, polling, debounce, normalSleep, lighting].some(value => value?.Firmware === "pending")}
    results={[
      settingsResult("dpi", "Mouse settings", snapshot && { ...snapshot, Error: { Code: model.inventory?.Error.Code || snapshot.Error.Code } }, feedbackFor, retries.dpi),
      settingsResult("polling", "Polling rate", polling, feedbackFor, retries.polling),
      settingsResult("debounce", "Key response time", debounce, feedbackFor, retries.debounce),
      settingsResult("sleep", "Sleep", normalSleep, feedbackFor, retries.sleep),
      settingsResult("lighting", "Lighting", lighting, feedbackFor),
      remapResult,
      { id: "library", label: "Macros", version: library.notice, error: library.error,
        success: library.notice === "Saved to library. No device changes made." ? "Saved"
          : library.notice === "Imported as a new local macro. No device changes made." ? "Imported"
          : library.notice === "Macro deleted from library." ? "Deleted" : undefined },
      { id: "reset", label: "Device", version: notice, error: resetError ? notice : undefined,
        success: notice === "Factory reset completed." ? "Reset" : undefined },
    ]}
  />;
}
