import { useState, type ReactNode } from "react";
import { SectionNavigation } from "./SectionNavigation";
import { TopBar } from "./TopBar";
import { WorkspaceViewContext, type WorkspaceViewId } from "./workspace-view-context";

export type WorkspaceShellProps = {
  children: ReactNode;
  busy?: boolean;
  activeView?: WorkspaceViewId;
  onNavigate?: (view: WorkspaceViewId) => void;
  deviceName?: string;
  deviceSubtitle?: string;
  connectionStatus?: ReactNode;
  titlebar?: ReactNode;
};

export function WorkspaceShell({
  children,
  busy = false,
  activeView: activeViewProp,
  onNavigate: onNavigateProp,
  deviceName = "Attack Shark X6",
  deviceSubtitle = "Gaming Mouse",
  connectionStatus,
  titlebar,
}: WorkspaceShellProps) {
  const [internalActiveView, setInternalActiveView] = useState<WorkspaceViewId>("performance");
  const currentView = activeViewProp ?? internalActiveView;

  const handleNavigate = (view: WorkspaceViewId) => {
    setInternalActiveView(view);
    onNavigateProp?.(view);
  };

  const reducedMotion = window.matchMedia?.("(prefers-reduced-motion: reduce)").matches ?? false;

  return (
    <div className="window workspace-shell" aria-busy={busy}>
      <div className="workspace-shell-content" inert={busy ? true : undefined}>
        {titlebar ?? <TopBar />}
        <aside className="workspace-rail" aria-label="Workspace sections">
          <div className="device-name">
            <strong>{deviceName}</strong>
            <span>{deviceSubtitle}</span>
          </div>
          <SectionNavigation activeView={currentView} onNavigate={handleNavigate} />
          {connectionStatus ?? (
            <div className="connection">
              <b><span className="dot" aria-hidden="true" />Device available</b>
              <span>Connected via receiver</span>
            </div>
          )}
        </aside>
        <main className="main" data-motion={reducedMotion ? "reduced" : "full"}>
          <WorkspaceViewContext value={{ activeView: currentView }}>
            {children}
          </WorkspaceViewContext>
        </main>
      </div>
      {busy && (
        <div
          className="configuration-overlay"
          role="status"
          aria-label="Applying configuration…"
          aria-live="polite"
        >
          Applying configuration…
        </div>
      )}
    </div>
  );
}
