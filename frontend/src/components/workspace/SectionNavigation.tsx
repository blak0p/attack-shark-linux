import type { WorkspaceViewId } from "./workspace-view-context";

const sections: Array<{ id: WorkspaceViewId; label: string }> = [
  { id: "performance", label: "Performance" }, { id: "lighting", label: "Lighting" },
  { id: "controls", label: "Controls" }, { id: "device", label: "Device" },
];

export function SectionNavigation({ activeView, onNavigate }: { activeView: WorkspaceViewId; onNavigate(view: WorkspaceViewId): void }) {
  return <nav aria-label="Workspace sections">{sections.map((section) => <a key={section.id} href={`#workspace-view-${section.id}`} aria-controls={`workspace-view-${section.id}`} aria-current={activeView === section.id ? "page" : undefined} onClick={(event) => { event.preventDefault(); onNavigate(section.id); }}>{section.label}</a>)}</nav>;
}
