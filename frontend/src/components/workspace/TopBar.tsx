import type { ReactNode } from "react";
import { Window } from "@wailsio/runtime";
import { useTranslation } from "react-i18next";

export function TopBar({
  title,
  subtitle = "Attack Shark X6",
  children,
  center,
}: {
  title?: string;
  subtitle?: string;
  children?: ReactNode;
  center?: ReactNode;
}) {
  const { t } = useTranslation();
  return (
    <header className={`titlebar top-bar${center ? ' top-bar-centered' : ''}`}>
      <div className="appname brand">
        <span className="app-icon" aria-hidden="true">✦</span>
        <span className="brand-title">{title ?? t("topBar.title")}</span>
        {subtitle && <small>{subtitle}</small>}
      </div>
      {center && <div className="top-bar-center">{center}</div>}
      <div className="top-bar-actions">
        <div className="top-bar-children">{children}</div>
        <div className="window-controls" aria-label={t("topBar.windowControls")}>
          <button type="button" className="window-control" aria-label={t("topBar.minimise")} onClick={() => { void Window.Minimise(); }}>
            <span aria-hidden="true">−</span>
          </button>
          <button type="button" className="window-control" aria-label={t("topBar.close")} onClick={() => { void Window.Close(); }}>
            <span aria-hidden="true">×</span>
          </button>
        </div>
      </div>
    </header>
  );
}
