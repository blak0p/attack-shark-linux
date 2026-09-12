import type { ReactNode } from "react";

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
      {children}
    </header>
  );
}
