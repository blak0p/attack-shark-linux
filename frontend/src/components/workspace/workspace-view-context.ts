import { createContext } from "react";

export type WorkspaceViewId = "performance" | "lighting" | "controls" | "remapping" | "macros" | "games" | "device";

export const WorkspaceViewContext = createContext<{ activeView: WorkspaceViewId }>({ activeView: "performance" });

/** True when the desktop service can persist game rules; gates the Games section. */
export const GamesAvailableContext = createContext(false);
