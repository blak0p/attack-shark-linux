import { useContext, useEffect, useRef, type ReactNode } from "react";
import { WorkspaceViewContext, type WorkspaceViewId } from "./workspace-view-context";

export function WorkspaceView({ id, title, children }: { id: WorkspaceViewId; title: string; children: ReactNode }) {
  const { activeView } = useContext(WorkspaceViewContext);
  const heading = useRef<HTMLHeadingElement>(null);
  const active = activeView === id;

  useEffect(() => {
    if (active) heading.current?.focus();
  }, [active]);

  return <section id={`workspace-view-${id}`} className="workspace-view" data-active={active} aria-labelledby={`workspace-view-${id}-title`}>
    <h2 id={`workspace-view-${id}-title`} ref={heading} tabIndex={-1}>{title}</h2>
    {children}
  </section>;
}
