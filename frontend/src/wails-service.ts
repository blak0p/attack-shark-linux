import { Events } from "@wailsio/runtime";
import * as bindings from "../bindings/github.com/alejandro/attack-shark-linux/internal/desktop/service";
import type { DesktopService } from "./App";

export const desktopService: DesktopService = {
  GetSnapshot: bindings.GetSnapshot,
  RefreshStatus: bindings.RefreshStatus,
  RefreshInventory: bindings.RefreshInventory,
  SelectDevice: bindings.SelectDevice,
  StageDPI: bindings.StageDPI as DesktopService["StageDPI"],
  RetryPersistence: bindings.RetryPersistence,
  OnStatusEvent: (callback) => Events.On("mouse:status", (event) => callback(event.data)),
	OnConfiguration: (callback) => Events.On("mouse:configuration", (event) => callback(event.data)),
};
