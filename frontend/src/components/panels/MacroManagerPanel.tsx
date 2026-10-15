import { useEffect, useId, useRef, useState } from "react";
import { OperationFeedback } from "../OperationFeedback";
import { GnomeSelect } from "./GnomeSelect";
import { captureInput, startTake, stopTake, type DelayPolicy, type Take } from "../../macros/recorder";
import type { Macro, MacroLibraryService } from "../../desktop-contract";
import { useMacroLibrary } from "../../hooks/useMacroLibrary";
import { deviceAdmission } from "../../macros/device-admission";
import { exportMacroFile, MAX_MACRO_FILE_BYTES } from "../../macros/macro-file";
import "./MacroManagerPanel.css";

type Library = ReturnType<typeof useMacroLibrary>;

// Presentation only: retain the original physical identity in the draft.
function keyboardLabel(code?: string) {
  if (!code) return "Keyboard (missing identity)";
  return `Key ${code.replace(/^(Key|Digit)(?=[A-Z0-9]$)/, "")}`;
}

// Display-only grouping: never reconstruct or normalize the draft's raw events.
function timeline(events: NonNullable<Library["draft"]>["events"]) {
  const rows: { index: number; count: number }[] = [];
  for (let index = 0; index < events.length;) {
    const down = events[index], up = events[index + 1];
    const sameInput = down.type === "keyboard"
      ? !!down.key_code?.trim() && up?.key_code === down.key_code
      : mouseButtons.some((button) => button.type === down.type)
        && !down.key_code && !up?.key_code;
    const paired = sameInput && down.action === "down" && up?.action === "up" && up.type === down.type;
    rows.push({ index, count: paired ? 2 : 1 });
    index += paired ? 2 : 1;
  }
  return rows;
}

const mouseButtons: { button: number; type: Macro["events"][number]["type"]; label: string }[] = [
  { button: 0, type: "mouse_left", label: "Left mouse" },
  { button: 2, type: "mouse_right", label: "Right mouse" },
  { button: 1, type: "mouse_middle", label: "Middle mouse" },
  { button: 3, type: "mouse_back", label: "Back mouse" },
  { button: 4, type: "mouse_forward", label: "Forward mouse" },
];

export function MacroManagerPanel({
  service,
  library,
  feedbackManaged = false,
  ...rest
}: {
  service: MacroLibraryService;
  library?: Library;
  feedbackManaged?: boolean;
  onAssign?(id: string): void;
  assignmentReady?: boolean;
}) {
  return library ? <MacroManagerEditor library={library} service={service} feedbackManaged={feedbackManaged} /> : <LocalMacroManager service={service} />;
}

function LocalMacroManager({ service }: { service: MacroLibraryService }) {
  const library = useMacroLibrary(service);
  return <MacroManagerEditor library={library} service={service} />;
}

function MacroManagerEditor({ library, service, feedbackManaged = false }: { library: Library; service: MacroLibraryService; feedbackManaged?: boolean }) {
  const { macros, draft, loading, loaded, reading, busy, error, confirmation, notice, validationError } = library;
  const unavailable = busy || reading || loading || confirmation;
  const [clickType, setClickType] = useState<Macro["events"][number]["type"]>("mouse_left");
  const fileInputRef = useRef<HTMLInputElement>(null);
  const rows = timeline(draft?.events ?? []);
  const admission = deviceAdmission(draft?.events ?? []);
  const compatible = admission.eligible;
  const editorDisabled = unavailable || library.recordingActive;
  const navigationDisabled = busy || loading || confirmation || library.recordingActive;
  const [policy, setPolicy] = useState<DelayPolicy>("fixed");
  const [captureTab, setCaptureTab] = useState(false);
  const startRef = useRef<HTMLButtonElement>(null);
  const focusRequest = useRef<{ frame: number; token: symbol } | undefined>(undefined);
  const cancelFocus = () => {
    if (focusRequest.current) cancelAnimationFrame(focusRequest.current.frame);
    focusRequest.current = undefined;
  };
  const restoreCaptureFocus = () => {
    cancelFocus();
    const token = Symbol("Tab focus");
    const surface = captureRef.current;
    const button = startRef.current;
    const frame = requestAnimationFrame(() => {
      if (focusRequest.current?.token !== token) return;
      focusRequest.current = undefined;
      // A user moving focus or replacing the editor owns the next destination.
      if (!session.current && button === startRef.current && button?.isConnected && !button.disabled &&
          surface === captureRef.current && document.activeElement === surface) button.focus();
    });
    focusRequest.current = { frame, token };
  };
  const policyID = useId();
  const [takeStatus, setTakeStatus] = useState("");
  const [pressCount, setPressCount] = useState(0);
  const captureRef = useRef<HTMLDivElement>(null);
  const session = useRef<{ take: Take; token: symbol; finish: Library["finishRecording"] } | undefined>(undefined);
  const stop = () => {
    const current = session.current;
    if (!current) return;
    session.current = undefined;
    const events = stopTake(current.take);
    const appended = current.finish(current.token, events);
    const presses = events.filter(event => event.action === "down").length;
    setTakeStatus(appended ? `${presses} ${presses === 1 ? "press" : "presses"}` : "Recording ended; the draft changed, so no events were delivered.");
    setPressCount(0);
  };
  useEffect(() => () => {
    cancelFocus();
    const current = session.current;
    session.current = undefined;
    if (current) current.finish(current.token, stopTake(current.take));
  }, [service, draft?.id]);
  useEffect(() => {
    if (session.current && (!library.recordingActive || unavailable)) stop();
  }, [library.recordingActive, unavailable]);
  const capture = (input: Omit<Macro["events"][number], "delay_ms">) => {
    const current = session.current;
    if (!current || unavailable || !library.recordingActive) return false;
    const next = captureInput(current.take, input, input.action === "down" ? performance.now() : 0);
    if (next === current.take) return false;
    current.take = next;
    setPressCount(next.events.filter(event => event.action === "down").length);
    return true;
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
        link.href = url;
        link.download = "macro.json";
        link.click();
      } finally {
        URL.revokeObjectURL(url);
      }
    } catch (failure) {
      library.reportFileError(failure);
    }
  };

  return (
    <div className="macro-manager" aria-busy={busy}>
      <article className="card macro-library" aria-label="Macro library">
        <div className="card-head">
          <h2>Macro library</h2>
          <button type="button" className="button primary" disabled={navigationDisabled} onClick={() => { cancelFocus(); library.newMacro(); }}>New macro</button>
        </div>

        {!loading && loaded && macros.length === 0 && <p className="hint">No macros in your library yet.</p>}
        <div className="macro-cards">
          {macros.map((macro) => (
            <button
              type="button"
              key={macro.id}
              className="macro-card"
              disabled={navigationDisabled}
              aria-pressed={draft?.id === macro.id}
              aria-label={`${macro.name} · ${macro.events.length} events`}
              onClick={() => { cancelFocus(); library.select(macro.id); }}
            >
              <strong>{macro.name}</strong>
              <span>{macro.events.length} events · Local library</span>
            </button>
          ))}
        </div>
        {error && !loading && <button type="button" className="button" disabled={editorDisabled} onClick={library.reload}>Retry library</button>}
      </article>

      <article className="card macro-detail" aria-label="Macro details">
        <header className="macro-editor-head">
          <div>
            <p className="macro-kicker">Macro editor</p>
            <h2>{draft?.name.trim() || "Untitled macro"}</h2>
          </div>
          <details className="macro-options">
            <summary>Macro options</summary>
            <div className="macro-options-body">
              <div className="macro-file-import">
                <input
                  ref={fileInputRef}
                  type="file"
                  accept=".json,application/json"
                  style={{ display: "none" }}
                  aria-label="Import macro JSON"
                  disabled={editorDisabled}
                  onChange={(event) => {
                    const file = event.target.files?.[0];
                    event.target.value = "";
                    upload(file);
                  }}
                />
                <button
                  type="button"
                  className="button"
                  disabled={editorDisabled}
                  onClick={() => fileInputRef.current?.click()}
                >
                  Import JSON
                </button>
              </div>
              <button type="button" className="button" disabled={editorDisabled || !library.savedMacro} onClick={download}>Export saved macro</button>
            </div>
          </details>
        </header>


        {!draft ? (
          <p className="macro-empty">Select a macro or create a new one.</p>
        ) : (
          <>
            <label className="macro-name">Macro name
              <input
                className="input"
                value={draft.name}
                disabled={editorDisabled}
                onChange={(event) => library.rename(event.target.value)}
              />
            </label>
            <section className="macro-recorder" aria-label="Local recording">
              <div className="macro-recorder-head"><h3>Record</h3><span className={`macro-record-indicator${library.recordingActive ? " active" : ""}`} aria-hidden="true">{library.recordingActive ? "● Recording" : "○ Ready"}</span></div>
              <div className="macro-record-toolbar">
              <div className="macro-policy">
                <label htmlFor={policyID}>Recording delay</label>
                <GnomeSelect id={policyID} aria-label="Recording delay" value={policy} disabled={editorDisabled}
                  options={[
                    { value: "minimum", label: "Minimum · 0 local ms", direct: true },
                    { value: "fixed", label: "Default · 10 ms", direct: true },
                    { value: "recorded", label: "Recorded · between presses", direct: true },
                  ]}
                  onChange={value => setPolicy(value as DelayPolicy)} />
              </div>
              <button ref={startRef} aria-label="Start recording" type="button" className="button primary" disabled={editorDisabled} onClick={() => {
                cancelFocus();
                const token = library.beginRecording();
                if (!token) return;
                session.current = { take: startTake(policy), token, finish: library.finishRecording };
                setTakeStatus(""); setPressCount(0);
                captureRef.current?.focus();
              }}><span aria-hidden="true">● </span>Record</button>
              <button type="button" aria-label="Stop recording" className="button" disabled={!library.recordingActive} onClick={stop}><span aria-hidden="true">■ </span>Stop</button>
              </div>
              <p id="macro-capture-help" className="macro-sr-only">Press keys or click here.</p>
              <div ref={captureRef} className="macro-capture" role="region" aria-label="Local input capture" aria-describedby="macro-capture-help" tabIndex={0}
                onBlur={stop}
                onKeyDown={event => {
                  if (event.target !== event.currentTarget || (event.code === "Tab" && !captureTab) || !event.code.trim() || event.repeat) return;
                  if (capture({ type: "keyboard", key_code: event.code, action: "down" })) event.preventDefault();
                }}
                onKeyUp={event => {
                  if (event.target !== event.currentTarget || (event.code === "Tab" && !captureTab) || !event.code.trim()) return;
                  if (capture({ type: "keyboard", key_code: event.code, action: "up" })) {
                    event.preventDefault();
                    if (event.code === "Tab") { stop(); restoreCaptureFocus(); }
                  }
                }}
                onMouseDown={event => {
                  const button = mouseButtons.find(button => button.button === event.button);
                  if (event.target === event.currentTarget && button && capture({ type: button.type, action: "down" })) event.preventDefault();
                }}
                onMouseUp={event => {
                  const button = mouseButtons.find(button => button.button === event.button);
                  if (event.target === event.currentTarget && button && capture({ type: button.type, action: "up" })) event.preventDefault();
                }}
                onContextMenu={event => { if (session.current && event.target === event.currentTarget) event.preventDefault(); }}
              >{library.recordingActive ? `Recording · ${pressCount} ${pressCount === 1 ? "press" : "presses"}` : "⌨ / 🖱"}</div>
              {takeStatus && <p role="status">{takeStatus}</p>}
              <details className="macro-record-options">
                <summary>Recording options</summary>
                <label><input type="checkbox" checked={captureTab} disabled={editorDisabled} onChange={event => setCaptureTab(event.target.checked)} />Record Tab</label>
              </details>
            </section>
            <section className="macro-composer" aria-label="Complete click composer">
              <h3>Add a complete click</h3>
              <div className="macro-click-types" role="group" aria-label="Click type">
                {mouseButtons.map((button) => (
                  <button
                    key={button.type}
                    type="button"
                    className="button"
                    aria-pressed={clickType === button.type}
                    disabled={editorDisabled}
                    onClick={() => setClickType(button.type)}
                  >
                    {button.label.replace(" mouse", "")} click
                  </button>
                ))}
              </div>
              <button
                type="button"
                className="button primary"
                disabled={editorDisabled}
                onClick={() =>
                  library.appendRecordedEvents([
                    { type: clickType, action: "down", delay_ms: 0 },
                    { type: clickType, action: "up", delay_ms: 0 },
                  ])
                }
              >
                Add click
              </button>
            </section>
            <section className="macro-timeline" aria-label="Sequence">
              <div className="macro-sequence-head">
                <h3>Sequence</h3>
                <span className="hint">{draft.events.filter(event => event.action === "down").length} {draft.events.filter(event => event.action === "down").length === 1 ? "press" : "presses"}</span>
              </div>
              <ol aria-label="Ordered timeline">
                {rows.map(({ index, count }, position) => {
                  const event = draft.events[index];
                  const label = event.type === "keyboard" ? keyboardLabel(event.key_code)
                    : mouseButtons.find((button) => button.type === event.type)?.label.replace(" mouse", "") ?? event.type;
                  return (
                    <li key={index}>
                      <span className="macro-step">{String(position + 1).padStart(2, "0")}</span>
                      <div>
                        <strong>{label}{event.type === "keyboard" ? "" : count === 2 ? " click" : ` · ${event.action}`}</strong>
                        <small aria-label="Press / release delay">{`↓ ${event.action === "down" ? `${event.delay_ms} ms` : "—"} / ↑ ${count === 2 ? `${draft.events[index + 1].delay_ms} ms` : event.action === "up" ? `${event.delay_ms} ms` : "—"}`}</small>
                      </div>
                    </li>
                  );
                })}
              </ol>
              {!rows.length && <p className="macro-empty">—</p>}
              <p className={`macro-compatibility${compatible ? " compatible" : ""}`}
                title={compatible ? "Playback remains unverified." : admission.reason}>
                {compatible ? "✓ Ready to assign" : "⚠ Local only"}
              </p>
            </section>
            <div className="macro-actions macro-save-footer">
              <button
                type="button"
                className="button primary"
                disabled={editorDisabled || !draft.name.trim() || !!validationError}
                onClick={library.save}
              >
                Save to library
              </button>
              {draft.id && (
                <button
                  type="button"
                  className="button"
                  disabled={editorDisabled}
                  onClick={library.requestDelete}
                >
                  Delete macro
                </button>
              )}
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
        {validationError && <p role="alert" className="macro-error">{validationError}</p>}
        {!feedbackManaged && <OperationFeedback scope="local-library" busy={busy || loading || reading} results={[{
          id: "library", label: "Macros", version: notice, error,
          success: notice === "Saved to library. No device changes made." ? "Saved"
            : notice === "Imported as a new local macro. No device changes made." ? "Imported"
            : notice === "Macro deleted from library." ? "Deleted" : undefined,
        }]} />}
      </article>
    </div>
  );
}
