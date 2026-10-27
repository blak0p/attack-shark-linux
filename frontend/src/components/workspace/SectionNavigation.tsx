import { useContext } from "react";
import { GamesAvailableContext, type WorkspaceViewId } from "./workspace-view-context";

const sections: Array<{ id: WorkspaceViewId; label: string; glyph: string }> = [
  { id: "performance", label: "Performance", glyph: "◉" },
  { id: "lighting", label: "Lighting", glyph: "☼" },
  { id: "controls", label: "Controls", glyph: "⌁" },
  { id: "remapping", label: "Button remapping", glyph: "↺" },
  { id: "macros", label: "Macros", glyph: "≡" },
  { id: "games", label: "Games", glyph: "▶" },
  { id: "device", label: "Device", glyph: "▣" },
];

export function SectionNavigation({ activeView, onNavigate }: { activeView: WorkspaceViewId; onNavigate(view: WorkspaceViewId): void }) {
  const gamesAvailable = useContext(GamesAvailableContext);
  return (
    <nav aria-label="Workspace sections">
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
          {section.label}
        </a>
      ))}
    </nav>
  );
}
