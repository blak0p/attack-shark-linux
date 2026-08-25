import { Events } from "@wailsio/runtime";
import * as bindings from "../bindings/github.com/blak0p/attack-shark-linux/internal/desktop/service";
import type { DesktopService } from "./App";

export const desktopService: DesktopService = {
  GetSnapshot: bindings.GetSnapshot,
  GetPollingSnapshot: bindings.GetPollingSnapshot,
  GetLightingSnapshot: bindings.GetLightingSnapshot,
  RefreshStatus: bindings.RefreshStatus,
  RefreshInventory: bindings.RefreshInventory,
  SelectDevice: bindings.SelectDevice,
  StageDPI: bindings.StageDPI as DesktopService["StageDPI"],
  StagePollingRate: bindings.StagePollingRate as DesktopService["StagePollingRate"],
  StageLighting: bindings.StageLighting as DesktopService["StageLighting"],
  ApplyLighting: bindings.ApplyLighting,
  RetryPollingPersistence: bindings.RetryPollingPersistence,
  ResetToFactory: bindings.ResetToFactory,
  RetryPersistence: bindings.RetryPersistence,
  OnStatusEvent: (callback) => Events.On("mouse:status", (event) => callback(event.data)),
	OnConfiguration: (callback) => Events.On("mouse:configuration", (event) => callback(event.data)),
	OnPollingConfiguration: (callback) => Events.On("mouse:polling-configuration", (event) => callback(event.data)),
};
