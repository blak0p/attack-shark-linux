import type { ReactNode } from "react";
import { Window } from "@wailsio/runtime";

export function TopBar({
  title = "Mouse configuration",
  subtitle = "Attack Shark X6",
  children,
}: {
  title?: string;
  subtitle?: string;
  children?: ReactNode;
}) {
  return (
    <header className="titlebar top-bar">
      <div className="appname brand">
        <span className="app-icon" aria-hidden="true">✦</span>
        {title}
        {subtitle && <small>{subtitle}</small>}
      </div>
      <div className="top-bar-actions">
        <div className="top-bar-children">{children}</div>
        <div className="window-controls" aria-label="Window controls">
          <button type="button" className="window-control" aria-label="Minimise window" onClick={() => { void Window.Minimise(); }}>
            <span aria-hidden="true">−</span>
          </button>
          <button type="button" className="window-control" aria-label="Close window" onClick={() => { void Window.Close(); }}>
            <span aria-hidden="true">×</span>
          </button>
        </div>
      </div>
    </header>
  );
}
