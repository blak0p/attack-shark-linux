import { useEffect, useRef, useState } from "react";
import type { Macro, MacroLibraryService } from "../desktop-contract";
import { copyMacroEvent, importMacroFile } from "../macros/macro-file";

type DraftEvent = Omit<Macro["events"][number], "delay_ms"> & { delay_ms: number | string };
type Draft = { id?: string; name: string; events: DraftEvent[] };
// Preserve numeric precision across the JavaScript/JSON boundary; this is not a device timing limit.
const validDelay = (value: number | string) => String(value).trim() !== "" && Number.isSafeInteger(Number(value)) && Number(value) >= 0;
const errorMessage = (error: unknown) => error instanceof Error ? error.message : String(error);

export function useMacroLibrary(service: MacroLibraryService) {
  const [macros, setMacros] = useState<Macro[]>([]);
  const [draft, setDraft] = useState<Draft>();
  const [loading, setLoading] = useState(true);
  const [loaded, setLoaded] = useState(false);
  const [reading, setReading] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [confirmation, setConfirmation] = useState(false);
  const [notice, setNotice] = useState("");
  const epoch = useRef(0);
  const readRequest = useRef(0);
  const listRequest = useRef(0);
  const submitting = useRef(false);
  const pendingRead = useRef(false);
  const pendingLoad = useRef(true);
  const pendingConfirmation = useRef(false);
  const recording = useRef<{ token: symbol; draft: Draft; generation: number } | undefined>(undefined);
  const [recordingActive, setRecordingActive] = useState(false);
  const validationError = (() => {
    for (const [index, event] of (draft?.events ?? []).entries()) {
      if (!validDelay(event.delay_ms)) return `Event ${index + 1} delay must be a nonnegative whole number of milliseconds.`;
      try { copyMacroEvent({ ...event, delay_ms: Number(event.delay_ms) }, `Event ${index + 1}`); }
      catch (failure) { return errorMessage(failure); }
    }
    return "";
  })();

  const editEvents = (update: (events: DraftEvent[]) => DraftEvent[]) => {
    if (recording.current || submitting.current || pendingRead.current || pendingConfirmation.current || reading || loading || confirmation) return;
    setDraft((current) => current && { ...current, events: update(current.events) });
    setError(""); setNotice("");
  };

  const load = async () => {
    const generation = epoch.current;
    const request = ++listRequest.current;
    pendingLoad.current = true;
    setLoading(true); setError("");
    try {
      const next = await service.ListMacros();
      if (generation === epoch.current && request === listRequest.current) { setMacros(next); setLoaded(true); }
    } catch (failure) {
      if (generation === epoch.current && request === listRequest.current) setError(errorMessage(failure));
    } finally {
      if (generation === epoch.current && request === listRequest.current) { pendingLoad.current = false; setLoading(false); }
    }
  };

  useEffect(() => {
    ++epoch.current;
    submitting.current = false;
    recording.current = undefined; setRecordingActive(false);
    pendingRead.current = false; pendingConfirmation.current = false;
    setMacros([]); setLoaded(false); setDraft(undefined); setReading(false); setBusy(false); setConfirmation(false);
    void load();
    return () => { recording.current = undefined; ++epoch.current; ++readRequest.current; ++listRequest.current; };
  }, [service]);

  const select = async (id: string) => {
    if (recording.current || submitting.current || loading || pendingConfirmation.current || confirmation) return;
    const generation = epoch.current;
    const request = ++readRequest.current;
    pendingRead.current = true;
    setReading(true); setError(""); setNotice(""); setConfirmation(false);
    try {
      const next = await service.ReadMacro(id);
      if (generation === epoch.current && request === readRequest.current) {
        setDraft(next);
        setMacros((current) => current.map((macro) => macro.id === next.id ? next : macro));
      }
    } catch (failure) {
      if (generation === epoch.current && request === readRequest.current) setError(errorMessage(failure));
    } finally {
      if (generation === epoch.current && request === readRequest.current) { pendingRead.current = false; setReading(false); }
    }
  };

  const mutate = async (remove: boolean) => {
    if (recording.current || !draft || submitting.current || pendingRead.current || reading || loading || (!remove && (pendingConfirmation.current || confirmation || !draft.name.trim() || validationError)) || (remove && !confirmation)) return;
    submitting.current = true;
    const generation = epoch.current;
    ++readRequest.current;
    setBusy(true); setError(""); setNotice("");
    try {
      if (remove && draft.id) {
        await service.DeleteMacro(draft.id);
        if (generation !== epoch.current) return;
        setMacros((current) => current.filter((macro) => macro.id !== draft.id));
        pendingConfirmation.current = false;
        setDraft(undefined); setConfirmation(false); setNotice("Macro deleted from library.");
      } else {
        const events = draft.events.map((event) => ({ ...event, delay_ms: Number(event.delay_ms) }));
        const next = draft.id
          ? await service.UpdateMacro(draft.id, draft.name, events)
          : await service.CreateMacro(draft.name, events);
        if (generation !== epoch.current) return;
        setMacros((current) => draft.id ? current.map((macro) => macro.id === next.id ? next : macro) : [...current, next]);
        setDraft(next); setNotice("Saved to library. No device changes made.");
      }
    } catch (failure) {
      if (generation === epoch.current) setError(errorMessage(failure));
    } finally {
      if (generation === epoch.current) { submitting.current = false; setBusy(false); }
    }
  };

  const importFile = async (readText: () => Promise<string>) => {
    if (recording.current || submitting.current || pendingRead.current || pendingConfirmation.current || reading || loading || confirmation) return;
    submitting.current = true;
    const generation = epoch.current;
    ++readRequest.current;
    setBusy(true); setError(""); setNotice("");
    try {
      const copy = importMacroFile(await readText());
      if (generation !== epoch.current) return;
      const next = await service.CreateMacro(copy.name, copy.events);
      if (generation !== epoch.current) return;
      setMacros((current) => [...current, next]);
      setDraft(next); setNotice("Imported as a new local macro. No device changes made.");
    } catch (failure) {
      if (generation === epoch.current) setError(errorMessage(failure));
    } finally {
      if (generation === epoch.current) { submitting.current = false; setBusy(false); }
    }
  };

  return {
    recordingActive,
    beginRecording: () => {
      if (!draft || recording.current || submitting.current || pendingRead.current || pendingLoad.current || pendingConfirmation.current || reading || loading || confirmation) return;
      const token = Symbol("macro take");
      recording.current = { token, draft, generation: epoch.current };
      setRecordingActive(true);
      return token;
    },
    finishRecording: (token: symbol, events: Macro["events"]) => {
      const session = recording.current;
      if (!session || session.token !== token) return false;
      recording.current = undefined; setRecordingActive(false);
      if (session.generation !== epoch.current) return false;
      setDraft(current => current === session.draft ? { ...current, events: [...current.events, ...events.map(event => ({ ...event }))] } : current);
      return true;
    },
    importFile,
    reportFileError: (failure: unknown) => setError(errorMessage(failure)),
    savedMacro: macros.find((macro) => macro.id === draft?.id),
    macros, draft, loading, loaded, reading, busy, error, confirmation, notice, validationError,
    appendRecordedEvents: (recorded: Macro["events"]) => editEvents((events) => [...events, ...recorded.map((event) => ({ ...event }))]),
    addEvent: () => editEvents((events) => [...events, { type: "mouse_left", action: "down", delay_ms: 0 }]),
    updateEvent: (index: number, update: Partial<DraftEvent>) => editEvents((events) => events.map((event, position) => position === index ? { ...event, ...update } : event)),
    removeEvent: (index: number) => editEvents((events) => events.filter((_, position) => position !== index)),
    moveEvent: (index: number, direction: -1 | 1) => editEvents((events) => {
      const target = index + direction;
      if (index < 0 || index >= events.length || target < 0 || target >= events.length) return events;
      const next = [...events];
      [next[index], next[target]] = [next[target], next[index]];
      return next;
    }),
    reload: () => { if (!recording.current && !submitting.current) void load(); },
    select: (id: string) => void select(id),
    newMacro: () => {
      if (recording.current || submitting.current || loading || pendingConfirmation.current || confirmation) return;
      ++readRequest.current; pendingRead.current = false; setReading(false); setError(""); setNotice(""); setConfirmation(false);
      setDraft({ name: "", events: [] });
    },
    rename: (name: string) => { if (!recording.current && !submitting.current && !pendingRead.current && !pendingConfirmation.current && !reading && !loading && !confirmation) setDraft((current) => current && { ...current, name }); },
    save: () => void mutate(false), deleteMacro: () => void mutate(true),
    requestDelete: () => { if (!recording.current && !submitting.current && !pendingRead.current && !reading && !loading) { pendingConfirmation.current = true; setConfirmation(true); } },
    cancelDelete: () => { pendingConfirmation.current = false; setConfirmation(false); },
  };
}
