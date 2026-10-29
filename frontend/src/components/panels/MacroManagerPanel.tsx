import { useEffect, useId, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { i18n } from "../../i18n";
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
  if (!code) return i18n.t("macro.keyboardMissing");
  return i18n.t("macro.key", { key: code.replace(/^(Key|Digit)(?=[A-Z0-9]$)/, "") });
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

const mouseButtons: { button: number; type: Macro["events"][number]["type"]; nameKey: string }[] = [
  { button: 0, type: "mouse_left", nameKey: "macro.mouse.left" },
  { button: 2, type: "mouse_right", nameKey: "macro.mouse.right" },
  { button: 1, type: "mouse_middle", nameKey: "macro.mouse.middle" },
  { button: 3, type: "mouse_back", nameKey: "macro.mouse.back" },
  { button: 4, type: "mouse_forward", nameKey: "macro.mouse.forward" },
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
  const { t } = useTranslation();
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
    setTakeStatus(appended ? t("macro.presses", { count: presses }) : t("macro.recordingEnded"));
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
      if (file.size > MAX_MACRO_FILE_BYTES) throw new Error(t("macro.fileTooLarge"));
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
      <article className="card macro-library" aria-label={t("macro.library")}>
        <div className="card-head">
          <h2>{t("macro.library")}</h2>
          <button type="button" className="button primary" disabled={navigationDisabled} onClick={() => { cancelFocus(); library.newMacro(); }}>{t("macro.new")}</button>
        </div>

        {!loading && loaded && macros.length === 0 && <p className="hint">{t("macro.empty")}</p>}
        <div className="macro-cards">
          {macros.map((macro) => (
            <button
              type="button"
              key={macro.id}
              className="macro-card"
              disabled={navigationDisabled}
              aria-pressed={draft?.id === macro.id}
              aria-label={`${macro.name} · ${t("macro.events", { n: macro.events.length })}`}
              onClick={() => { cancelFocus(); library.select(macro.id); }}
            >
              <strong>{macro.name}</strong>
              <span>{t("macro.events", { n: macro.events.length })} · {t("macro.localLibrary")}</span>
            </button>
          ))}
        </div>
        {error && !loading && <button type="button" className="button" disabled={editorDisabled} onClick={library.reload}>{t("macro.retry")}</button>}
      </article>

      <article className="card macro-detail" aria-label={t("macro.details")}>
        <header className="macro-editor-head">
          <div>
            <p className="macro-kicker">{t("macro.editor")}</p>
            <h2>{draft?.name.trim() || t("macro.untitled")}</h2>
          </div>
          <details className="macro-options">
            <summary>{t("macro.options")}</summary>
            <div className="macro-options-body">
              <div className="macro-file-import">
                <input
                  ref={fileInputRef}
                  type="file"
                  accept=".json,application/json"
                  style={{ display: "none" }}
                  aria-label={t("macro.importJson")}
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
                  {t("macro.importButton")}
                </button>
              </div>
              <button type="button" className="button" disabled={editorDisabled || !library.savedMacro} onClick={download}>{t("macro.export")}</button>
            </div>
          </details>
        </header>


        {!draft ? (
          <p className="macro-empty">{t("macro.select")}</p>
        ) : (
          <>
            <label className="macro-name">{t("macro.name")}
              <input
                className="input"
                value={draft.name}
                disabled={editorDisabled}
                onChange={(event) => library.rename(event.target.value)}
              />
            </label>
            <section className="macro-recorder" aria-label={t("macro.recording")}>
              <div className="macro-recorder-head"><h3>{t("macro.record")}</h3><span className={`macro-record-indicator${library.recordingActive ? " active" : ""}`} aria-hidden="true">{library.recordingActive ? t("macro.indicatorRecording") : t("macro.indicatorReady")}</span></div>
              <div className="macro-record-toolbar">
              <div className="macro-policy">
                <label htmlFor={policyID}>{t("macro.delay")}</label>
                <GnomeSelect id={policyID} aria-label={t("macro.delay")} value={policy} disabled={editorDisabled}
                  options={[
                    { value: "minimum", label: t("macro.delayMin"), direct: true },
                    { value: "fixed", label: t("macro.delayDefault"), direct: true },
                    { value: "recorded", label: t("macro.delayRecorded"), direct: true },
                  ]}
                  onChange={value => setPolicy(value as DelayPolicy)} />
              </div>
              <button ref={startRef} aria-label={t("macro.start")} type="button" className="button primary" disabled={editorDisabled} onClick={() => {
                cancelFocus();
                const token = library.beginRecording();
                if (!token) return;
                session.current = { take: startTake(policy), token, finish: library.finishRecording };
                setTakeStatus(""); setPressCount(0);
                captureRef.current?.focus();
              }}><span aria-hidden="true">● </span>{t("macro.record")}</button>
              <button type="button" aria-label={t("macro.stop")} className="button" disabled={!library.recordingActive} onClick={stop}><span aria-hidden="true">■ </span>{t("macro.stopLabel")}</button>
              </div>
              <p id="macro-capture-help" className="macro-sr-only">{t("macro.captureHelp")}</p>
              <div ref={captureRef} className="macro-capture" role="region" aria-label={t("macro.capture")} aria-describedby="macro-capture-help" tabIndex={0}
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
              >{library.recordingActive ? t("macro.recordingCount", { count: pressCount }) : "⌨ / 🖱"}</div>
              {takeStatus && <p role="status">{takeStatus}</p>}
              <details className="macro-record-options">
                <summary>{t("macro.recordingOptions")}</summary>
                <label><input type="checkbox" checked={captureTab} disabled={editorDisabled} onChange={event => setCaptureTab(event.target.checked)} />{t("macro.recordTab")}</label>
              </details>
            </section>
            <section className="macro-composer" aria-label={t("macro.composer")}>
              <h3>{t("macro.addClickTitle")}</h3>
              <div className="macro-click-types" role="group" aria-label={t("macro.clickType")}>
                {mouseButtons.map((button) => (
                  <button
                    key={button.type}
                    type="button"
                    className="button"
                    aria-pressed={clickType === button.type}
                    disabled={editorDisabled}
                    onClick={() => setClickType(button.type)}
                  >
                    {t("macro.click", { button: t(button.nameKey) })}
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
                {t("macro.addClick")}
              </button>
            </section>
            <section className="macro-timeline" aria-label={t("macro.sequence")}>
              <div className="macro-sequence-head">
                <h3>{t("macro.sequence")}</h3>
                <span className="hint">{t("macro.presses", { count: draft.events.filter(event => event.action === "down").length })}</span>
              </div>
              <ol aria-label={t("macro.timeline")}>
                {rows.map(({ index, count }, position) => {
                  const event = draft.events[index];
                  const label = event.type === "keyboard" ? keyboardLabel(event.key_code)
                    : (() => { const found = mouseButtons.find((button) => button.type === event.type); return found ? t(found.nameKey) : event.type; })();
                  return (
                    <li key={index}>
                      <span className="macro-step">{String(position + 1).padStart(2, "0")}</span>
                      <div>
                        <strong>{event.type === "keyboard" ? label : count === 2 ? t("macro.clickLabel", { label }) : `${label} · ${t(`macro.action.${event.action}`)}`}</strong>
                        <small aria-label={t("macro.pressReleaseDelay")}>{`↓ ${event.action === "down" ? `${event.delay_ms} ms` : "—"} / ↑ ${count === 2 ? `${draft.events[index + 1].delay_ms} ms` : event.action === "up" ? `${event.delay_ms} ms` : "—"}`}</small>
                      </div>
                    </li>
                  );
                })}
              </ol>
              {!rows.length && <p className="macro-empty">—</p>}
              <p className={`macro-compatibility${compatible ? " compatible" : ""}`}
                title={compatible ? t("macro.unverified") : admission.reason}>
                {compatible ? t("macro.ready") : t("macro.localOnly")}
              </p>
            </section>
            <div className="macro-actions macro-save-footer">
              <button
                type="button"
                className="button primary"
                disabled={editorDisabled || !draft.name.trim() || !!validationError}
                onClick={library.save}
              >
                {t("macro.save")}
              </button>
              {draft.id && (
                <button
                  type="button"
                  className="button"
                  disabled={editorDisabled}
                  onClick={library.requestDelete}
                >
                  {t("macro.delete")}
                </button>
              )}
            </div>
            {confirmation && (
              <div role="dialog" aria-modal="false" aria-label={t("macro.deleteConfirmLabel")} className="group">
                <p>{t("macro.deleteConfirm", { name: draft.name })}</p>
                <div className="macro-actions">
                  <button type="button" className="button" disabled={busy} onClick={library.cancelDelete}>{t("common.cancel")}</button>
                  <button type="button" className="button danger" disabled={busy} onClick={library.deleteMacro}>{t("macro.confirmDelete")}</button>
                </div>
              </div>
            )}
          </>
        )}
        {validationError && <p role="alert" className="macro-error">{validationError}</p>}
        {!feedbackManaged && <OperationFeedback scope="local-library" busy={busy || loading || reading} results={[{
          id: "library", label: t("feedback.labels.macros"), version: notice, error,
          success: notice === "Saved to library. No device changes made." ? t("feedback.saved")
            : notice === "Imported as a new local macro. No device changes made." ? t("feedback.imported")
            : notice === "Macro deleted from library." ? t("feedback.deleted") : undefined,
        }]} />}
      </article>
    </div>
  );
}
