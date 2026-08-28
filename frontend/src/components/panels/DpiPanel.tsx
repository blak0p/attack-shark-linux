import type { ReactNode } from "react";
export function DpiPanel({ children }: { children: ReactNode }) { return <section id="dpi" className="panel" aria-labelledby="dpi-title">{children}</section>; }
