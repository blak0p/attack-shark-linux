import { Events } from "@wailsio/runtime";
import * as bindings from "../bindings/github.com/alejandro/attack-shark-linux/internal/desktop/service";
import type { DesktopService } from "./App";

export const desktopService: DesktopService = {
  GetSnapshot: bindings.GetSnapshot,
  RefreshStatus: bindings.RefreshStatus,
  StageDPI: bindings.StageDPI as DesktopService["StageDPI"],
  ApplyDPI: bindings.ApplyDPI,
  OnStatusEvent: (callback) => Events.On("x6:status", (event) => callback(event.data)),
};
