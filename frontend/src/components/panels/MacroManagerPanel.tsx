import { useRef, useState } from "react";
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
  ...rest
}: {
  service: MacroLibraryService;
  library?: Library;
  onAssign?(id: string): void;
  assignmentReady?: boolean;
}) {
  return library ? <MacroManagerEditor library={library} service={service} /> : <LocalMacroManager service={service} />;
}

function LocalMacroManager({ service }: { service: MacroLibraryService }) {
  const library = useMacroLibrary(service);
  return <MacroManagerEditor library={library} service={service} />;
}

function MacroManagerEditor({ library, service }: { library: Library; service: MacroLibraryService }) {
  const { macros, draft, loading, loaded, reading, busy, error, confirmation, notice, validationError } = library;
  const unavailable = busy || reading || loading || confirmation;
  const [clickType, setClickType] = useState<Macro["events"][number]["type"]>("mouse_left");
  const fileInputRef = useRef<HTMLInputElement>(null);
  const rows = timeline(draft?.events ?? []);
  const compatible = rows.length >= 1 && rows.length <= 2 && rows.every((row) => row.count === 2);
  const editorDisabled = unavailable;

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
          <button type="button" className="button primary" disabled={busy || loading} onClick={() => library.newMacro()}>New macro</button>
        </div>
        <p className="hint">Shared across devices. Available offline.</p>
        {loading && <p role="status">Loading library…</p>}
        {!loading && loaded && macros.length === 0 && <p className="hint">No macros in your library yet.</p>}
        <div className="macro-cards">
          {macros.map((macro) => (
            <button
              type="button"
              key={macro.id}
              className="macro-card"
              disabled={busy || loading}
              aria-pressed={draft?.id === macro.id}
              aria-label={`${macro.name} · ${macro.events.length} events`}
              onClick={() => library.select(macro.id)}
            >
              <strong>{macro.name}</strong>
              <span>{macro.events.length} events · Local library</span>
            </button>
          ))}
        </div>
        {error && !loading && <button type="button" className="button" disabled={busy} onClick={library.reload}>Retry library</button>}
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
              <p className="hint">One version-1 JSON macro per file, up to 1 MiB (local file safety budget, not a device limit). Import creates a new copy, even with a duplicate name; existing macros are never replaced.</p>
              <button type="button" className="button" disabled={editorDisabled || !library.savedMacro} onClick={download}>Export saved macro</button>
              <p className="hint">Exports saved name and events without the local ID. Unsaved editor changes are not exported.</p>
            </div>
          </details>
        </header>

        {reading && <p role="status">Loading macro…</p>}
        {!draft ? (
          <p className="macro-empty">Select a macro or create a new one.</p>
        ) : (
          <>
            <p className="hint macro-guidance">
              To assign this macro to a mouse button, open Button remapping.
            </p>
            <label className="macro-name">Macro name
              <input
                className="input"
                value={draft.name}
                disabled={editorDisabled}
                onChange={(event) => library.rename(event.target.value)}
              />
            </label>
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
                <span className="hint">{rows.length} actions · {draft.events.length} events</span>
              </div>
              <ol aria-label="Ordered timeline">
                {rows.map(({ index, count }, position) => {
                  const event = draft.events[index];
                  const label = mouseButtons.find((button) => button.type === event.type)?.label;
                  return (
                    <li key={index}>
                      <span className="macro-step">{String(position + 1).padStart(2, "0")}</span>
                      <div>
                        <strong>{label?.replace(" mouse", "")}{count === 2 ? " click" : ` · ${event.action}`}</strong>
                        <small>{count === 2 ? "Complete click · zero local delay" : `Raw event ${index + 1} · ${event.delay_ms} ms`}</small>
                      </div>
                    </li>
                  );
                })}
              </ol>
              {!rows.length && <p className="macro-empty">No actions yet. Add a click above. Empty macros can be saved.</p>}
              <p className={`macro-compatibility${compatible ? " compatible" : ""}`}>
                {compatible
                  ? "Eligible for assignment after saving. Playback remains unverified."
                  : "Local sequence only · this layout cannot be assigned to the device."}
              </p>
            </section>
            <div className="macro-actions macro-save-footer">
              <p className="hint">Local library · no device changes</p>
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
        {busy && <p role="status">Saving local library…</p>}
        {(validationError || error) && <p role="alert" className="macro-error">{validationError || error}</p>}
        {notice && <p role="status">{notice}</p>}
      </article>
    </div>
  );
}
