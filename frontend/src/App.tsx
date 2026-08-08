import { useEffect, useState, type CSSProperties } from "react";

export type DPIConfig = { DPI: number[]; ActiveStage: number; StageMask: number; LiftDistance: number; Colors?: number[][]; AngleControl?: boolean; RippleControl?: boolean };
export type Snapshot = { Connection: string; Battery?: number | null; Applied: DPIConfig; Pending: DPIConfig; Factory: DPIConfig; Revision: number; Error: { Code: string } };
export type StatusEvent = { Connection?: string; Battery?: number | null; ActiveStage?: number | null };
export type DesktopService = { GetSnapshot(): Promise<Snapshot>; RefreshStatus(): Promise<Snapshot>; StageDPI(config: DPIConfig): Promise<Snapshot>; ApplyDPI(): Promise<Snapshot>; OnStatusEvent(callback: (event: StatusEvent) => void): () => void };

// Protocol-derived bounds: the official Windows app caps its DPI slider at
// 520 (= DPI/50), matching the PAW3395 sensor's 26000 max (docs/app-x6.md).
const DPI_MIN = 50;
const DPI_MAX = 26000;
const DPI_STEP = 50;

const position = (value: number) => Math.round(Math.max(0, Math.min(1, (value - DPI_MIN) / (DPI_MAX - DPI_MIN))) * 1000) / 10;

export function App({ service }: { service: DesktopService }) {
  const [snapshot, setSnapshot] = useState<Snapshot>();
  const [notice, setNotice] = useState("");
  useEffect(() => { void service.RefreshStatus().then(setSnapshot); }, [service]);
  useEffect(() => {
    const unsubscribe = service.OnStatusEvent((event) => {
      setSnapshot((current) => (current ? applyStatusEvent(current, event) : current));
    });
    return unsubscribe;
  }, [service]);
  if (!snapshot) return <main className="app-shell" aria-busy="true">Loading configuration…</main>;

  const connected = snapshot.Connection !== "";
  const pending = snapshot.Pending;
  const stages = pending.DPI.map((dpi, index) => ({ index, dpi })).filter(({ index }) => ((pending.StageMask ?? 0) >> index) & 1);
  const activeIndex = pending.ActiveStage >= 1 && pending.ActiveStage <= pending.DPI.length ? pending.ActiveStage - 1 : -1;
  const activeDPI = activeIndex >= 0 ? pending.DPI[activeIndex] : null;
  const activeColor = activeIndex >= 0 && pending.Colors ? pending.Colors[activeIndex] : null;
  const colorFor = (index: number) => (pending.Colors && pending.Colors[index] ? `rgb(${pending.Colors[index].join(", ")})` : null);

  const stage = (index: number, value: number) => {
    const next = { ...pending, DPI: pending.DPI.map((dpi, i) => (i === index ? value : dpi)) };
    void service.StageDPI(next).then((snap) => { setSnapshot(snap); setNotice("Changes are staged locally. Select Save to Device to send them."); });
  };
  const selectStage = (index: number) => {
    const next = { ...pending, ActiveStage: index + 1 };
    void service.StageDPI(next).then((snap) => { setSnapshot(snap); setNotice("Changes are staged locally. Select Save to Device to send them."); });
  };
  const reset = () => void service.StageDPI(snapshot.Factory).then((snap) => { setSnapshot(snap); setNotice("Factory defaults staged. Select Save to Device to send them."); });
  const apply = () => void service.ApplyDPI().then((next) => { setSnapshot(next); setNotice(next.Error.Code ? "Save failed. Your staged values are still available." : "DPI configuration applied and saved."); });

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
          <div aria-live="polite" className={`status ${connected ? "online" : "offline"}`}>
            <strong>{connected ? "Device available" : "Device unavailable"}</strong>
            <span>{snapshot.Battery == null ? "Battery unavailable" : `Battery ${snapshot.Battery}%`}</span>
            {snapshot.Error.Code && <span role="alert">{snapshot.Error.Code.replaceAll("_", " ")}</span>}
          </div>
          <button className="save-btn" disabled={!connected} onClick={apply}>
            <span>Save to Device</span>
            <small>(Commit to Firmware)</small>
          </button>
        </div>

        <section className="panel" aria-labelledby="dpi-title">
          <div className="panel-header">
            <div>
              <h2 className="panel-title" id="dpi-title">DPI & Performance</h2>
              <div className="panel-subtitle">
                {activeDPI != null ? <>Active DPI: {activeDPI}{activeColor && <span className="color-swatch" style={{ background: `rgb(${activeColor.join(", ")})` }} />}</> : "No active DPI stage"}
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
                          onChange={(event) => stage(index, Number(event.target.value))}
                          style={{ "--fill": `${position(dpi)}%`, "--stage-color": colorFor(index) ?? "#3b82f6" } as CSSProperties}
                        />
                      </div>
                    </div>
                  ))}
            </div>
          </div>

          <button className="reset-btn" onClick={reset}>Reset to factory</button>
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
    next.Applied.ActiveStage = event.ActiveStage;
    next.Pending.ActiveStage = event.ActiveStage;
  }
  return next;
}
