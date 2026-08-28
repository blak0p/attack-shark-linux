import type { Binding as GeneratedBinding } from "../bindings/github.com/blak0p/attack-shark-linux/internal/desktop/models";

export type Binding = GeneratedBinding;
export type DPIConfig = { DPI: number[]; ActiveStage: number; StageMask: number; LiftDistance: number; Colors?: number[][]; AngleControl?: boolean; RippleControl?: boolean };
export type Snapshot = { Connection: string; Battery?: number | null; Applied: DPIConfig; Pending: DPIConfig; Factory: DPIConfig; Revision: number; Error: { Code: string }; Firmware?: string; Persistence?: string; RetryAvailable?: boolean; ObservedStage?: number | null; ObservedDPI?: number | null };
export type PollingSnapshot = { Desired: number; Applied: number; Persisted?: number | null; Factory: number; Revision: number; Firmware?: string; Persistence?: string; RetryAvailable?: boolean };
export type LightingMode = 0x00 | 0x10 | 0x20 | 0x30 | 0x40 | 0x50 | 0x60;
export type LightingSelection = { Mode: LightingMode; TemplateID: string };
export type LightingSpeedVariant = { TemplateID: string };
export type LightingColorTemplate = { TemplateID: string; CSSColor: string };
export type LightingEffect = { Mode: LightingMode; Label: string; DefaultTemplateID: string; SpeedVariants: LightingSpeedVariant[]; ColorTemplates: LightingColorTemplate[] };
export type LightingSnapshot = { Pending: LightingSelection; Applied: LightingSelection | null; Effects: LightingEffect[]; Revision: number; Firmware: string; Error: { Code: string } };
export type DeviceID = { VendorID: number; ProductID: number; Serial: string };
export type Device = { ID: DeviceID; Profile?: string; Path: string; Eligible: boolean; Warning?: string; Connection?: string };
export type Inventory = { Devices: Device[]; Selected: Binding | null; Error: { Code: string } };
export type StatusEvent = Partial<Binding> & { Connection?: string; Battery?: number | null; ActiveStage?: number | null };
export type ConfigurationEvent = { Binding: Binding; Snapshot: Snapshot };
export type PollingConfigurationEvent = { Binding: Binding; Snapshot: PollingSnapshot };
export type DesktopService = { GetSnapshot(): Promise<Snapshot>; GetPollingSnapshot(): Promise<PollingSnapshot>; GetLightingSnapshot(): Promise<LightingSnapshot>; RefreshStatus(): Promise<Snapshot>; RefreshInventory(): Promise<Inventory>; SelectDevice(id: DeviceID): Promise<Inventory>; StageDPI(config: DPIConfig): Promise<Snapshot>; StagePollingRate(rate: number): Promise<PollingSnapshot>; StageLighting(selection: LightingSelection): Promise<LightingSnapshot>; ApplyLighting(): Promise<LightingSnapshot>; ResetToFactory(): Promise<Snapshot>; RetryPersistence(): Promise<Snapshot>; RetryPollingPersistence(): Promise<PollingSnapshot>; OnStatusEvent(callback: (event: StatusEvent) => void): () => void; OnConfiguration(callback: (event: ConfigurationEvent) => void): () => void; OnPollingConfiguration(callback: (event: PollingConfigurationEvent) => void): () => void };

export type WorkspaceModel = { snapshot?: Snapshot; polling?: PollingSnapshot; lighting?: LightingSnapshot; inventory?: Inventory; ready: boolean; notice: string };

export type WorkspaceActions = { selectDevice(serial: string): void; stageDPI(index: number, value: number): void; selectStage(index: number): void; stageFeature(patch: Partial<DPIConfig>): void; stagePollingRate(rate: number): void; stageLighting(value: LightingSelection): void; applyLighting(): void; reset(): void; retryPersistence(): void; retryPollingPersistence(): void };
