import type { CSSProperties } from "react";
import { useDesktopWorkspace } from "./hooks/useDesktopWorkspace";
import type { DesktopService, Device, LightingEffect } from "./desktop-contract";
import { WorkspaceShell } from "./components/workspace/WorkspaceShell";
import { WorkspaceView } from "./components/workspace/WorkspaceView";
import { TopBar } from "./components/workspace/TopBar";
import { DpiPanel } from "./components/panels/DpiPanel";
import { PollingPanel } from "./components/panels/PollingPanel";
import { PowerSettingsPanel } from "./components/panels/PowerSettingsPanel";
import { SleepPanel } from "./components/panels/SleepPanel";
import { MouseFeaturesPanel } from "./components/panels/MouseFeaturesPanel";
import { LightingPanel } from "./components/panels/LightingPanel";
import { LightingEffectSelect } from "./components/panels/LightingEffectSelect";
import { ButtonRemapPanel } from "./components/panels/ButtonRemapPanel";
import { DeviceStatusPanel } from "./components/panels/DeviceStatusPanel";
import { ResetPanel } from "./components/panels/ResetPanel";
export type {
  ConfigurationEvent,
  DesktopService,
  LightingSnapshot,
  PollingConfigurationEvent,
  PollingSnapshot,
  RemapSnapshot,
  Snapshot,
} from "./desktop-contract";

const identityLabel = (device: Device) => `Serial ${device.ID.Serial || "unavailable"}`;
const feedbackFor = (code: string) =>
  code === "stale_binding"
    ? "Device connection changed. Refresh the device list and select the mouse again before saving."
    : code.replaceAll("_", " ");
const SWATCH_COLORS = ["#00FF00", "#FE5EF9", "#FF7F00", "#FFFF00"];

type LightingSlot = { label: string; color?: string; templateID?: string; disabled: boolean };

const lightingSlots = (
  effect: LightingEffect | undefined,
  appliedColors?: number[][]
): LightingSlot[] => {
  if (!effect)
    return Array.from({ length: 5 }, (_, index) => ({
      label: `Unavailable color ${index + 1}`,
      disabled: true,
    }));
  const interactive = effect.ColorTemplates.slice(0, 4).map((template) => ({
    label: template.CSSColor,
    color: template.CSSColor,
    templateID: template.TemplateID,
    disabled: false,
  }));
  const fallback = (index: number): LightingSlot => ({
    label: `Unavailable color ${index + 1}`,
    color: SWATCH_COLORS[index],
    disabled: true,
  });
  return [
    ...interactive,
    ...Array.from({ length: 4 - interactive.length }, (_, index) =>
      fallback(interactive.length + index)
    ),
    {
      label: "Effect controlled",
      color: appliedColors?.length ? `rgb(${appliedColors[0].join(", ")})` : undefined,
      disabled: true,
    },
  ];
};

const speedFill = (index: number, count: number) =>
  count > 1 ? `${Math.round((index / (count - 1)) * 100)}%` : "0%";

export function App({ service }: { service: DesktopService }) {
  const { model, actions } = useDesktopWorkspace(service);
  const {
    snapshot,
    polling,
    debounce,
    lighting,
    normalSleep,
    remap,
    inventory,
    ready,
    notice,
    resetConfirmation,
    automaticApplyBusy,
  } = model;

  if (!snapshot) return <main className="app-shell" aria-busy="true">Loading configuration…</main>;

  const connected = ready;
  const errorCode = inventory?.Error.Code || snapshot.Error.Code;
  const pending = snapshot.Pending;
  const stages = pending.DPI.map((dpi, index) => ({ index, dpi })).filter(
    ({ index }) => ((pending.StageMask ?? 0) >> index) & 1
  );
  const activeIndex =
    snapshot.ObservedStage != null ? snapshot.ObservedStage - 1 : pending.ActiveStage - 1;
  const activeDPI =
    snapshot.ObservedStage != null
      ? snapshot.ObservedDPI ?? null
      : pending.DPI[activeIndex] ?? null;
  const activeColor =
    activeIndex >= 0 && snapshot.Applied.Colors ? snapshot.Applied.Colors[activeIndex] : null;
  const colorFor = (index: number) =>
    snapshot.Applied.Colors && snapshot.Applied.Colors[index]
      ? `rgb(${snapshot.Applied.Colors[index].join(", ")})`
      : null;
  const lightingEffect = lighting?.Effects.find(
    (effect) => effect.Mode === lighting.Pending.Mode
  );
  const speedIndex = Math.max(
    0,
    lightingEffect?.SpeedVariants.findIndex(
      (variant) => variant.TemplateID === lighting.Pending.TemplateID
    ) ?? 0
  );
  const slots = lightingSlots(lightingEffect, snapshot.Applied.Colors);
  const eligibleDevices = inventory?.Devices.filter((device) => device.Eligible) ?? [];

  const titlebarElement = (
    <TopBar title="Mouse configuration" subtitle="Attack Shark X6">
      {eligibleDevices.length > 1 && (
        <label style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 12 }}>
          <span>Mouse device</span>
          <select
            className="select"
            aria-label="Mouse device"
            value={inventory?.Selected?.ID.Serial ?? ""}
            onChange={(event) => actions.selectDevice(event.target.value)}
          >
            <option value="" disabled>
              Select a mouse
            </option>
            {eligibleDevices.map((device) => (
              <option key={device.ID.Serial} value={device.ID.Serial}>
                {device.ID.Serial}
              </option>
            ))}
          </select>
        </label>
      )}

      <div style={{ display: "flex", alignItems: "center", gap: 10, fontSize: 12 }}>
        {snapshot.Firmware && (
          <span
            role="status"
            aria-label={
              snapshot.Firmware === "success"
                ? "Firmware applied"
                : snapshot.Firmware === "pending"
                ? "Firmware synchronization queued"
                : "Firmware synchronization failed"
            }
          >
            {snapshot.Firmware === "success"
              ? "Firmware applied"
              : snapshot.Firmware === "pending"
              ? "Firmware synchronization queued"
              : "Firmware synchronization failed"}
          </span>
        )}
        {snapshot.Persistence && (
          <span role="status">
            {snapshot.Persistence === "success" ? "Persistence saved" : "Persistence failed"}
          </span>
        )}
        {snapshot.RetryAvailable && (
          <button type="button" className="button" onClick={actions.retryPersistence}>
            Retry local persistence
          </button>
        )}
      </div>
    </TopBar>
  );

  const connectionStatusElement = (
    <div
      id="device"
      aria-live="polite"
      className={`connection${connected ? "" : " offline"}`}
    >
      <b>
        <span className={`dot${connected ? "" : " offline"}`} aria-hidden="true" />
        {connected ? "Device available" : "Device unavailable"}
      </b>
      <span>{snapshot.Battery == null ? "Battery unavailable" : `Battery ${snapshot.Battery}%`}</span>
      {errorCode && (
        <span role="alert" style={{ display: "block", color: "var(--danger)", marginTop: 4 }}>
          {feedbackFor(errorCode)}
        </span>
      )}
    </div>
  );

  return (
    <WorkspaceShell
      busy={automaticApplyBusy}
      deviceName="Attack Shark X6"
      deviceSubtitle="Wireless Gaming Mouse"
      titlebar={titlebarElement}
      connectionStatus={connectionStatusElement}
    >
      {/* 1. Performance View */}
      <WorkspaceView
        id="performance"
        title="Performance"
        subtitle="DPI, polling rate, and normal sleep."
        placeholder="Device values"
      >
        <div className="stack">
          <DpiPanel
            stages={stages}
            activeIndex={activeIndex}
            activeDPI={activeDPI}
            activeColor={activeColor}
            colorFor={colorFor}
            ready={ready}
            onSelectStage={actions.selectStage}
            onStageDPI={actions.stageDPI}
          />

          {polling && (
            <PollingPanel
              snapshot={polling}
              ready={ready}
              onStagePollingRate={actions.stagePollingRate}
              onRetry={actions.retryPollingPersistence}
            />
          )}

          {normalSleep && (
            <SleepPanel
              snapshot={normalSleep}
              ready={ready}
              onStage={actions.stageNormalSleep}
              onRetry={actions.retryNormalSleepPersistence}
            />
          )}
        </div>
      </WorkspaceView>

      {/* 2. Lighting View */}
      <WorkspaceView
        id="lighting"
        title="Lighting"
        subtitle="Select an effect, its available speed, and lighting color."
        placeholder="Available effects"
      >
        {lighting && (
          <LightingPanel>
            <article className="card">
              <h2 id="lighting-title">Effect</h2>
              <p className="hint">Effect labels are supplied by the device.</p>
              <label className="lighting-effect-select">
                <span className="sr-only">Effect</span>
                <LightingEffectSelect
                  effects={lighting.Effects}
                  value={lighting.Pending.Mode}
                  disabled={!ready}
                  onChange={(effect) =>
                    actions.stageLighting({
                      Mode: effect.Mode,
                      TemplateID: effect.DefaultTemplateID,
                    })
                  }
                />
              </label>
            </article>

            <article className="card">
              <h2>Lighting color</h2>
              <p className="hint">Colors are supplied by the selected effect.</p>

              {lightingEffect && lightingEffect.SpeedVariants.length > 1 && (
                <div className="range-row" style={{ marginBottom: 14 }}>
                  <span>{lightingEffect.Label} speed</span>
                  <input
                    type="range"
                    aria-label={`${lightingEffect.Label} speed`}
                    min={0}
                    max={lightingEffect.SpeedVariants.length - 1}
                    step={1}
                    value={speedIndex}
                    disabled={!ready}
                    style={
                      {
                        "--fill": speedFill(speedIndex, lightingEffect.SpeedVariants.length),
                      } as CSSProperties
                    }
                    onChange={(event) => {
                      const variant =
                        lightingEffect.SpeedVariants[Number(event.target.value)];
                      if (variant)
                        actions.stageLighting({
                          Mode: lightingEffect.Mode,
                          TemplateID: variant.TemplateID,
                        });
                    }}
                  />
                  <span>
                    {speedIndex + 1} / {lightingEffect.SpeedVariants.length}
                  </span>
                </div>
              )}

              <fieldset className="lighting-colors" disabled={!ready}>
                <legend style={{ fontWeight: 600, marginBottom: 8 }}>Lighting color</legend>
                <div
                  className="lighting-options"
                  role="radiogroup"
                  aria-label="Lighting color"
                >
                  {slots.map((slot, index) => (
                    <label
                      className="lighting-option"
                      key={`${slot.label}-${index}`}
                    >
                      <input
                        type="radio"
                        name="lighting-color"
                        aria-label={slot.label}
                        checked={
                          slot.templateID != null &&
                          lighting?.Pending.TemplateID === slot.templateID
                        }
                        disabled={slot.disabled}
                        onChange={() =>
                          slot.templateID != null &&
                          lightingEffect &&
                          actions.stageLighting({
                            Mode: lightingEffect.Mode,
                            TemplateID: slot.templateID,
                          })
                        }
                      />
                      <span
                        className={`lighting-color-swatch${
                          slot.disabled ? " is-disabled" : ""
                        }`}
                        aria-hidden="true"
                        style={slot.color ? { backgroundColor: slot.color } : undefined}
                      />
                    </label>
                  ))}
                </div>
              </fieldset>
            </article>

            <div
              className="lighting-state status"
              role="status"
              aria-label="Lighting status"
            >
              {lighting.Firmware === "success" ? (
                "Lighting applied"
              ) : lighting.Firmware === "failed" ? (
                <span role="alert">
                  Lighting application failed: {feedbackFor(lighting.Error.Code)}
                </span>
              ) : (
                "Lighting selection pending"
              )}
            </div>
          </LightingPanel>
        )}
      </WorkspaceView>

      {/* 3. Controls View */}
      <WorkspaceView
        id="controls"
        title="Controls"
        subtitle="Mouse features and key response time."
        placeholder="Pending settings"
      >
        <article className="card">
          <MouseFeaturesPanel
            angleSnap={pending.AngleControl === true}
            rippleControl={pending.RippleControl === true}
            liftDistance={pending.LiftDistance ?? 1}
            disabled={!ready}
            onAngleSnapChange={(checked) => actions.stageFeature({ AngleControl: checked })}
            onRippleControlChange={(checked) => actions.stageFeature({ RippleControl: checked })}
            onLiftDistanceChange={(dist) => actions.stageFeature({ LiftDistance: dist })}
          />

          {debounce && (
            <PowerSettingsPanel
              value={debounce.Desired}
              appliedValue={debounce.Applied}
              firmwareStatus={debounce.Firmware}
              persistenceStatus={debounce.Persistence}
              retryAvailable={debounce.RetryAvailable}
              disabled={!ready}
              onChange={actions.stageDebounce}
              onRetry={actions.retryDebouncePersistence}
            />
          )}
        </article>
      </WorkspaceView>

      {/* 4. Button Remapping View */}
      <WorkspaceView
        id="remapping"
        title="Button remapping"
        subtitle="Review the complete assignment, then apply or discard it."
        placeholder="Pending assignment"
      >
        {remap && (
          <ButtonRemapPanel
            remap={remap}
            ready={ready}
            onStage={actions.stageRemap}
            onApply={actions.applyRemap}
            onDiscard={actions.discardRemap}
          />
        )}
      </WorkspaceView>

      {/* 5. Device View */}
      <WorkspaceView
        id="device"
        title="Device"
        subtitle="Connection, battery, inventory, and factory reset."
        placeholder="Device status"
      >
        <div className="stack">
          <DeviceStatusPanel
            connectionType={snapshot.Connection}
            battery={snapshot.Battery}
            serial={inventory?.Selected?.ID.Serial || undefined}
            inventory={inventory}
            errorCode={errorCode}
            feedbackFor={feedbackFor}
            onSelectDevice={actions.selectDevice}
          />

          {inventory && (
            <article className="card">
              <h2>Device inventory</h2>
              <div aria-label="Device inventory" className="rows">
                {inventory.Devices.map((device) => (
                  <div
                    className="inventory-device row"
                    key={`${device.Path}:${device.ID.Serial}`}
                  >
                    <span>{identityLabel(device)}</span>
                    {!device.Eligible && (
                      <div>
                        <span style={{ marginRight: 8 }}>{`Connection: ${device.Connection || "unavailable"}`}</span>
                        <span>{device.Warning || "not eligible"}</span>
                      </div>
                    )}
                  </div>
                ))}
              </div>
            </article>
          )}

          {inventory && !ready && (
            <p className="configuration-state status" role="status">
              Receiver detected, configuration unavailable.
            </p>
          )}

          <ResetPanel
            confirmation={resetConfirmation}
            ready={ready}
            onRequestReset={actions.requestReset}
            onConfirm={actions.confirmReset}
            onCancel={actions.cancelReset}
          />
        </div>
      </WorkspaceView>

      <p aria-live="polite" className="notice">
        {notice}
      </p>
    </WorkspaceShell>
  );
}

