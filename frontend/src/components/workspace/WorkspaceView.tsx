import { useContext, useEffect, useRef, type ReactNode } from "react";
import { WorkspaceViewContext, type WorkspaceViewId } from "./workspace-view-context";

export function WorkspaceView({
  id,
  title,
  subtitle,
  placeholder,
  children,
}: {
  id: WorkspaceViewId;
  title: string;
  subtitle?: string;
  placeholder?: ReactNode;
  children: ReactNode;
}) {
  const { activeView } = useContext(WorkspaceViewContext);
  const heading = useRef<HTMLHeadingElement>(null);
  const active = activeView === id;

  useEffect(() => {
    if (active) heading.current?.focus();
  }, [active]);

  return (
    <section
      id={`workspace-view-${id}`}
      className={`view workspace-view${active ? " active" : ""}`}
      data-active={active}
      aria-label={title}
      aria-labelledby={`workspace-view-${id}-title`}
    >
      <div className="heading">
        <div>
          <h1 id={`workspace-view-${id}-title`} ref={heading} tabIndex={-1}>
            {title}
          </h1>
          {subtitle && <p>{subtitle}</p>}
        </div>
        {placeholder && <span className="placeholder">{placeholder}</span>}
      </div>
      {children}
    </section>
  );
}
