import { Events } from "@wailsio/runtime";
import * as bindings from "../../cmd/x6configurator/frontend/bindings/github.com/blak0p/attack-shark-linux/internal/desktop/service";
import type { Action, EventType, Macro as GeneratedMacro } from "../../cmd/x6configurator/frontend/bindings/github.com/blak0p/attack-shark-linux/internal/macros/models";
import type { DesktopService, Macro, MacroEvent } from "./desktop-contract";

const bindingEvents = (events: MacroEvent[]) => events.map((event) => ({
  ...event, type: event.type as EventType, action: event.action as Action,
}));
const localMacro = (macro: GeneratedMacro): Macro => ({
  id: macro.id, name: macro.name,
  events: macro.events.map((event): MacroEvent => {
    if ((event.type !== "mouse_left" && event.type !== "mouse_right") ||
        (event.action !== "down" && event.action !== "up")) throw new Error("Unsupported local macro event");
    return { type: event.type, action: event.action, delay_ms: event.delay_ms };
  }),
});

type ExplicitApplyBindings = {
  ApplyDPI(): ReturnType<DesktopService["ApplyDPI"]>;
  ApplyPollingRate(): ReturnType<DesktopService["ApplyPollingRate"]>;
  ApplyRemap(config: Parameters<DesktopService["ApplyRemap"]>[0]): ReturnType<DesktopService["ApplyRemap"]>;
  ResetToFactory(): ReturnType<DesktopService["ResetToFactory"]>;
};
const explicitApplyBindings = bindings as typeof bindings & ExplicitApplyBindings;

export const desktopService: DesktopService = {
  ListMacros: async () => (await bindings.ListMacros()).map(localMacro),
  CreateMacro: async (name, events) => localMacro(await bindings.CreateMacro(name, bindingEvents(events))),
  ReadMacro: async (id) => localMacro(await bindings.ReadMacro(id)),
  UpdateMacro: async (id, name, events) => localMacro(await bindings.UpdateMacro(id, name, bindingEvents(events))),
  DeleteMacro: bindings.DeleteMacro,
  GetApplicationVersion: bindings.GetApplicationVersion,
  CheckForUpdate: bindings.CheckForUpdate as DesktopService["CheckForUpdate"],
  ApplyVerifiedUpdate: bindings.ApplyVerifiedUpdate as DesktopService["ApplyVerifiedUpdate"],
  GetSnapshot: bindings.GetSnapshot,
  GetPollingSnapshot: bindings.GetPollingSnapshot,
  GetDebounceSnapshot: bindings.GetDebounceSnapshot,
	GetLightingSnapshot: bindings.GetLightingSnapshot,
  GetNormalSleepSnapshot: bindings.GetNormalSleepSnapshot,
	GetRemapSnapshot: bindings.GetRemapSnapshot,
  RefreshStatus: bindings.RefreshStatus,
  RefreshInventory: bindings.RefreshInventory,
  SelectDevice: bindings.SelectDevice,
  StageDPI: bindings.StageDPI as DesktopService["StageDPI"],
  ApplyDPI: explicitApplyBindings.ApplyDPI,
  StagePollingRate: bindings.StagePollingRate as DesktopService["StagePollingRate"],
  ApplyPollingRate: explicitApplyBindings.ApplyPollingRate,
  StageDebounce: bindings.StageDebounce,
  ApplyDebounce: bindings.ApplyDebounce,
  RetryDebouncePersistence: bindings.RetryDebouncePersistence,
	StageLighting: bindings.StageLighting as DesktopService["StageLighting"],
  StageNormalSleep: bindings.StageNormalSleep,
  ApplyNormalSleep: bindings.ApplyNormalSleep,
  RetryNormalSleepPersistence: bindings.RetryNormalSleepPersistence,
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
