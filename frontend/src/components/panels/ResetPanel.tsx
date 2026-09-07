import type { ReactNode } from "react";
export function ResetPanel({ children, confirmation, onConfirm, onCancel }: { children: ReactNode; confirmation: boolean; onConfirm(): void; onCancel(): void }) {
  return <section id="reset" className="panel-section">
    {children}
    {confirmation && <div role="alertdialog" aria-label="Confirm factory reset">
      <p>This resets DPI, polling, and button remapping on the selected device.</p>
      <button type="button" onClick={onConfirm}>Confirm factory reset</button>
      <button type="button" onClick={onCancel}>Cancel factory reset</button>
    </div>}
  </section>;
}
