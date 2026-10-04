import { useEffect, useRef, useState } from "react";
import type { Macro, MacroLibraryService } from "../desktop-contract";

type Draft = { id?: string; name: string; events: Macro["events"] };
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

  const load = async () => {
    const generation = epoch.current;
    const request = ++listRequest.current;
    setLoading(true); setError("");
    try {
      const next = await service.ListMacros();
      if (generation === epoch.current && request === listRequest.current) { setMacros(next); setLoaded(true); }
    } catch (failure) {
      if (generation === epoch.current && request === listRequest.current) setError(errorMessage(failure));
    } finally {
      if (generation === epoch.current && request === listRequest.current) setLoading(false);
    }
  };

  useEffect(() => {
    ++epoch.current;
    submitting.current = false;
    setMacros([]); setLoaded(false); setDraft(undefined); setReading(false); setBusy(false); setConfirmation(false);
    void load();
    return () => { ++epoch.current; ++readRequest.current; ++listRequest.current; };
  }, [service]);

  const select = async (id: string) => {
    if (submitting.current) return;
    const generation = epoch.current;
    const request = ++readRequest.current;
    setReading(true); setError(""); setNotice(""); setConfirmation(false);
    try {
      const next = await service.ReadMacro(id);
      if (generation === epoch.current && request === readRequest.current) setDraft(next);
    } catch (failure) {
      if (generation === epoch.current && request === readRequest.current) setError(errorMessage(failure));
    } finally {
      if (generation === epoch.current && request === readRequest.current) setReading(false);
    }
  };

  const mutate = async (remove: boolean) => {
    if (!draft || submitting.current || reading || loading || (!remove && !draft.name.trim()) || (remove && !confirmation)) return;
    submitting.current = true;
    const generation = epoch.current;
    ++readRequest.current;
    setBusy(true); setError(""); setNotice("");
    try {
      if (remove && draft.id) {
        await service.DeleteMacro(draft.id);
        if (generation !== epoch.current) return;
        setMacros((current) => current.filter((macro) => macro.id !== draft.id));
        setDraft(undefined); setConfirmation(false); setNotice("Macro deleted from library.");
      } else {
        const next = draft.id
          ? await service.UpdateMacro(draft.id, draft.name, draft.events)
          : await service.CreateMacro(draft.name, draft.events);
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

  return {
    macros, draft, loading, loaded, reading, busy, error, confirmation, notice,
    reload: () => { if (!submitting.current) void load(); },
    select: (id: string) => void select(id),
    newMacro: () => {
      if (submitting.current || loading) return;
      ++readRequest.current; setReading(false); setError(""); setNotice(""); setConfirmation(false);
      setDraft({ name: "", events: [] });
    },
    rename: (name: string) => setDraft((current) => current && { ...current, name }),
    save: () => void mutate(false), deleteMacro: () => void mutate(true),
    requestDelete: () => setConfirmation(true), cancelDelete: () => setConfirmation(false),
  };
}
