import type { UpdateInfo } from "../desktop-contract";

export function UpdateBanner({ update, applying, error, onApply }: { update: UpdateInfo; applying: boolean; error: string; onApply(): void }) {
  return <aside className="update-banner" role="status" aria-live="polite">
    <span>Version {update.Version} is ready.</span>
    <button type="button" className="button" disabled={applying} onClick={onApply}>
      {applying ? "Updating…" : "Update and restart"}
    </button>
    {error && <span role="alert">Update failed: {error}</span>}
  </aside>;
}
