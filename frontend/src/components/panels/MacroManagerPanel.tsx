import type { MacroLibraryService } from "../../desktop-contract";
import { useMacroLibrary } from "../../hooks/useMacroLibrary";
import "./MacroManagerPanel.css";

export function MacroManagerPanel({ service }: { service: MacroLibraryService }) {
  const library = useMacroLibrary(service);
  const { macros, draft, loading, loaded, reading, busy, error, confirmation, notice, validationError } = library;
  const editorDisabled = busy || reading || loading || confirmation;
  return (
    <div className="macro-manager" aria-busy={busy}>
      <article className="card macro-library" aria-label="Macro library">
        <div className="card-head">
          <h2>Macro library</h2>
          <button type="button" className="button primary" disabled={busy || loading} onClick={library.newMacro}>New macro</button>
        </div>
        <p className="hint">Shared across devices. Available offline.</p>
        {loading && <p role="status">Loading library…</p>}
        {!loading && loaded && macros.length === 0 && <p className="hint">No macros in your library yet.</p>}
        <div className="macro-cards">
          {macros.map((macro) => (
            <button type="button" key={macro.id} className="macro-card" disabled={busy || loading}
              aria-pressed={draft?.id === macro.id} aria-label={`${macro.name} · ${macro.events.length} events`}
              onClick={() => library.select(macro.id)}>
              <strong>{macro.name}</strong><span>{macro.events.length} events · Local library</span>
            </button>
          ))}
        </div>
        {error && !loading && <button type="button" className="button" disabled={busy} onClick={library.reload}>Retry library</button>}
      </article>
      <article className="card macro-detail" aria-label="Macro details">
        <h2>{draft?.id ? "Macro details" : "New macro"}</h2>
        <p className="hint">Save to library only. Device assignment comes later.</p>
        {reading && <p role="status">Loading macro…</p>}
        {!draft ? <p className="macro-empty">Select a macro or create a new one.</p> : (
          <>
            <label className="macro-name">Macro name
              <input className="input" value={draft.name} disabled={editorDisabled}
                onChange={(event) => library.rename(event.target.value)} />
            </label>
            <section className="group macro-events" aria-label="Ordered events">
              <h3>Events · {draft.events.length}</h3>
              <p className="hint">Ordered left/right press and release events. Delays are local milliseconds, not verified device timing.</p>
              <button type="button" className="button" disabled={editorDisabled} onClick={library.addEvent}>Add event</button>
              {draft.events.length === 0 ? <p className="macro-empty">No events yet. Empty macros can be saved.</p> : (
                <ol>{draft.events.map((event, index) => (
                  <li key={index}>
                    <span>{event.type === "mouse_left" ? "Left mouse" : "Right mouse"} · {event.action} · {event.delay_ms} ms</span>
                    <div className="macro-event-fields">
                      <label>Event {index + 1} button
                        <select disabled={editorDisabled} value={event.type}
                          onChange={(change) => library.updateEvent(index, { type: change.target.value as typeof event.type })}>
                          <option value="mouse_left">Left mouse</option><option value="mouse_right">Right mouse</option>
                        </select>
                      </label>
                      <label>Event {index + 1} action
                        <select disabled={editorDisabled} value={event.action}
                          onChange={(change) => library.updateEvent(index, { action: change.target.value as typeof event.action })}>
                          <option value="down">Press (down)</option><option value="up">Release (up)</option>
                        </select>
                      </label>
                      <label>Event {index + 1} delay (ms)
                        <input inputMode="numeric" disabled={editorDisabled} value={event.delay_ms}
                          aria-invalid={validationError.startsWith(`Event ${index + 1} delay`)}
                          onChange={(change) => library.updateEvent(index, { delay_ms: change.target.value })} />
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
            <div className="macro-actions">
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
