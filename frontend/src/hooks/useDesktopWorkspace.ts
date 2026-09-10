import { useEffect, useRef, useState } from "react";
import type { Binding, DesktopService, DPIConfig, Snapshot, WorkspaceActions, WorkspaceModel } from "../desktop-contract";

const dpiAppliedNotice = "DPI applied.";
const pollingAppliedNotice = "Polling applied.";

export function useDesktopWorkspace(service: DesktopService): { model: WorkspaceModel; actions: WorkspaceActions } {
  const [snapshot, setSnapshot] = useState<Snapshot>();
  const [polling, setPolling] = useState<WorkspaceModel["polling"]>();
	const [lighting, setLighting] = useState<WorkspaceModel["lighting"]>();
	const [remap, setRemap] = useState<WorkspaceModel["remap"]>();
  const [inventory, setInventory] = useState<WorkspaceModel["inventory"]>();
  const [notice, setNotice] = useState("");
  const [resetConfirmation, setResetConfirmation] = useState(false);
  const [automaticApplyRequests, setAutomaticApplyRequests] = useState(0);
  const selected = useRef<Binding | null>(null);

  useEffect(() => { void service.RefreshStatus().then(setSnapshot); }, [service]);
  useEffect(() => { void service.GetPollingSnapshot().then(setPolling); }, [service]);
	useEffect(() => { void service.GetLightingSnapshot().then(setLighting); }, [service]);
	useEffect(() => { void service.GetRemapSnapshot().then(setRemap); }, [service]);
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
	useEffect(() => service.OnRemapConfiguration((event) => {
		if (receivesEvent(selected.current, event.Binding)) setRemap(event.Snapshot);
	}), [service]);

  const runAutomaticApply = <T,>(request: () => Promise<T>): Promise<T> => {
    setAutomaticApplyRequests((count) => count + 1);
    return request().finally(() => setAutomaticApplyRequests((count) => Math.max(0, count - 1)));
  };
  const stageConfig = (next: DPIConfig) => void runAutomaticApply(() => service.StageDPI(next).then((staged) => {
    setSnapshot(staged);
    if (staged.Error.Code) return;
    return service.ApplyDPI().then((updated) => {
      setSnapshot(updated);
      setNotice(updated.Firmware === "failed" ? "DPI application failed." : dpiAppliedNotice);
      return updated;
    });
  }));
  const actions: WorkspaceActions = {
    selectDevice: (serial) => {
      const device = inventory?.Devices.find((candidate) => candidate.ID.Serial === serial);
      if (device) void service.SelectDevice(device.ID).then(setInventory);
    },
    stageDPI: (index, value) => snapshot && stageConfig({ ...snapshot.Pending, DPI: snapshot.Pending.DPI.map((dpi, current) => current === index ? value : dpi) }),
    selectStage: (index) => snapshot && stageConfig({ ...snapshot.Pending, ActiveStage: index + 1 }),
    stageFeature: (patch) => snapshot && stageConfig({ ...snapshot.Pending, ...patch }),
    stagePollingRate: (rate) => void runAutomaticApply(() => service.StagePollingRate(rate).then((staged) => {
      setPolling(staged);
      if (staged.Error?.Code) return staged;
      return service.ApplyPollingRate().then((updated) => {
        setPolling(updated);
        setNotice(updated.Firmware === "failed" ? "Polling application failed." : pollingAppliedNotice);
        return updated;
      });
    })),
		stageLighting: (selection) => void runAutomaticApply(() => service.StageLighting(selection).then((staged) => {
      setLighting(staged);
      if (staged.Error?.Code) return staged;
      return service.ApplyLighting().then((updated) => {
        setLighting(updated);
        return updated;
      });
		})),
		stageRemap: (button, action) => setRemap((current) => current ? { ...current, Pending: { ...current.Pending, Buttons: current.Pending.Buttons.map((item) => item.Button === button ? { ...item, Action: action, PreservedDefault: "" } : item) } } : current),
		applyRemap: () => remap && void service.ApplyRemap(remap.Pending).then((updated) => { setRemap(updated); setNotice("Button remapping applied."); }),
		discardRemap: () => setRemap((current) => current ? { ...current, Pending: current.Applied } : current),
		requestReset: () => setResetConfirmation(true),
		cancelReset: () => setResetConfirmation(false),
		confirmReset: () => void service.ResetToFactory().then((result) => {
		  setResetConfirmation(false);
		  if (result.Cleanup.State !== "success") {
				setNotice(`Factory reset failed: ${result.Error.Code}.`);
				return;
			}
		  setNotice("Factory reset completed.");
		  void service.GetSnapshot().then(setSnapshot);
		  void service.GetPollingSnapshot().then(setPolling);
		}),
    retryPersistence: () => void service.RetryPersistence().then(setSnapshot),
    retryPollingPersistence: () => void service.RetryPollingPersistence().then(setPolling),
  };
		return { model: { snapshot, polling, lighting, remap, inventory, ready: inventory?.Selected != null, notice, resetConfirmation, automaticApplyBusy: automaticApplyRequests > 0 }, actions };
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
