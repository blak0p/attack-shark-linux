import { useEffect, useState } from "react";

export type DPIConfig = { DPI: number[]; ActiveStage: number; StageMask: number; LiftDistance: number };
export type Snapshot = { Connection: string; Battery?: number | null; Applied: DPIConfig; Pending: DPIConfig; Revision: number; Error: { Code: string } };
export type DesktopService = { GetSnapshot(): Promise<Snapshot>; RefreshStatus(): Promise<Snapshot>; StageDPI(config: DPIConfig): Promise<Snapshot>; ApplyDPI(): Promise<Snapshot> };

export function App({ service }: { service: DesktopService }) {
  const [snapshot, setSnapshot] = useState<Snapshot>();
  const [notice, setNotice] = useState("");
  useEffect(() => { void service.RefreshStatus().then(setSnapshot); }, [service]);
  if (!snapshot) return <main aria-busy="true">Loading configuration…</main>;
  const connected = snapshot.Connection !== "";
  const stage = (index: number, value: number) => {
    const pending = { ...snapshot.Pending, DPI: snapshot.Pending.DPI.map((dpi, i) => i === index ? value : dpi) };
    void service.StageDPI(pending).then(next => { setSnapshot(next); setNotice("Changes are staged locally. Select Apply to send them."); });
  };
  const apply = () => void service.ApplyDPI().then(next => { setSnapshot(next); setNotice(next.Error.Code ? "Apply failed. Your staged values are still available." : "DPI configuration applied and saved."); });
  const refresh = () => void service.RefreshStatus().then(setSnapshot);
  return <main className="app-shell">
    <header><p className="eyebrow">Attack Shark X6</p><h1>Device control</h1><button onClick={refresh}>Refresh status</button></header>
	<section aria-live="polite" className={connected ? "status online" : "status offline"}><strong>{connected ? "Device available" : "Device unavailable"}</strong><span>{snapshot.Battery == null ? "Battery unavailable" : `Battery ${snapshot.Battery}%`}</span>{snapshot.Error.Code && <span role="alert">{snapshot.Error.Code.replaceAll("_", " ")}</span>}</section>
    <section aria-labelledby="dpi-title"><div><p className="eyebrow">Configuration</p><h2 id="dpi-title">DPI stages</h2><p>Edits remain on this screen until you explicitly apply them.</p></div><div className="dpi-grid">{snapshot.Pending.DPI.map((dpi, index) => <label key={index}>Stage {index + 1}<input aria-label={`Stage ${index + 1} DPI`} type="number" min="50" step="50" value={dpi} onChange={event => stage(index, Number(event.target.value))} /></label>)}</div><button className="apply" disabled={!connected} onClick={apply}>Apply DPI</button></section>
    <p aria-live="polite" className="notice">{notice}</p>
  </main>;
}
