import { useContext } from "react";
import { useTranslation } from "react-i18next";
import { GamesAvailableContext, type WorkspaceViewId } from "./workspace-view-context";

const sections: Array<{ id: WorkspaceViewId; labelKey: string; glyph: string }> = [
  { id: "performance", labelKey: "nav.performance", glyph: "◉" },
  { id: "lighting", labelKey: "nav.lighting", glyph: "☼" },
  { id: "controls", labelKey: "nav.controls", glyph: "⌁" },
  { id: "remapping", labelKey: "nav.remapping", glyph: "↺" },
  { id: "macros", labelKey: "nav.macros", glyph: "≡" },
  { id: "games", labelKey: "nav.games", glyph: "▶" },
  { id: "device", labelKey: "nav.device", glyph: "▣" },
];

export function SectionNavigation({ activeView, onNavigate }: { activeView: WorkspaceViewId; onNavigate(view: WorkspaceViewId): void }) {
  const { t } = useTranslation();
  const gamesAvailable = useContext(GamesAvailableContext);
  return (
    <nav aria-label={t("nav.sections")}>
      {sections.filter((section) => section.id !== "games" || gamesAvailable).map((section) => (
        <a
          key={section.id}
          href={`#workspace-view-${section.id}`}
          className={`nav${activeView === section.id ? " active" : ""}`}
          data-view={section.id}
          aria-controls={`workspace-view-${section.id}`}
          aria-current={activeView === section.id ? "page" : undefined}
          onClick={(event) => {
            event.preventDefault();
            onNavigate(section.id);
          }}
        >
          <span className="glyph" aria-hidden="true">{section.glyph}</span>
          {t(section.labelKey)}
        </a>
      ))}
    </nav>
  );
}
