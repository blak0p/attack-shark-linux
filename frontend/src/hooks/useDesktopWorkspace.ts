import { useEffect, useRef, useState } from "react";
import type { Binding, DesktopService, DPIConfig, RemapSnapshot, Snapshot, WorkspaceActions, WorkspaceModel } from "../desktop-contract";

const dpiAppliedNotice = "DPI applied.";
const pollingAppliedNotice = "Polling applied.";

export function useDesktopWorkspace(service: DesktopService): { model: WorkspaceModel; actions: WorkspaceActions } {
  const [snapshot, setSnapshot] = useState<Snapshot>();
  const [polling, setPolling] = useState<WorkspaceModel["polling"]>();
  const [debounce, setDebounce] = useState<WorkspaceModel["debounce"]>();
	const [lighting, setLighting] = useState<WorkspaceModel["lighting"]>();
  const [normalSleep, setNormalSleep] = useState<WorkspaceModel["normalSleep"]>();
	const [remap, setRemap] = useState<WorkspaceModel["remap"]>();
  const [inventory, setInventory] = useState<WorkspaceModel["inventory"]>();
  const [notice, setNotice] = useState("");
  const [resetConfirmation, setResetConfirmation] = useState(false);
  const [automaticApplyRequests, setAutomaticApplyRequests] = useState(0);
  const [applicationVersion, setApplicationVersion] = useState<string>();
  const [update, setUpdate] = useState<WorkspaceModel["update"]>();
  const [updateApplying, setUpdateApplying] = useState(false);
  const [updateError, setUpdateError] = useState("");
  const selected = useRef<Binding | null>(null);
  const [remapBusy, setRemapBusy] = useState(false);
  const [remapError, setRemapError] = useState("");
  const remapScope = useRef({ service, epoch: 0, active: true, busy: false });
  const remapValue = useRef(remap);
  remapValue.current = remap;

  const invalidateRemap = () => {
    remapScope.current.epoch++;
    remapScope.current.busy = false;
    setRemapBusy(false);
    setRemapError("");
    setNotice("");
  };
  const refreshRemap = (assignment = false) => {
    const scope = remapScope.current;
    const epoch = ++scope.epoch;
    // Both APIs return the complete remap state; retain the ordinary initial-load contract.
    const request = assignment ? service.GetMacroAssignmentSnapshot() : service.GetRemapSnapshot();
    void request.then((next) => {
      if (scope.active && scope.service === service && scope.epoch === epoch) setRemap(next);
    }).catch((error: unknown) => {
      if (scope.active && scope.service === service && scope.epoch === epoch) setRemapError(error instanceof Error ? error.message : "Unable to load remapping.");
    });
  };

  useEffect(() => {
    const scope = { service, epoch: 0, active: true, busy: false };
    remapScope.current = scope;
    selected.current = null;
    remapValue.current = undefined;
    setRemap(undefined);
    setRemapBusy(false);
    setRemapError("");
    setNotice("");
    return () => { scope.active = false; scope.epoch++; };
  }, [service]);

  const runRemap = (kind: "stage" | "apply" | "discard", request: (current: () => boolean) => Promise<RemapSnapshot>) => {
    const scope = remapScope.current;
    if (!scope.active || scope.service !== service || scope.busy || !selected.current || !remapValue.current) return;
    const epoch = ++scope.epoch;
    const current = () => scope.active && scope.service === service && scope.epoch === epoch;
    scope.busy = true;
    setRemapBusy(true);
    setRemapError("");
    void Promise.resolve().then(() => current() ? request(current) : undefined).then((next) => {
      if (!current() || !next) return;
      remapValue.current = next;
      setRemap(next);
      setRemapError(next.Error.Code);
      if (kind === "apply") {
        setNotice(next.Firmware !== "success"
          ? "Remap transport not confirmed; hardware state is unknown or partial."
          : next.Persistence === "failed"
            ? "Remap transport confirmed; local saving failed; playback and hardware persistence are unverified."
            : "Remap transport confirmed; playback and hardware persistence are unverified.");
      } else {
        setNotice(next.Error.Code ? "Local remap staging failed; draft retained." : kind === "discard" ? "Remap draft discarded locally; hardware unchanged." : "Remap staged locally; not applied to hardware.");
      }
    }).catch((error: unknown) => {
      if (!current()) return;
      setRemapError(error instanceof Error ? error.message : "Remap request failed.");
      setNotice(kind === "apply" ? "Remap transport failed; hardware state is unknown or partial." : "Local remap request failed; draft retained.");
    }).finally(() => {
      if (!current()) return;
      scope.busy = false;
      setRemapBusy(false);
    });
  };

  const refreshSelectedDeviceSnapshots = (assignment = false) => {
    void service.RefreshStatus().then(setSnapshot);
    void service.GetPollingSnapshot().then(setPolling);
    void service.GetDebounceSnapshot().then(setDebounce);
    void service.GetLightingSnapshot().then(setLighting);
    void service.GetNormalSleepSnapshot().then(setNormalSleep);
    refreshRemap(assignment);
  };

  useEffect(() => { refreshSelectedDeviceSnapshots(); }, [service]);
  useEffect(() => {
    let active = true;
    const scope = remapScope.current;
    const epoch = scope.epoch;
    void service.RefreshInventory().then((next) => {
      if (!active || scope.epoch !== epoch) return;
      selected.current = next.Selected;
      setInventory(next);
      refreshSelectedDeviceSnapshots();
    }).catch(() => {});
    return () => { active = false; };
  }, [service]);
  useEffect(() => {
    let cancelled = false;
    if (service.GetApplicationVersion) void service.GetApplicationVersion().then((version) => { if (!cancelled) setApplicationVersion(version); }).catch(() => {});
    return () => { cancelled = true; };
  }, [service]);
  useEffect(() => {
    let cancelled = false;
    if (service.CheckForUpdate) void service.CheckForUpdate().then((available) => { if (!cancelled && available) setUpdate(available); }).catch(() => {});
    return () => { cancelled = true; };
  }, [service]);
  useEffect(() => service.OnStatusEvent((event) => {
    setSnapshot((current) => current && receivesEvent(selected.current, event) ? applyStatusEvent(current, event) : current);
  }), [service]);
  useEffect(() => service.OnConfiguration((event) => {
    if (receivesEvent(selected.current, event.Binding)) {
      setSnapshot(event.Snapshot);
      void service.GetPollingSnapshot().then(setPolling);
      void service.GetDebounceSnapshot().then(setDebounce);
      void service.GetNormalSleepSnapshot().then(setNormalSleep);
    }
  }), [service]);
  useEffect(() => service.OnPollingConfiguration((event) => {
    if (receivesEvent(selected.current, event.Binding)) setPolling(event.Snapshot);
  }), [service]);
	useEffect(() => service.OnRemapConfiguration((event) => {
    if (selected.current && remapScope.current.active && remapScope.current.service === service && !remapScope.current.busy && receivesEvent(selected.current, event.Binding)) {
      // Idle events supersede reads; an in-flight mutation owns its completion.
      invalidateRemap();
      remapValue.current = event.Snapshot;
      setRemap(event.Snapshot);
    }
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
    applyUpdate: () => {
      setUpdateApplying(true); setUpdateError("");
      if (!service.ApplyVerifiedUpdate) { setUpdateApplying(false); return; }
      void service.ApplyVerifiedUpdate().catch((error: unknown) => setUpdateError(error instanceof Error ? error.message : "Unable to restart")).finally(() => setUpdateApplying(false));
    },
    selectDevice: (serial) => {
      const device = inventory?.Devices.find((candidate) => candidate.ID.Serial === serial);
      if (device) {
        invalidateRemap();
        const scope = remapScope.current;
        const epoch = scope.epoch;
        remapValue.current = undefined;
        setRemap(undefined);
        selected.current = null;
        // No binding is actionable until native selection confirms it, including recovery.
        setInventory((current) => current ? { ...current, Selected: null } : current);
        void service.SelectDevice(device.ID).then((next) => {
          if (!scope.active || scope.service !== service || scope.epoch !== epoch) return;
          selected.current = next.Selected;
          setInventory(next);
          refreshSelectedDeviceSnapshots(true);
        }).catch((error: unknown) => {
          if (!scope.active || scope.service !== service || scope.epoch !== epoch) return;
          setRemapError(error instanceof Error ? error.message : "Device selection failed.");
          setNotice("Device selection failed; select the previous device again to recover its workspace.");
        });
      }
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
    stageDebounce: (responseTimeMs) => void runAutomaticApply(() => service.StageDebounce(responseTimeMs).then((staged) => {
      setDebounce(staged);
      if (staged.Error.Code) return staged;
      return service.ApplyDebounce().then((updated) => {
        setDebounce(updated);
        setNotice(updated.Firmware === "failed" ? "Key response time application failed." : "Key response time applied.");
        return updated;
      });
    })),
    retryDebouncePersistence: () => void service.RetryDebouncePersistence().then(setDebounce),
    stageNormalSleep: (minutes) => void runAutomaticApply(() => service.StageNormalSleep(minutes).then((staged) => {
      setNormalSleep(staged);
      if (staged.Error.Code) return staged;
      return service.ApplyNormalSleep().then((updated) => { setNormalSleep(updated); return updated; });
    })),
    retryNormalSleepPersistence: () => void service.RetryNormalSleepPersistence().then(setNormalSleep),
		stageLighting: (selection) => void runAutomaticApply(() => service.StageLighting(selection).then((staged) => {
      setLighting(staged);
      if (staged.Error?.Code) return staged;
      return service.ApplyLighting().then((updated) => {
        setLighting(updated);
        return updated;
      });
		})),
    stageMacroAssignment: (id, button, repeat) => runRemap("stage", () => service.StageMacroAssignment(id, button, repeat)),
    clearButtonMacroAssignment: (button: number) => runRemap("stage", () => {
      if (!service.ClearButtonMacroAssignment) throw new Error("Per-button macro clearing is unavailable");
      return service.ClearButtonMacroAssignment(button);
    }),
    stageRemap: (button, action) => {
      const draft = remapValue.current;
      if (!draft) return;
      const pending = { ...draft.Pending, Buttons: draft.Pending.Buttons.map((item) => item.Button === button ? { ...item, Action: action, PreservedDefault: "" as const } : item) };
      runRemap("stage", async (current) => {
        const staged = await service.StageRemap(pending);
        const hasMacro = staged.MacroDrafts ? Boolean(staged.MacroDrafts[button]) : staged.MacroPending?.Button === button;
        if (!current() || staged.Error.Code || !hasMacro) return staged;
        // Retain the confirmed ordinary draft even if clearing the overlay rejects.
        remapValue.current = staged;
        setRemap(staged);
        // StageRemap preserves the overlay; explicitly clear it only on its target.
        if (!service.ClearButtonMacroAssignment) throw new Error("Per-button macro clearing is unavailable");
        return service.ClearButtonMacroAssignment(button);
      });
    },
    applyRemap: () => { const draft = remapValue.current; if (draft) runRemap("apply", () => service.ApplyRemap(draft.Pending)); },
    discardRemap: () => runRemap("discard", () => service.DiscardRemap()),
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
		return { model: { snapshot, polling, debounce, lighting, normalSleep, remap, inventory, applicationVersion, update, updateApplying, updateError, ready: inventory?.Selected != null, notice, resetConfirmation, automaticApplyBusy: automaticApplyRequests > 0, remapBusy, remapError }, actions };
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
  return !!selected && (!event.ID || (selected.ID.VendorID === event.ID.VendorID && selected.ID.ProductID === event.ID.ProductID && selected.ID.Serial === event.ID.Serial && selected.Path === event.Path && selected.InventoryRevision === event.InventoryRevision));
}
