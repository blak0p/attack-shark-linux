import { useEffect, useRef, useState, type MouseEvent } from "react";
import type { Macro, MacroLibraryService } from "../../desktop-contract";
import { useMacroLibrary } from "../../hooks/useMacroLibrary";
import { exportMacroFile, MAX_MACRO_FILE_BYTES } from "../../macros/macro-file";
import "./MacroManagerPanel.css";

type Library = ReturnType<typeof useMacroLibrary>;

// Display-only grouping: never reconstruct or normalize the draft's raw events.
function timeline(events: NonNullable<Library["draft"]>["events"]) {
  const rows: { index: number; count: number }[] = [];
  for (let index = 0; index < events.length;) {
    const down = events[index], up = events[index + 1];
    const paired = down.action === "down" && down.delay_ms === 0 && up?.action === "up"
      && up.delay_ms === 0 && up.type === down.type;
    rows.push({ index, count: paired ? 2 : 1 });
    index += paired ? 2 : 1;
  }
  return rows;
}

// DOM button order differs from the protocol/editor order: middle is button 1.
const mouseButtons: { button: number; type: Macro["events"][number]["type"]; label: string }[] = [
  { button: 0, type: "mouse_left", label: "Left mouse" },
  { button: 2, type: "mouse_right", label: "Right mouse" },
  { button: 1, type: "mouse_middle", label: "Middle mouse" },
  { button: 3, type: "mouse_back", label: "Back mouse" },
  { button: 4, type: "mouse_forward", label: "Forward mouse" },
];

type AssignmentShortcut = { onAssign?(id: string): void; assignmentReady?: boolean };

export function MacroManagerPanel({ service, library, ...shortcut }: { service: MacroLibraryService; library?: Library } & AssignmentShortcut) {
  return library ? <MacroManagerEditor library={library} service={service} {...shortcut} /> : <LocalMacroManager service={service} {...shortcut} />;
}

function LocalMacroManager({ service, ...shortcut }: { service: MacroLibraryService } & AssignmentShortcut) {
  const library = useMacroLibrary(service);
  return <MacroManagerEditor library={library} service={service} {...shortcut} />;
}

function MacroManagerEditor({ library, service, onAssign, assignmentReady = false }: { library: Library; service: MacroLibraryService } & AssignmentShortcut) {
  const { macros, draft, loading, loaded, reading, busy, error, confirmation, notice, validationError } = library;
  const unavailable = busy || reading || loading || confirmation;
  const [armed, setArmed] = useState(false);
  const [clickType, setClickType] = useState<Macro["events"][number]["type"]>("mouse_left");
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const rows = timeline(draft?.events ?? []);
  const compatible = rows.length >= 1 && rows.length <= 2 && rows.every((row) => row.count === 2);
  const [recordingMessage, setRecordingMessage] = useState("");
  const [recordingError, setRecordingError] = useState("");
  const zone = useRef<HTMLDivElement>(null);
  const session = useRef<{ events: Macro["events"]; held: Set<number> } | undefined>(undefined);
  const editorDisabled = unavailable || armed;

  const discardRecording = (message = "Recording discarded. Draft unchanged.") => {
    if (!session.current) return;
    session.current = undefined;
    setArmed(false); setRecordingMessage(""); setRecordingError(message);
  };
  // No global listeners: focus/exit guards belong only to the visible zone.
  useEffect(() => {
    discardRecording();
  }, [service, draft?.id, reading, loading, confirmation]);
  useEffect(() => () => { session.current = undefined; }, []);

  const armRecording = () => {
    if (!draft || unavailable) return;
    session.current = { events: [], held: new Set() };
    setArmed(true); setRecordingError(""); setRecordingMessage("Recording armed. Click only inside the zone.");
    zone.current?.focus();
  };
  const record = (event: MouseEvent<HTMLDivElement>, action: "down" | "up") => {
    const current = session.current;
    const button = mouseButtons.find((candidate) => candidate.button === event.button);
    if (!current || unavailable || !button) return;
    // Cancel armed-zone defaults even for duplicate downs and unmatched ups.
    event.preventDefault();
    if ((action === "down") === current.held.has(event.button)) return;
    if (action === "down") event.currentTarget.focus();
    // Schema compatibility only: zero requests no pause locally, not instant firmware playback.
    if (action === "down") current.held.add(event.button); else current.held.delete(event.button);
    current.events.push({ type: button.type, action, delay_ms: 0 });
    setRecordingMessage(`Recording armed · ${current.events.length} captured events (not saved).`);
  };
  const preventRecordingDefault = (event: MouseEvent<HTMLDivElement>) => {
    if (session.current && !unavailable && mouseButtons.some((button) => button.button === event.button)) {
      event.preventDefault();
    }
  };
  const stopRecording = () => {
    const current = session.current;
    if (!current) return;
    if (current.held.size) { discardRecording("Recording discarded: a mouse button was not released inside the zone. Draft unchanged."); return; }
    session.current = undefined;
    setArmed(false); setRecordingError("");
    library.appendRecordedEvents(current.events);
    setRecordingMessage(`${current.events.length} recorded events appended to draft. Save explicitly to keep them.`);
  };
  const upload = (file?: File) => {
    if (!file || editorDisabled) return;
    void library.importFile(async () => {
      if (file.size > MAX_MACRO_FILE_BYTES) throw new Error("Macro JSON file is too large (local file safety budget: 1 MiB).");
      return file.text();
    });
  };
  const download = () => {
    if (!library.savedMacro || editorDisabled) return;
    try {
      const text = exportMacroFile(library.savedMacro);
      const url = URL.createObjectURL(new Blob([text], { type: "application/json" }));
      try {
        const link = document.createElement("a");
        link.href = url; link.download = "macro.json";
        link.click();
      } finally { URL.revokeObjectURL(url); }
    } catch (failure) { library.reportFileError(failure); }
  };
  return (
    <div className="macro-manager" aria-busy={busy}>
      <article className="card macro-library" aria-label="Macro library">
        <div className="card-head">
          <h2>Macro library</h2>
          <button type="button" className="button primary" disabled={busy || loading} onClick={() => { discardRecording(); library.newMacro(); }}>New macro</button>
        </div>
        <p className="hint">Shared across devices. Available offline.</p>
        {loading && <p role="status">Loading library…</p>}
        {!loading && loaded && macros.length === 0 && <p className="hint">No macros in your library yet.</p>}
        <div className="macro-cards">
          {macros.map((macro) => (
            <button type="button" key={macro.id} className="macro-card" disabled={busy || loading}
              aria-pressed={draft?.id === macro.id} aria-label={`${macro.name} · ${macro.events.length} events`}
              onClick={() => { discardRecording(); library.select(macro.id); }}>
              <strong>{macro.name}</strong><span>{macro.events.length} events · Local library</span>
            </button>
          ))}
        </div>
        {error && !loading && <button type="button" className="button" disabled={busy} onClick={library.reload}>Retry library</button>}
      </article>
      <article className="card macro-detail" aria-label="Macro details">
        <header className="macro-editor-head">
          <div><p className="macro-kicker">Macro editor</p><h2>{draft?.name.trim() || "Untitled macro"}</h2></div>
          <details className="macro-options">
            <summary>Macro options</summary>
            <div className="macro-options-body">
              <label className="macro-name">Import macro JSON
                <input type="file" accept=".json,application/json" disabled={editorDisabled}
                  onChange={(event) => { const file = event.target.files?.[0]; event.target.value = ""; upload(file); }} />
              </label>
              <p className="hint">One version-1 JSON macro per file, up to 1 MiB (local file safety budget, not a device limit). Import creates a new copy, even with a duplicate name; existing macros are never replaced.</p>
              <button type="button" className="button" disabled={editorDisabled || !library.savedMacro} onClick={download}>Export saved macro</button>
              <p className="hint">Exports saved name and events without the local ID. Unsaved editor changes are not exported.</p>
            </div>
          </details>
        </header>
        {onAssign && <div className="macro-actions">
          <button type="button" className="button" disabled={editorDisabled || !loaded || !!error || !assignmentReady || !library.savedMacro || !macros.some((macro) => macro.id === library.savedMacro?.id)}
            onClick={() => {
              const saved = library.savedMacro;
              if (!editorDisabled && loaded && !error && assignmentReady && saved && macros.some((macro) => macro.id === saved.id)) {
                discardRecording();
                onAssign(saved.id);
              }
            }}>Assign to button</button>
          <p className="hint">Uses the saved macro only. Unsaved editor changes are not saved or included; they remain here. Choose a destination and repetitions in Button remapping, then explicitly Stage and Apply.</p>
          {!assignmentReady && <p className="hint">Connect and select a device before assigning. Local editing remains available offline.</p>}
        </div>}
        {reading && <p role="status">Loading macro…</p>}
        {!draft ? <p className="macro-empty">Select a macro or create a new one.</p> : (
          <>
            <label className="macro-name">Macro name
              <input className="input" value={draft.name} disabled={editorDisabled}
                onChange={(event) => library.rename(event.target.value)} />
            </label>
            <section className="macro-composer" aria-label="Complete click composer">
              <h3>Add a complete click</h3>
              <div className="macro-click-types" role="group" aria-label="Click type">
                {mouseButtons.map((button) => <button key={button.type} type="button" className="button"
                  aria-pressed={clickType === button.type} disabled={editorDisabled}
                  onClick={() => setClickType(button.type)}>{button.label.replace(" mouse", "")} click</button>)}
              </div>
              <button type="button" className="button primary" disabled={editorDisabled} onClick={() => library.appendRecordedEvents([
                { type: clickType, action: "down", delay_ms: 0 },
                { type: clickType, action: "up", delay_ms: 0 },
              ])}>Add click</button>
            </section>
            <section className="macro-timeline" aria-label="Sequence">
              <div className="macro-sequence-head"><h3>Sequence</h3><span className="hint">{rows.length} actions · {draft.events.length} events</span></div>
              <ol aria-label="Ordered timeline">{rows.map(({ index, count }, position) => {
                const event = draft.events[index];
                const label = mouseButtons.find((button) => button.type === event.type)?.label;
                return <li key={index}>
                  <span className="macro-step">{String(position + 1).padStart(2, "0")}</span>
                  <div><strong>{label?.replace(" mouse", "")}{count === 2 ? " click" : ` · ${event.action}`}</strong>
                    <small>{count === 2 ? "Complete click · zero local delay" : `Raw event ${index + 1} · ${event.delay_ms} ms`}</small></div>
                </li>;
              })}</ol>
              {!rows.length && <p className="macro-empty">No actions yet. Add a click above. Empty macros can be saved.</p>}
              <p className={`macro-compatibility${compatible ? " compatible" : ""}`}>{compatible
                ? "Eligible for assignment after saving. Playback remains unverified."
                : "Local sequence only · this layout cannot be assigned to the device."}</p>
            </section>
            <details className="macro-advanced" open={advancedOpen} onToggle={(event) => {
              setAdvancedOpen(event.currentTarget.open);
              if (!event.currentTarget.open) discardRecording();
            }}>
              <summary>Advanced events and recording</summary>
            <section className="group" aria-label="Local browser recording">
              <h3>Local browser recording</h3>
              <p className="hint">Left/right/middle/back/forward mouse buttons only, inside the zone below. Stop appends balanced events to this draft; an incomplete session is discarded. No hardware capture or verified X6 playback timing.</p>
              <p className="hint">Records event order only, without configurable pauses. New events use a local no-pause-request placeholder, not a guarantee of instantaneous device playback. Leaving with a button held or losing zone focus discards the session.</p>
              <div className="macro-actions">
                <button type="button" className="button" disabled={unavailable || armed} onClick={armRecording}>Arm recording</button>
                <button type="button" className="button" disabled={!armed} onMouseDown={(event) => event.preventDefault()} onClick={stopRecording}>Stop recording</button>
              </div>
              <div ref={zone} role="region" aria-label="Mouse recording zone" tabIndex={0}
                className={`macro-recording-zone${armed ? " armed" : ""}`}
                onMouseDown={(event) => record(event, "down")} onMouseUp={(event) => record(event, "up")}
                onMouseLeave={() => { if (session.current?.held.size) discardRecording(); }}
                onBlur={() => discardRecording()}
                onAuxClick={preventRecordingDefault} onContextMenu={preventRecordingDefault}>
                {armed ? "Recording armed — press and release here" : "Recording disarmed"}
              </div>
              {recordingMessage && <p role="status">{recordingMessage}</p>}
              {recordingError && <p role="alert" className="macro-error">{recordingError}</p>}
            </section>
            <section className="group macro-events" aria-label="Ordered events">
              <h3>Events · {draft.events.length}</h3>
              <p className="hint">Ordered left/right/middle/back/forward press and release events.</p>
              {draft.events.some((event) => Number(event.delay_ms) !== 0) && (
                <p className="hint">Existing delay values are preserved for compatibility, not editable here. Vendor pause support is unverified; Button remapping rejects unsupported timing rather than discarding it.</p>
              )}
              <button type="button" className="button" disabled={editorDisabled} onClick={library.addEvent}>Add event</button>
              {draft.events.length === 0 ? <p className="macro-empty">No events yet. Empty macros can be saved.</p> : (
                <ol>{draft.events.map((event, index) => (
                  <li key={index}>
                    <span>{mouseButtons.find((button) => button.type === event.type)?.label} · {event.action}</span>
                    <small className="macro-raw-delay">Delay: {event.delay_ms} ms (preserved)</small>
                    <div className="macro-event-fields">
                      <label>Event {index + 1} button
                        <select disabled={editorDisabled} value={event.type}
                          onChange={(change) => library.updateEvent(index, { type: change.target.value as typeof event.type })}>
                          {mouseButtons.map((button) => <option key={button.type} value={button.type}>{button.label}</option>)}
                        </select>
                      </label>
                      <label>Event {index + 1} action
                        <select disabled={editorDisabled} value={event.action}
                          onChange={(change) => library.updateEvent(index, { action: change.target.value as typeof event.action })}>
                          <option value="down">Press (down)</option><option value="up">Release (up)</option>
                        </select>
                      </label>
                    </div>
                    <div className="macro-actions">
                      <button type="button" className="button" aria-label={`Move event ${index + 1} up`}
                        disabled={editorDisabled || index === 0} onClick={() => library.moveEvent(index, -1)}>Move up</button>
                      <button type="button" className="button" aria-label={`Move event ${index + 1} down`}
                        disabled={editorDisabled || index === draft.events.length - 1} onClick={() => library.moveEvent(index, 1)}>Move down</button>
                      <button type="button" className="button" aria-label={`Remove event ${index + 1}`}
                        disabled={editorDisabled} onClick={() => library.removeEvent(index)}>Remove</button>
                    </div>
                  </li>
                ))}</ol>
              )}
            </section>
            </details>
            <details className="macro-help"><summary>Saving and assignment</summary>
              <p className="hint">Save to library only; no device changes. To assign a saved macro, open Button remapping, choose its name and fixed repetitions (1–255), then Apply remap. Playback and device persistence remain unverified.</p>
              <p className="hint">Backend admission: one or two complete zero-delay clicks using left/right/middle/back/forward actions. Local save preserves other event layouts and delays; these cannot be assigned. Transport confirmation does not prove playback or device persistence.</p>
            </details>
            <div className="macro-actions macro-save-footer">
              <p className="hint">Local library · no device changes</p>
              <button type="button" className="button primary" disabled={editorDisabled || !draft.name.trim() || !!validationError} onClick={library.save}>Save to library</button>
              {draft.id && <button type="button" className="button" disabled={editorDisabled} onClick={library.requestDelete}>Delete macro</button>}
            </div>
            {confirmation && (
              <div role="dialog" aria-modal="false" aria-label="Delete macro confirmation" className="group">
                <p>Delete “{draft.name}” from the local library? This cannot be undone.</p>
                <div className="macro-actions">
                  <button type="button" className="button" disabled={busy} onClick={library.cancelDelete}>Cancel</button>
                  <button type="button" className="button danger" disabled={busy} onClick={library.deleteMacro}>Confirm delete</button>
                </div>
              </div>
            )}
          </>
        )}
        {busy && <p role="status">Saving local library…</p>}
        {(validationError || error) && <p role="alert" className="macro-error">{validationError || error}</p>}
        {notice && <p role="status">{notice}</p>}
      </article>
    </div>
  );
}
