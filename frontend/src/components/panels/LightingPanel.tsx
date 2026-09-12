import type { ReactNode } from "react";
export function LightingPanel({ children }: { children: ReactNode }) {
  return <div id="lighting" className="stack">{children}</div>;
}
