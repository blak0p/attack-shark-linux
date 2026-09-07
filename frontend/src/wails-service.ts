import { Events } from "@wailsio/runtime";
import * as bindings from "../../cmd/x6configurator/frontend/bindings/github.com/blak0p/attack-shark-linux/internal/desktop/service";
import type { DesktopService } from "./desktop-contract";

type ExplicitApplyBindings = {
  ApplyDPI(): ReturnType<DesktopService["ApplyDPI"]>;
  ApplyPollingRate(): ReturnType<DesktopService["ApplyPollingRate"]>;
  ApplyRemap(config: Parameters<DesktopService["ApplyRemap"]>[0]): ReturnType<DesktopService["ApplyRemap"]>;
  ResetToFactory(): ReturnType<DesktopService["ResetToFactory"]>;
};
const explicitApplyBindings = bindings as typeof bindings & ExplicitApplyBindings;

export const desktopService: DesktopService = {
  GetSnapshot: bindings.GetSnapshot,
  GetPollingSnapshot: bindings.GetPollingSnapshot,
	GetLightingSnapshot: bindings.GetLightingSnapshot,
	GetRemapSnapshot: bindings.GetRemapSnapshot,
  RefreshStatus: bindings.RefreshStatus,
  RefreshInventory: bindings.RefreshInventory,
  SelectDevice: bindings.SelectDevice,
  StageDPI: bindings.StageDPI as DesktopService["StageDPI"],
  ApplyDPI: explicitApplyBindings.ApplyDPI,
  StagePollingRate: bindings.StagePollingRate as DesktopService["StagePollingRate"],
  ApplyPollingRate: explicitApplyBindings.ApplyPollingRate,
	StageLighting: bindings.StageLighting as DesktopService["StageLighting"],
	ApplyRemap: explicitApplyBindings.ApplyRemap,
	ApplyLighting: bindings.ApplyLighting,
	RetryRemapPersistence: bindings.RetryRemapPersistence,
  RetryPollingPersistence: bindings.RetryPollingPersistence,
  ResetToFactory: explicitApplyBindings.ResetToFactory,
  RetryPersistence: bindings.RetryPersistence,
  OnStatusEvent: (callback) => Events.On("mouse:status", (event) => callback(event.data)),
	OnConfiguration: (callback) => Events.On("mouse:configuration", (event) => callback(event.data)),
	OnPollingConfiguration: (callback) => Events.On("mouse:polling-configuration", (event) => callback(event.data)),
	OnRemapConfiguration: (callback) => Events.On("mouse:remap-configuration", (event) => callback(event.data)),
};
