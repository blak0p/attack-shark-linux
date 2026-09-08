import { useState, type ReactNode } from "react";
import { CollapsibleRail } from "./CollapsibleRail";
import { SectionNavigation } from "./SectionNavigation";
import { WorkspaceViewContext, type WorkspaceViewId } from "./workspace-view-context";

export function WorkspaceShell({ children, busy = false }: { children: ReactNode; busy?: boolean }) {
  const [activeView, setActiveView] = useState<WorkspaceViewId>("performance");
  const [navigationExpanded, setNavigationExpanded] = useState(true);
  const reducedMotion = window.matchMedia?.("(prefers-reduced-motion: reduce)").matches ?? false;
  return <div className="workspace-shell" aria-busy={busy}>
    <div className="workspace-shell-content" inert={busy}>
      <CollapsibleRail id="workspace-navigation-rail" label="navigation rail" expanded={navigationExpanded} onToggle={() => setNavigationExpanded((value) => !value)}>
        <SectionNavigation activeView={activeView} onNavigate={setActiveView} />
      </CollapsibleRail>
      <main className="main" data-motion={reducedMotion ? "reduced" : "full"}><WorkspaceViewContext value={{ activeView }}>{children}</WorkspaceViewContext></main>
    </div>
    {busy && <div className="configuration-overlay" role="status" aria-label="Applying configuration…" aria-live="polite">Applying configuration…</div>}
  </div>;
}
