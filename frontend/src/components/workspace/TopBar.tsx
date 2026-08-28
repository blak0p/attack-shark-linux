import type { ReactNode } from "react";

export function TopBar({ children }: { children: ReactNode }) {
  return <header className="top-bar"><div className="brand"><p className="eyebrow">Attack Shark X6</p><h1>Device control</h1></div>{children}</header>;
}
