import { Call, Events } from "@wailsio/runtime";
import { copyMacroEvent } from "./macros/macro-file";
import * as bindings from "../../cmd/x6configurator/frontend/bindings/github.com/blak0p/attack-shark-linux/internal/desktop/service";
import type { Action, EventType, Macro as GeneratedMacro } from "../../cmd/x6configurator/frontend/bindings/github.com/blak0p/attack-shark-linux/internal/macros/models";
import type { RemapSnapshot as GeneratedRemapSnapshot } from "../../cmd/x6configurator/frontend/bindings/github.com/blak0p/attack-shark-linux/internal/desktop/models";
import { RemapAction as GeneratedRemapAction, type RemapConfig as GeneratedRemapConfig } from "../../cmd/x6configurator/frontend/bindings/github.com/blak0p/attack-shark-linux/internal/protocol/x6/models";
import type { DesktopService, Macro, MacroEvent, MacroDraft, RemapSnapshot, RemapConfig, ProfilesService, ProfileSnapshot, WatcherService, WatcherSettingsSnapshot, WatcherRule, GamesService } from "./desktop-contract";

const bindingRemap = (config: RemapConfig): GeneratedRemapConfig => ({
  Buttons: config.Buttons.map((button) => {
    // Only existing preserved DPI defaults may use Go's zero action.
    if (button.Action === null || String(button.Action) === "") {
      if (!((button.Button === 6 && button.PreservedDefault === "DPI+") ||
            (button.Button === 7 && button.PreservedDefault === "DPI-"))) {
        throw new Error("Unsupported preserved remap default");
      }
      return { ...button, Action: GeneratedRemapAction.$zero };
    }
    const action = Object.values(GeneratedRemapAction).find((value) =>
      value !== GeneratedRemapAction.$zero && value === button.Action);
    if (action === undefined) throw new Error("Unsupported remap action");
    return { ...button, Action: action };
  }),
});

const bindingEvents = (events: MacroEvent[]) => events.map((event) => ({
  ...copyMacroEvent(event), type: event.type as EventType, action: event.action as Action,
}));
const localMacro = (macro: GeneratedMacro): Macro => ({
  id: macro.id, name: macro.name,
  events: macro.events.map((event): MacroEvent => {
    if ((event.type !== "mouse_left" && event.type !== "mouse_right" &&
         event.type !== "mouse_middle" && event.type !== "mouse_back" && event.type !== "mouse_forward" && event.type !== "keyboard") ||
        (event.action !== "down" && event.action !== "up")) throw new Error("Unsupported local macro event");
    return copyMacroEvent(event);
  }),
});

const localDraft = (draft: GeneratedRemapSnapshot["MacroPending"] | undefined): MacroDraft | null | undefined => draft == null ? draft : ({
  ID: draft.ID, Name: draft.Name, Button: draft.Button, Repeat: draft.Repeat, Mode: draft.Mode ?? 0,
  Events: localMacro({ id: draft.ID, name: draft.Name, events: draft.Events }).events,
});
const localDraftMap = (drafts: GeneratedRemapSnapshot["MacroDrafts"] | undefined): Record<number, MacroDraft> | undefined => {
  if (!drafts) return undefined;
  const res: Record<number, MacroDraft> = {};
  for (const [k, v] of Object.entries(drafts)) {
    const d = localDraft(v);
    if (d) res[Number(k)] = d;
  }
  return res;
};
const localRemap = (snapshot: GeneratedRemapSnapshot): RemapSnapshot => ({
  ...snapshot,
  MacroPending: localDraft(snapshot.MacroPending),
  MacroApplied: localDraft(snapshot.MacroApplied),
  MacroDrafts: localDraftMap(snapshot.MacroDrafts),
  MacroAppliedDrafts: localDraftMap(snapshot.MacroAppliedDrafts),
});

type ExplicitApplyBindings = {
  ApplyDPI(): ReturnType<DesktopService["ApplyDPI"]>;
  ApplyPollingRate(): ReturnType<DesktopService["ApplyPollingRate"]>;
  ResetToFactory(): ReturnType<DesktopService["ResetToFactory"]>;
};
const explicitApplyBindings = bindings as typeof bindings & ExplicitApplyBindings;

const profileCall = (method: keyof ProfilesService, ...args: string[]): Promise<ProfileSnapshot> =>
  Call.ByName(`github.com/blak0p/attack-shark-linux/internal/desktop.Service.${method}`, ...args);

const watcherCall = (method: keyof WatcherService, ...args: unknown[]): Promise<WatcherSettingsSnapshot> =>
  Call.ByName(`github.com/blak0p/attack-shark-linux/internal/desktop.Service.${method}`, ...args);

const gamesCall = <T,>(method: keyof GamesService): Promise<T> =>
  Call.ByName(`github.com/blak0p/attack-shark-linux/internal/desktop.Service.${method}`);

export const desktopService: DesktopService & ProfilesService & WatcherService & GamesService = {
  GetLauncherPath: () => gamesCall('GetLauncherPath'),
  ListInstalledGames: () => gamesCall('ListInstalledGames'),
  PickGameFolder: () => gamesCall('PickGameFolder'),
  GetWatcherSettings: () => watcherCall('GetWatcherSettings'),
  SetWatcherSettings: (enabled: boolean, rules: WatcherRule[]) => watcherCall('SetWatcherSettings', enabled, rules),
  SetTrayEnabled: (enabled: boolean) => watcherCall('SetTrayEnabled', enabled),
  GetProfiles: () => profileCall('GetProfiles'),
  CreateProfile: name => profileCall('CreateProfile', name),
  RenameProfile: (id, name) => profileCall('RenameProfile', id, name),
  DuplicateProfile: (id, name) => profileCall('DuplicateProfile', id, name),
  DeleteProfile: id => profileCall('DeleteProfile', id),
  SwitchProfile: id => profileCall('SwitchProfile', id),
  DismissProfileNotice: () => profileCall('DismissProfileNotice'),
  ListMacros: async () => (await bindings.ListMacros()).map(localMacro),
  CreateMacro: async (name, events) => localMacro(await bindings.CreateMacro(name, bindingEvents(events))),
  ReadMacro: async (id) => localMacro(await bindings.ReadMacro(id)),
  UpdateMacro: async (id, name, events) => localMacro(await bindings.UpdateMacro(id, name, bindingEvents(events))),
  DeleteMacro: bindings.DeleteMacro,
  GetApplicationVersion: bindings.GetApplicationVersion,
  CheckUnifiedUpdates: bindings.CheckUnifiedUpdates,
  ApplyUnifiedUpdate: bindings.ApplyUnifiedUpdate,
  CheckForUpdate: bindings.CheckForUpdate as DesktopService["CheckForUpdate"],
  ApplyVerifiedUpdate: bindings.ApplyVerifiedUpdate as DesktopService["ApplyVerifiedUpdate"],
  GetSnapshot: bindings.GetSnapshot,
  GetPollingSnapshot: bindings.GetPollingSnapshot,
  GetDebounceSnapshot: bindings.GetDebounceSnapshot,
	GetLightingSnapshot: bindings.GetLightingSnapshot,
  GetNormalSleepSnapshot: bindings.GetNormalSleepSnapshot,
	GetRemapSnapshot: async () => localRemap(await bindings.GetRemapSnapshot()),
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
  StageMacroAssignment: async (id, button, repeat) => localRemap(await bindings.StageMacroAssignment(id, button, repeat)),
  StageMacroAssignmentWithMode: async (id, button, repeat, mode) => localRemap(await bindings.StageMacroAssignmentWithMode(id, button, repeat, mode)),
  StageRemap: async (config) => localRemap(await bindings.StageRemap(bindingRemap(config))),
  ClearButtonMacroAssignment: async (button: number) =>
    localRemap(await bindings.ClearButtonMacroAssignment(button)),
  ClearMacroAssignment: async () => localRemap(await bindings.ClearMacroAssignment()),
  DiscardRemap: async () => localRemap(await bindings.DiscardRemap()),
  GetMacroAssignmentSnapshot: async () => localRemap(await bindings.GetMacroAssignmentSnapshot()),
  ApplyRemap: async (config) => localRemap(await bindings.ApplyRemap(bindingRemap(config))),
	ApplyLighting: bindings.ApplyLighting,
	RetryRemapPersistence: async () => localRemap(await bindings.RetryRemapPersistence()),
  RetryPollingPersistence: bindings.RetryPollingPersistence,
  ResetToFactory: explicitApplyBindings.ResetToFactory,
  RetryPersistence: bindings.RetryPersistence,
  OnStatusEvent: (callback) => Events.On("mouse:status", (event) => callback(event.data)),
	OnConfiguration: (callback) => Events.On("mouse:configuration", (event) => callback(event.data)),
	OnPollingConfiguration: (callback) => Events.On("mouse:polling-configuration", (event) => callback(event.data)),
	OnRemapConfiguration: (callback) => Events.On("mouse:remap-configuration", (event) => callback(event.data)),
};
