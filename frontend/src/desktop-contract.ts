import type { Binding as GeneratedBinding } from "../bindings/github.com/blak0p/attack-shark-linux/internal/desktop/models";

export type Binding = GeneratedBinding;
export type DPIConfig = { DPI: number[]; ActiveStage: number; StageMask: number; LiftDistance: number; Colors?: number[][]; AngleControl?: boolean; RippleControl?: boolean };
export type Snapshot = { Connection: string; Battery?: number | null; Applied: DPIConfig; Pending: DPIConfig; Factory: DPIConfig; Revision: number; Error: { Code: string }; Firmware?: string; Persistence?: string; RetryAvailable?: boolean; ObservedStage?: number | null; ObservedDPI?: number | null };
export type PollingSnapshot = { Desired: number; Applied: number; Persisted?: number | null; Factory: number; Revision: number; Error: { Code: string }; Firmware?: string; Persistence?: string; RetryAvailable?: boolean };
export type DebounceSnapshot = { Desired: number; Applied: number; Persisted?: number | null; Factory: number; Revision: number; Error: { Code: string }; Firmware: string; Persistence: string; RetryAvailable: boolean };
export type NormalSleepSnapshot = { Pending: number; Applied: number; Persisted?: number | null; Revision: number; Firmware: string; Persistence: string; RetryAvailable: boolean; Error: { Code: string } };
export type LightingMode = 0x00 | 0x10 | 0x20 | 0x30 | 0x40 | 0x50 | 0x60;
export type LightingSelection = { Mode: LightingMode; TemplateID: string };
export type LightingSpeedVariant = { TemplateID: string };
export type LightingColorTemplate = { TemplateID: string; CSSColor: string };
export type LightingEffect = { Mode: LightingMode; Label: string; DefaultTemplateID: string; SpeedVariants: LightingSpeedVariant[]; ColorTemplates: LightingColorTemplate[] };
export type LightingSnapshot = { Pending: LightingSelection; Applied: LightingSelection | null; Effects: LightingEffect[]; Revision: number; Firmware: string; Error: { Code: string } };
export type RemapAction = "off" | "left" | "right" | "middle" | "forward" | "backward" | "double_click" | "fire" | "media_player" | "play_pause" | "stop" | "previous_track" | "next_track" | "volume_up" | "volume_down" | "mute" | "scroll_up" | "scroll_down" | "dpi_cycle" | "dpi_plus" | "dpi_minus" | "browser_calculator" | "browser_email" | "browser_forward" | "browser_backward" | "browser_stop" | "browser_my_computer" | "browser_refresh" | "browser_home" | "browser_search";
export type RemapButton = { Button: number; Action: RemapAction | null; PreservedDefault: "" | "DPI+" | "DPI-" };
export type RemapConfig = { Buttons: RemapButton[] };
// Transport confirmation does not establish playback or device persistence.
export type MacroProgress = { Assignment: 0 | 1 | 2; Upload: 0 | 1 | 2 };
export type MacroDraft = { ID: string; Name: string; Button: number; Repeat: number; Events: MacroEvent[] };
export type RemapSnapshot = { MacroPending?: MacroDraft | null; MacroApplied?: MacroDraft | null; MacroProgress?: MacroProgress; Pending: RemapConfig; Applied: RemapConfig; Factory: RemapConfig; Actions: RemapAction[]; Revision: number; Firmware: string; Persistence: string; RetryAvailable: boolean; Error: { Code: string } };
export type UpdateInfo = { Version: string };
export type ResetLaneResult = { Lane: string; State: string; Code: string };
export type ResetResult = { Lanes: ResetLaneResult[]; Cleanup: ResetLaneResult; Error: { Code: string }; RetryAvailable: boolean };
export type DeviceID = { VendorID: number; ProductID: number; Serial: string };
export type Device = { ID: DeviceID; Profile?: string; Path: string; Eligible: boolean; Warning?: string; Connection?: string };
export type Inventory = { Devices: Device[]; Selected: Binding | null; Error: { Code: string } };
export type StatusEvent = Partial<Binding> & { Connection?: string; Battery?: number | null; ActiveStage?: number | null };
export type ConfigurationEvent = { Binding: Binding; Snapshot: Snapshot };
export type PollingConfigurationEvent = { Binding: Binding; Snapshot: PollingSnapshot };
export type RemapConfigurationEvent = { Binding: Binding; Snapshot: RemapSnapshot };
export type MacroEvent = { type: "mouse_left" | "mouse_right"; action: "down" | "up"; delay_ms: number };
export type Macro = { id: string; name: string; events: MacroEvent[] };
export type MacroLibraryService = {
  ListMacros(): Promise<Macro[]>;
  CreateMacro(name: string, events: MacroEvent[]): Promise<Macro>;
  ReadMacro(id: string): Promise<Macro>;
  UpdateMacro(id: string, name: string, events: MacroEvent[]): Promise<Macro>;
  DeleteMacro(id: string): Promise<void>;
};
export type DesktopService = MacroLibraryService & { GetApplicationVersion?(): Promise<string>; CheckForUpdate?(): Promise<UpdateInfo | null>; ApplyVerifiedUpdate?(): Promise<void>; GetSnapshot(): Promise<Snapshot>; GetPollingSnapshot(): Promise<PollingSnapshot>; GetDebounceSnapshot(): Promise<DebounceSnapshot>; GetLightingSnapshot(): Promise<LightingSnapshot>; GetNormalSleepSnapshot(): Promise<NormalSleepSnapshot>; GetRemapSnapshot(): Promise<RemapSnapshot>; RefreshStatus(): Promise<Snapshot>; RefreshInventory(): Promise<Inventory>; SelectDevice(id: DeviceID): Promise<Inventory>; StageDPI(config: DPIConfig): Promise<Snapshot>; ApplyDPI(): Promise<Snapshot>; StagePollingRate(rate: number): Promise<PollingSnapshot>; ApplyPollingRate(): Promise<PollingSnapshot>; StageDebounce(responseTimeMs: number): Promise<DebounceSnapshot>; ApplyDebounce(): Promise<DebounceSnapshot>; RetryDebouncePersistence(): Promise<DebounceSnapshot>; StageLighting(selection: LightingSelection): Promise<LightingSnapshot>; StageNormalSleep(minutes: number): Promise<NormalSleepSnapshot>; ApplyNormalSleep(): Promise<NormalSleepSnapshot>; RetryNormalSleepPersistence(): Promise<NormalSleepSnapshot>; StageMacroAssignment(id: string, button: number, repeat: number): Promise<RemapSnapshot>; StageRemap(config: RemapConfig): Promise<RemapSnapshot>; ClearMacroAssignment(): Promise<RemapSnapshot>; DiscardRemap(): Promise<RemapSnapshot>; GetMacroAssignmentSnapshot(): Promise<RemapSnapshot>; ApplyRemap(config: RemapConfig): Promise<RemapSnapshot>; ApplyLighting(): Promise<LightingSnapshot>; RetryRemapPersistence(): Promise<RemapSnapshot>; ResetToFactory(): Promise<ResetResult>; RetryPersistence(): Promise<Snapshot>; RetryPollingPersistence(): Promise<PollingSnapshot>; OnStatusEvent(callback: (event: StatusEvent) => void): () => void; OnConfiguration(callback: (event: ConfigurationEvent) => void): () => void; OnPollingConfiguration(callback: (event: PollingConfigurationEvent) => void): () => void; OnRemapConfiguration(callback: (event: RemapConfigurationEvent) => void): () => void };

export type WorkspaceModel = { snapshot?: Snapshot; polling?: PollingSnapshot; debounce?: DebounceSnapshot; lighting?: LightingSnapshot; normalSleep?: NormalSleepSnapshot; remap?: RemapSnapshot; inventory?: Inventory; applicationVersion?: string; update?: UpdateInfo; updateApplying: boolean; updateError: string; ready: boolean; notice: string; resetConfirmation: boolean; automaticApplyBusy: boolean; remapBusy?: boolean; remapError?: string };

export type WorkspaceActions = { applyUpdate(): void; selectDevice(serial: string): void; stageDPI(index: number, value: number): void; selectStage(index: number): void; stageFeature(patch: Partial<DPIConfig>): void; stagePollingRate(rate: number): void; stageDebounce(responseTimeMs: number): void; retryDebouncePersistence(): void; stageLighting(value: LightingSelection): void; stageNormalSleep(minutes: number): void; retryNormalSleepPersistence(): void; stageMacroAssignment(id: string, button: number, repeat: number): void; stageRemap(button: number, action: RemapAction): void; applyRemap(): void; discardRemap(): void; requestReset(): void; cancelReset(): void; confirmReset(): void; retryPersistence(): void; retryPollingPersistence(): void };
