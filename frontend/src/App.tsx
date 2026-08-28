import type { CSSProperties } from "react";
import { useDesktopWorkspace } from "./hooks/useDesktopWorkspace";
import type { DesktopService, Device, LightingEffect } from "./desktop-contract";
import { WorkspaceShell } from "./components/workspace/WorkspaceShell";
import { WorkspaceView } from "./components/workspace/WorkspaceView";
import { TopBar } from "./components/workspace/TopBar";
import { DpiPanel } from "./components/panels/DpiPanel";
import { PollingPanel } from "./components/panels/PollingPanel";
import { MouseFeaturesPanel } from "./components/panels/MouseFeaturesPanel";
import { LightingPanel } from "./components/panels/LightingPanel";
import { ResetPanel } from "./components/panels/ResetPanel";
export type { ConfigurationEvent, DesktopService, LightingSnapshot, PollingConfigurationEvent, PollingSnapshot, Snapshot } from "./desktop-contract";

// Protocol-derived bounds: the official Windows app caps its DPI slider at
// 520 (= DPI/50), matching the PAW3395 sensor's 26000 max (docs/app-x6.md).
const DPI_MIN = 50;
const DPI_MAX = 26000;
const DPI_STEP = 50;
const position = (value: number) => Math.round(Math.max(0, Math.min(1, (value - DPI_MIN) / (DPI_MAX - DPI_MIN))) * 1000) / 10;
const identityLabel = (device: Device) => `Serial ${device.ID.Serial || "unavailable"}`;
const feedbackFor = (code: string) => code === "stale_binding"
  ? "Device connection changed. Refresh the device list and select the mouse again before saving."
  : code.replaceAll("_", " ");
const SWATCH_COLORS = ["#00FF00", "#FE5EF9", "#FF7F00", "#FFFF00"];
type LightingSlot = { label: string; color?: string; templateID?: string; disabled: boolean };
const lightingSlots = (effect: LightingEffect | undefined, appliedColors?: number[][]): LightingSlot[] => {
  if (!effect) return Array.from({ length: 5 }, (_, index) => ({ label: `Unavailable color ${index + 1}`, disabled: true }));
  const interactive = effect.ColorTemplates.slice(0, 4).map((template) => ({ label: template.CSSColor, color: template.CSSColor, templateID: template.TemplateID, disabled: false }));
  const fallback = (index: number): LightingSlot => ({ label: `Unavailable color ${index + 1}`, color: SWATCH_COLORS[index], disabled: true });
  return [...interactive, ...Array.from({ length: 4 - interactive.length }, (_, index) => fallback(interactive.length + index)), { label: "Effect controlled", color: appliedColors?.length ? `rgb(${appliedColors[0].join(", ")})` : undefined, disabled: true }];
};
const speedFill = (index: number, count: number) => count > 1 ? `${Math.round((index / (count - 1)) * 100)}%` : "0%";

export function App({ service }: { service: DesktopService }) {
  const { model, actions } = useDesktopWorkspace(service);
  const { snapshot, polling, lighting, inventory, ready, notice } = model;
  if (!snapshot) return <main className="app-shell" aria-busy="true">Loading configuration…</main>;

  const connected = ready;
  const errorCode = inventory?.Error.Code || snapshot.Error.Code;
  const pending = snapshot.Pending;
  const stages = pending.DPI.map((dpi, index) => ({ index, dpi })).filter(({ index }) => ((pending.StageMask ?? 0) >> index) & 1);
  const activeIndex = snapshot.ObservedStage != null ? snapshot.ObservedStage - 1 : pending.ActiveStage - 1;
  const activeDPI = snapshot.ObservedStage != null ? snapshot.ObservedDPI ?? null : pending.DPI[activeIndex] ?? null;
  const activeColor = activeIndex >= 0 && snapshot.Applied.Colors ? snapshot.Applied.Colors[activeIndex] : null;
  const colorFor = (index: number) => (snapshot.Applied.Colors && snapshot.Applied.Colors[index] ? `rgb(${snapshot.Applied.Colors[index].join(", ")})` : null);
  const lightingEffect = lighting?.Effects.find((effect) => effect.Mode === lighting.Pending.Mode);
  const speedIndex = Math.max(0, lightingEffect?.SpeedVariants.findIndex((variant) => variant.TemplateID === lighting.Pending.TemplateID) ?? 0);
  const slots = lightingSlots(lightingEffect, snapshot.Applied.Colors);

  return (
    <WorkspaceShell>
        <TopBar>
	          {inventory && inventory.Devices.filter((device) => device.Eligible).length > 1 && (
            <label>
              <span>Mouse device</span>
              <select aria-label="Mouse device" value={inventory.Selected?.ID.Serial ?? ""} onChange={(event) => actions.selectDevice(event.target.value)}>
                <option value="" disabled>Select a mouse</option>
                {inventory.Devices.filter((device) => device.Eligible).map((device) => (
                  <option key={device.ID.Serial} value={device.ID.Serial}>{device.ID.Serial}</option>
                ))}
              </select>
            </label>
	          )}
          {inventory && <div aria-label="Device inventory">
            {inventory.Devices.map((device) => <div className="inventory-device" key={`${device.Path}:${device.ID.Serial}`}>
              <span>{identityLabel(device)}</span>
              {!device.Eligible && <>
                <span>{`Connection: ${device.Connection || "unavailable"}`}</span>
                <span>{device.Warning || "not eligible"}</span>
              </>}
            </div>)}
          </div>}
          <div id="device" aria-live="polite" className={`status ${connected ? "online" : "offline"}`}>
            <strong>{connected ? "Device available" : "Device unavailable"}</strong>
            <span>{snapshot.Battery == null ? "Battery unavailable" : `Battery ${snapshot.Battery}%`}</span>
            {errorCode && <span role="alert">{feedbackFor(errorCode)}</span>}
          </div>
          {inventory && !ready && <p className="configuration-state" role="status">Receiver detected, configuration unavailable.</p>}
          {snapshot.Firmware && <span role="status" aria-label={snapshot.Firmware === "success" ? "Firmware applied" : snapshot.Firmware === "pending" ? "Firmware synchronization queued" : "Firmware synchronization failed"}>{snapshot.Firmware === "success" ? "Firmware applied" : snapshot.Firmware === "pending" ? "Firmware synchronization queued" : "Firmware synchronization failed"}</span>}
          {snapshot.Persistence && <span role="status">{snapshot.Persistence === "success" ? "Persistence saved" : "Persistence failed"}</span>}
          {snapshot.RetryAvailable && <button type="button" onClick={actions.retryPersistence}>Retry local persistence</button>}
        </TopBar>

        <WorkspaceView id="performance" title="Performance"><DpiPanel>
          <div className="panel-header">
            <div>
              <h2 className="panel-title" id="dpi-title">DPI & Performance</h2>
              <div className="panel-subtitle">
                {activeIndex >= 0 ? activeDPI != null ? <>Active DPI: {activeDPI}{activeColor && <span className="color-swatch" style={{ background: `rgb(${activeColor.join(", ")})` }} />}</> : "Active DPI: unknown" : "No active DPI stage"}
              </div>
            </div>
            <span className="dots">⋮</span>
          </div>

          <div className="panel-body">
            <div className="stage-indicator" role="group" aria-label="Active stage">
              {stages.map(({ index }) => (
                <button
                  key={index}
                  type="button"
                  className={`stage-dot${index === activeIndex ? " active" : ""}`}
                  aria-pressed={index === activeIndex}
                  aria-label={`Stage ${index + 1}${index === activeIndex ? ", active" : ""}`}
                  disabled={!ready}
                  onClick={() => actions.selectStage(index)}
                  style={{ "--dot-color": colorFor(index) ?? "#3b82f6" } as CSSProperties}
                >
                  <span>{index + 1}</span>
                </button>
              ))}
            </div>
            <div className="stage-list">
              {stages.length === 0
                ? <p className="stage-empty">No DPI stages enabled on this device.</p>
                : stages.map(({ index, dpi }) => (
                    <div className="stage-item" key={index}>
                    <div className="stage-label">
                      <span>Stage {index + 1}</span>
                      <span>{dpi}</span>
                    </div>
                      <div className="slider-container">
                        <input
                          className="stage-slider"
                          type="range"
                          aria-label={`Stage ${index + 1} DPI`}
                          min={DPI_MIN}
                          max={DPI_MAX}
                          step={DPI_STEP}
                          value={dpi}
                          disabled={!ready}
                          onChange={(event) => actions.stageDPI(index, Number(event.target.value))}
                          style={{ "--fill": `${position(dpi)}%`, "--stage-color": colorFor(index) ?? "#3b82f6" } as CSSProperties}
                        />
                      </div>
                    </div>
                  ))}
            </div>
          </div>

          {polling && <PollingPanel><fieldset className="polling-control" disabled={!ready}>
            <legend>Polling rate</legend>
            <p className="polling-guidance">Higher rates favor responsiveness; lower rates favor battery life.</p>
            <div className="polling-options" role="radiogroup" aria-label="Polling rate">
              {[125, 250, 500, 1000].map((rate) => (
                <label className="polling-option" key={rate}>
                  <input
                    type="radio"
                    name="polling-rate"
                    value={rate}
                    checked={polling.Desired === rate}
                    onChange={() => actions.stagePollingRate(rate)}
                  />
                  <span>{rate} Hz</span>
                </label>
              ))}
            </div>
            <div className="polling-state" role="status" aria-label="Polling status">
              {polling.Firmware === "pending" ? <><span className="pending-spinner" aria-hidden="true" />Applying…</> : polling.Firmware === "failed" ? <>Polling change failed</> : <>Applied {polling.Applied} Hz</>}
              {polling.Persistence === "failed" && <span>Polling preference was not saved.</span>}
              {polling.RetryAvailable && <button type="button" onClick={actions.retryPollingPersistence}>Retry polling persistence</button>}
            </div>
          </fieldset></PollingPanel>}
        </DpiPanel></WorkspaceView>

        <WorkspaceView id="controls" title="Controls"><MouseFeaturesPanel><fieldset className="polling-control" disabled={!ready}>
            <legend>Mouse features</legend>
            <div className="polling-options">
              <label className="polling-option">
                <input
                  type="checkbox"
                  name="angle-snap"
                  checked={pending.AngleControl === true}
                  onChange={(event) => actions.stageFeature({ AngleControl: event.target.checked })}
                />
                <span>Angle snap</span>
              </label>
              <label className="polling-option">
                <input
                  type="checkbox"
                  name="ripple-control"
                  checked={pending.RippleControl === true}
                  onChange={(event) => actions.stageFeature({ RippleControl: event.target.checked })}
                />
                <span>Ripple control</span>
              </label>
              <label className="lighting-effect-select">
                <span>Lift-off distance</span>
                <select
                  aria-label="Lift-off distance"
                  value={String(pending.LiftDistance ?? 1)}
                  onChange={(event) => actions.stageFeature({ LiftDistance: Number(event.target.value) })}
                >
                  <option value="1">1 mm</option>
                  <option value="0">2 mm</option>
                </select>
              </label>
            </div>
          </fieldset></MouseFeaturesPanel></WorkspaceView>

        <WorkspaceView id="lighting" title="Lighting">{lighting && <LightingPanel>
            <h3 id="lighting-title">Lighting</h3>
            <label className="lighting-effect-select">
              <span>Effect</span>
              <select
                aria-label="Lighting effect"
                value={String(lighting.Pending.Mode)}
                disabled={!ready}
                onChange={(event) => {
                  const effect = lighting.Effects.find((candidate) => String(candidate.Mode) === event.target.value);
                  if (effect) actions.stageLighting({ Mode: effect.Mode, TemplateID: effect.DefaultTemplateID });
                }}
              >
                {lighting.Effects.map((effect) => <option key={effect.Mode} value={String(effect.Mode)}>{effect.Label}</option>)}
              </select>
            </label>
            {lightingEffect && lightingEffect.SpeedVariants.length > 1 && <fieldset className="lighting-speed" disabled={!ready}>
              <legend>{lightingEffect.Label} speed</legend>
              <input
                type="range"
                aria-label={`${lightingEffect.Label} speed`}
                min={0}
                max={lightingEffect.SpeedVariants.length - 1}
                step={1}
                value={speedIndex}
                style={{ "--fill": speedFill(speedIndex, lightingEffect.SpeedVariants.length) } as CSSProperties}
                onChange={(event) => {
                  const variant = lightingEffect.SpeedVariants[Number(event.target.value)];
                  if (variant) actions.stageLighting({ Mode: lightingEffect.Mode, TemplateID: variant.TemplateID });
                }}
              />
            </fieldset>}
            <fieldset className="lighting-colors" disabled={!ready}>
              <legend>Lighting color</legend>
              <div className="lighting-options" role="radiogroup" aria-label="Lighting color">
                {slots.map((slot, index) => (
                  <label className="lighting-option lighting-color-option" key={`${slot.label}-${index}`}>
                    <input
                      type="radio"
                      name="lighting-color"
                      aria-label={slot.label}
                      checked={slot.templateID != null && lighting?.Pending.TemplateID === slot.templateID}
                      disabled={slot.disabled}
                      onChange={() => slot.templateID != null && lightingEffect && actions.stageLighting({ Mode: lightingEffect.Mode, TemplateID: slot.templateID })}
                    />
                    <span className={`lighting-color-swatch${slot.disabled ? " is-disabled" : ""}`} aria-hidden="true" style={slot.color ? { backgroundColor: slot.color } : undefined} />
                  </label>
                ))}
              </div>
            </fieldset>
            <button className="apply-lighting-btn" type="button" disabled={!ready} onClick={actions.applyLighting}>Apply lighting</button>
            <div className="lighting-state" role="status" aria-label="Lighting status">
              {lighting.Firmware === "success" ? "Lighting applied" : lighting.Firmware === "failed" ? <span role="alert">Lighting application failed: {feedbackFor(lighting.Error.Code)}</span> : "Lighting selection pending"}
            </div>
          </LightingPanel>}</WorkspaceView>

        <WorkspaceView id="device" title="Device"><ResetPanel><button className="reset-btn" disabled={!ready} onClick={actions.reset}>Reset to factory</button></ResetPanel></WorkspaceView>

        <p aria-live="polite" className="notice">{notice}</p>
    </WorkspaceShell>
  );
}
