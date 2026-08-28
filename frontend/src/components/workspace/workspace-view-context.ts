import { createContext } from "react";

export type WorkspaceViewId = "performance" | "lighting" | "controls" | "device";

export const WorkspaceViewContext = createContext<{ activeView: WorkspaceViewId }>({ activeView: "performance" });
