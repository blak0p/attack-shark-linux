import { useEffect, useRef, useState, type CSSProperties } from "react";
import type { Binding as GeneratedBinding } from "../bindings/github.com/blak0p/attack-shark-linux/internal/desktop/models";

export type Binding = GeneratedBinding;
export type DPIConfig = { DPI: number[]; ActiveStage: number; StageMask: number; LiftDistance: number; Colors?: number[][]; AngleControl?: boolean; RippleControl?: boolean };
export type Snapshot = { Connection: string; Battery?: number | null; Applied: DPIConfig; Pending: DPIConfig; Factory: DPIConfig; Revision: number; Error: { Code: string }; Firmware?: string; Persistence?: string; RetryAvailable?: boolean; ObservedStage?: number | null; ObservedDPI?: number | null };
export type DeviceID = { VendorID: number; ProductID: number; Serial: string };
export type Device = { ID: DeviceID; Profile?: string; Path: string; Eligible: boolean; Warning?: string; Connection?: string };
export type Inventory = { Devices: Device[]; Selected: Binding | null; Error: { Code: string } };
export type StatusEvent = Partial<Binding> & { Connection?: string; Battery?: number | null; ActiveStage?: number | null };
export type ConfigurationEvent = { Binding: Binding; Snapshot: Snapshot };
export type DesktopService = { GetSnapshot(): Promise<Snapshot>; RefreshStatus(): Promise<Snapshot>; RefreshInventory(): Promise<Inventory>; SelectDevice(id: DeviceID): Promise<Inventory>; StageDPI(config: DPIConfig): Promise<Snapshot>; RetryPersistence(): Promise<Snapshot>; OnStatusEvent(callback: (event: StatusEvent) => void): () => void; OnConfiguration(callback: (event: ConfigurationEvent) => void): () => void };

// Protocol-derived bounds: the official Windows app caps its DPI slider at
// 520 (= DPI/50), matching the PAW3395 sensor's 26000 max (docs/app-x6.md).
const DPI_MIN = 50;
const DPI_MAX = 26000;
const DPI_STEP = 50;

const position = (value: number) => Math.round(Math.max(0, Math.min(1, (value - DPI_MIN) / (DPI_MAX - DPI_MIN))) * 1000) / 10;
const identityLabel = (device: Device) => `VID ${device.ID.VendorID.toString(16).padStart(4, "0").toUpperCase()} · PID ${device.ID.ProductID.toString(16).padStart(4, "0").toUpperCase()} · Serial ${device.ID.Serial || "unavailable"}`;
const feedbackFor = (code: string) => code === "stale_binding"
  ? "Device connection changed. Refresh the device list and select the mouse again before saving."
  : code.replaceAll("_", " ");

export function App({ service }: { service: DesktopService }) {
  const [snapshot, setSnapshot] = useState<Snapshot>();
  const [inventory, setInventory] = useState<Inventory>();
  const selected = useRef<Binding | null>(null);
  const [notice, setNotice] = useState("");
  useEffect(() => { void service.RefreshStatus().then(setSnapshot); }, [service]);
  useEffect(() => { void service.RefreshInventory().then(setInventory); }, [service]);
  useEffect(() => { selected.current = inventory?.Selected; }, [inventory]);
  useEffect(() => {
    const unsubscribe = service.OnStatusEvent((event) => {
      setSnapshot((current) => (current && receivesEvent(selected.current, event) ? applyStatusEvent(current, event) : current));
    });
    return unsubscribe;
  }, [service]);
	  useEffect(() => service.OnConfiguration((event) => {
	  if (receivesEvent(selected.current, event.Binding)) setSnapshot(event.Snapshot);
	}), [service]);
  if (!snapshot) return <main className="app-shell" aria-busy="true">Loading configuration…</main>;

  const ready = inventory?.Selected != null;
  const connected = ready;
  const errorCode = inventory?.Error.Code || snapshot.Error.Code;
  const pending = snapshot.Pending;
  const stages = pending.DPI.map((dpi, index) => ({ index, dpi })).filter(({ index }) => ((pending.StageMask ?? 0) >> index) & 1);
  const activeIndex = snapshot.ObservedStage != null ? snapshot.ObservedStage - 1 : pending.ActiveStage - 1;
  const activeDPI = snapshot.ObservedStage != null ? snapshot.ObservedDPI ?? null : pending.DPI[activeIndex] ?? null;
  const activeColor = activeIndex >= 0 && snapshot.Applied.Colors ? snapshot.Applied.Colors[activeIndex] : null;
  const colorFor = (index: number) => (snapshot.Applied.Colors && snapshot.Applied.Colors[index] ? `rgb(${snapshot.Applied.Colors[index].join(", ")})` : null);
  const selectDevice = (serial: string) => {
    const device = inventory?.Devices.find((candidate) => candidate.ID.Serial === serial);
    if (device) void service.SelectDevice(device.ID).then(setInventory);
  };

  const stage = (index: number, value: number) => {
    const next = { ...pending, DPI: pending.DPI.map((dpi, i) => (i === index ? value : dpi)) };
    void service.StageDPI(next).then((snap) => { setSnapshot(snap); setNotice("Synchronization queued. It will apply after one second of inactivity."); });
  };
  const selectStage = (index: number) => {
    const next = { ...pending, ActiveStage: index + 1 };
    void service.StageDPI(next).then((snap) => { setSnapshot(snap); setNotice("Synchronization queued. It will apply after one second of inactivity."); });
  };
  const reset = () => void service.StageDPI(snapshot.Factory).then((snap) => { setSnapshot(snap); setNotice("Synchronization queued. It will apply after one second of inactivity."); });
  const retryPersistence = () => void service.RetryPersistence().then(setSnapshot);

  return (
    <div className="app">
      <aside className="sidebar">
        <div className="nav-item active">DPI</div>
      </aside>

      <main className="main">
        <div className="top-bar">
          <div className="brand">
            <p className="eyebrow">Attack Shark X6</p>
            <h1>Device control</h1>
          </div>
	          {inventory && inventory.Devices.filter((device) => device.Eligible).length > 1 && (
            <label>
              <span>Mouse device</span>
              <select aria-label="Mouse device" value={inventory.Selected?.ID.Serial ?? ""} onChange={(event) => selectDevice(event.target.value)}>
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
          <div aria-live="polite" className={`status ${connected ? "online" : "offline"}`}>
            <strong>{connected ? "Device available" : "Device unavailable"}</strong>
            <span>{snapshot.Battery == null ? "Battery unavailable" : `Battery ${snapshot.Battery}%`}</span>
            {errorCode && <span role="alert">{feedbackFor(errorCode)}</span>}
          </div>
          {inventory && !ready && <p className="configuration-state" role="status">Receiver detected, configuration unavailable.</p>}
          {snapshot.Firmware && <span role="status" aria-label={snapshot.Firmware === "success" ? "Firmware applied" : snapshot.Firmware === "pending" ? "Firmware synchronization queued" : "Firmware synchronization failed"}>{snapshot.Firmware === "success" ? "Firmware applied" : snapshot.Firmware === "pending" ? "Firmware synchronization queued" : "Firmware synchronization failed"}</span>}
          {snapshot.Persistence && <span role="status">{snapshot.Persistence === "success" ? "Persistence saved" : "Persistence failed"}</span>}
          {snapshot.RetryAvailable && <button type="button" onClick={retryPersistence}>Retry local persistence</button>}
        </div>

        <section className="panel" aria-labelledby="dpi-title">
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
                  onClick={() => selectStage(index)}
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
                          onChange={(event) => stage(index, Number(event.target.value))}
                          style={{ "--fill": `${position(dpi)}%`, "--stage-color": colorFor(index) ?? "#3b82f6" } as CSSProperties}
                        />
                      </div>
                    </div>
                  ))}
            </div>
          </div>

          <button className="reset-btn" disabled={!ready} onClick={reset}>Reset to factory</button>
        </section>

        <p aria-live="polite" className="notice">{notice}</p>
      </main>
    </div>
  );
}

// applyStatusEvent merges one dongle-pushed status delta into the snapshot.
// Battery heartbeats and DPI stage button presses arrive as separate events,
// so only the fields the report carried are written over the current state.
function applyStatusEvent(current: Snapshot, event: StatusEvent): Snapshot {
  const next: Snapshot = { ...current, Applied: { ...current.Applied }, Pending: { ...current.Pending } };
  if (event.Connection !== undefined) next.Connection = event.Connection;
  if (event.Battery != null) next.Battery = event.Battery;
  if (event.ActiveStage != null) {
    next.ObservedStage = event.ActiveStage;
    next.ObservedDPI = ((next.Applied.StageMask >> (event.ActiveStage - 1)) & 1) ? next.Applied.DPI[event.ActiveStage - 1] : null;
  }
  return next;
}

function receivesEvent(selected: Binding | null | undefined, event: StatusEvent): boolean {
  return !selected || !event.ID || (selected.ID.VendorID === event.ID.VendorID && selected.ID.ProductID === event.ID.ProductID && selected.ID.Serial === event.ID.Serial && selected.Path === event.Path && selected.InventoryRevision === event.InventoryRevision);
}
