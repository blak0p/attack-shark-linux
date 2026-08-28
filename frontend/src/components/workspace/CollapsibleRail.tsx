import type { KeyboardEvent, ReactNode } from "react";

export function CollapsibleRail({ id, label, expanded, onToggle, children }: { id: string; label: string; expanded: boolean; onToggle(): void; children: ReactNode }) {
  const toggleFromKeyboard = (event: KeyboardEvent<HTMLButtonElement>) => {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      onToggle();
    }
  };
  return <aside className={`workspace-rail${expanded ? "" : " is-collapsed"}`}><button type="button" aria-label={`Toggle ${label}`} aria-expanded={expanded} aria-controls={id} onClick={onToggle} onKeyDown={toggleFromKeyboard}>☰</button><div id={id} aria-label={label}>{children}</div></aside>;
}
