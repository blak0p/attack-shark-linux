import { useEffect, useRef, useState } from "react";
import type { Binding, DesktopService, DPIConfig, LightingSelection, Snapshot, WorkspaceActions, WorkspaceModel } from "../desktop-contract";

const syncNotice = "Synchronization queued. It will apply after one second of inactivity.";

export function useDesktopWorkspace(service: DesktopService): { model: WorkspaceModel; actions: WorkspaceActions } {
  const [snapshot, setSnapshot] = useState<Snapshot>();
  const [polling, setPolling] = useState<WorkspaceModel["polling"]>();
  const [lighting, setLighting] = useState<WorkspaceModel["lighting"]>();
  const [inventory, setInventory] = useState<WorkspaceModel["inventory"]>();
  const [notice, setNotice] = useState("");
  const selected = useRef<Binding | null>(null);

  useEffect(() => { void service.RefreshStatus().then(setSnapshot); }, [service]);
  useEffect(() => { void service.GetPollingSnapshot().then(setPolling); }, [service]);
  useEffect(() => { void service.GetLightingSnapshot().then(setLighting); }, [service]);
  useEffect(() => { void service.RefreshInventory().then(setInventory); }, [service]);
  useEffect(() => { selected.current = inventory?.Selected ?? null; }, [inventory]);
  useEffect(() => service.OnStatusEvent((event) => {
    setSnapshot((current) => current && receivesEvent(selected.current, event) ? applyStatusEvent(current, event) : current);
  }), [service]);
  useEffect(() => service.OnConfiguration((event) => {
    if (receivesEvent(selected.current, event.Binding)) {
      setSnapshot(event.Snapshot);
      void service.GetPollingSnapshot().then(setPolling);
    }
  }), [service]);
  useEffect(() => service.OnPollingConfiguration((event) => {
    if (receivesEvent(selected.current, event.Binding)) setPolling(event.Snapshot);
  }), [service]);

  const stageConfig = (next: DPIConfig) => void service.StageDPI(next).then((updated) => {
    setSnapshot(updated);
    setNotice(syncNotice);
  });
  const actions: WorkspaceActions = {
    selectDevice: (serial) => {
      const device = inventory?.Devices.find((candidate) => candidate.ID.Serial === serial);
      if (device) void service.SelectDevice(device.ID).then(setInventory);
    },
    stageDPI: (index, value) => snapshot && stageConfig({ ...snapshot.Pending, DPI: snapshot.Pending.DPI.map((dpi, current) => current === index ? value : dpi) }),
    selectStage: (index) => snapshot && stageConfig({ ...snapshot.Pending, ActiveStage: index + 1 }),
    stageFeature: (patch) => snapshot && stageConfig({ ...snapshot.Pending, ...patch }),
    stagePollingRate: (rate) => void service.StagePollingRate(rate).then((updated) => {
      setPolling(updated);
      setNotice("Polling synchronization queued. It will apply after one second of inactivity.");
    }),
    stageLighting: (selection) => void service.StageLighting(selection).then((updated) => {
      setLighting(updated);
      setNotice("Lighting selection staged. Apply lighting to send it to the device.");
    }),
    applyLighting: () => void service.ApplyLighting().then(setLighting),
    reset: () => void service.ResetToFactory().then((updated) => {
      setSnapshot(updated);
      return service.GetPollingSnapshot();
    }).then((updated) => {
      setPolling(updated);
      setNotice("Factory defaults queued. They will apply after one second of inactivity.");
    }),
    retryPersistence: () => void service.RetryPersistence().then(setSnapshot),
    retryPollingPersistence: () => void service.RetryPollingPersistence().then(setPolling),
  };
  return { model: { snapshot, polling, lighting, inventory, ready: inventory?.Selected != null, notice }, actions };
}

function applyStatusEvent(current: Snapshot, event: { Connection?: string; Battery?: number | null; ActiveStage?: number | null }): Snapshot {
  const next = { ...current, Applied: { ...current.Applied }, Pending: { ...current.Pending } };
  if (event.Connection !== undefined) next.Connection = event.Connection;
  if (event.Battery != null) next.Battery = event.Battery;
  if (event.ActiveStage != null) {
    next.ObservedStage = event.ActiveStage;
    next.ObservedDPI = ((next.Applied.StageMask >> (event.ActiveStage - 1)) & 1) ? next.Applied.DPI[event.ActiveStage - 1] : null;
  }
  return next;
}

function receivesEvent(selected: Binding | null, event: Partial<Binding>): boolean {
  return !selected || !event.ID || (selected.ID.VendorID === event.ID.VendorID && selected.ID.ProductID === event.ID.ProductID && selected.ID.Serial === event.ID.Serial && selected.Path === event.Path && selected.InventoryRevision === event.InventoryRevision);
}
