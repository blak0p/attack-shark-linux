import { useEffect, useId, useRef, useState } from "react";
import { createPortal } from "react-dom";
import type { Macro, MacroDraft, MacroPlaybackMode, MacroProgress, RemapAction as Action } from "../../desktop-contract";
import { deviceAdmission } from "../../macros/device-admission";
import { GnomeSelect, type GnomeSelectOption } from "./GnomeSelect";

type Button = { Button: number; Action: Action | null; PreservedDefault: string };
type Remap = {
  Pending: { Buttons: Button[] };
  Applied: { Buttons: Button[] };
  Actions: Action[];
  Firmware: string;
  Error: { Code: string };
  MacroPending?: MacroDraft | null;
  MacroApplied?: MacroDraft | null;
  MacroDrafts?: Record<number, MacroDraft>;
  MacroAppliedDrafts?: Record<number, MacroDraft>;
  MacroProgress?: MacroProgress;
};

const compatible = (macro: Macro) => deviceAdmission(macro.events).eligible;

const labelFor = (action: Action) =>
  ({
    off: "Off",
    left: "Left",
    right: "Right",
    middle: "Middle",
    forward: "Forward",
    backward: "Backward",
    double_click: "Double Click",
    fire: "Fire",
    media_player: "Media Player",
    play_pause: "Play/Pause",
    stop: "Stop",
    previous_track: "Previous Track",
    next_track: "Next Track",
    volume_up: "Volume Up",
    volume_down: "Volume Down",
    mute: "Mute",
    scroll_up: "Scroll Up",
    scroll_down: "Scroll Down",
    dpi_cycle: "DPI Cycle",
    dpi_plus: "DPI+",
    dpi_minus: "DPI−",
    browser_calculator: "Calculator",
    browser_email: "Email",
    browser_forward: "Forward",
    browser_backward: "Backward",
    browser_stop: "Stop",
    browser_my_computer: "My Computer",
    browser_refresh: "Refresh",
    browser_home: "Home",
    browser_search: "Search",
  })[action];

const isMultimedia = (action: Action) => ["media_player", "play_pause", "stop", "previous_track", "next_track", "volume_up", "volume_down", "mute"].includes(action);
const isMouseControls = (action: Action) => ["scroll_up", "scroll_down", "dpi_cycle", "dpi_plus", "dpi_minus"].includes(action);
const isBrowser = (action: Action) => ["browser_calculator", "browser_email", "browser_forward", "browser_backward", "browser_stop", "browser_my_computer", "browser_refresh", "browser_home", "browser_search"].includes(action);

const getButtonMacroDraft = (remap: Remap, buttonNum: number): MacroDraft | null => {
  if (remap.MacroDrafts !== undefined) {
    return remap.MacroDrafts[buttonNum] ?? null;
  }
  if (remap.MacroPending && remap.MacroPending.Button === buttonNum) {
    return remap.MacroPending;
  }
  return null;
};

const getAllStagedDrafts = (remap: Remap): MacroDraft[] => {
  if (remap.MacroDrafts && Object.keys(remap.MacroDrafts).length > 0) {
    return Object.values(remap.MacroDrafts);
  }
  if (remap.MacroPending) {
    return [remap.MacroPending];
  }
  return [];
};

const getAllAppliedDrafts = (remap: Remap): MacroDraft[] => {
  if (remap.MacroAppliedDrafts && Object.keys(remap.MacroAppliedDrafts).length > 0) {
    return Object.values(remap.MacroAppliedDrafts);
  }
  if (remap.MacroApplied) {
    return [remap.MacroApplied];
  }
  return [];
};

const normalizeDrafts = (draftsMap?: Record<number, MacroDraft>, single?: MacroDraft | null) => {
  if (draftsMap && Object.keys(draftsMap).length > 0) {
    const keys = Object.keys(draftsMap).map(Number).sort((a, b) => a - b);
    return keys.map((k) => ({ ...draftsMap[k], Mode: draftsMap[k].Mode ?? 0 }));
  }
  if (single) return [{ ...single, Mode: single.Mode ?? 0 }];
  return [];
};

const playbackLabels = [
  "Repeat a set number of times",
  "Repeat until pressed again",
  "Repeat while button is held",
] as const;

function MacroAssignmentDialog({ button, initialID, initialRepeat, initialMode, macros, available, onCancel, onConfirm }: {
  button: number;
  initialID: string;
  initialRepeat: number;
  initialMode: MacroPlaybackMode;
  macros: Macro[];
  available: boolean;
  onCancel(): void;
  onConfirm(id: string, repeat: number, mode: MacroPlaybackMode): void;
}) {
  const dialogRef = useRef<HTMLDialogElement>(null);
  const titleID = useId();
  const [id, setID] = useState(initialID);
  const [repeat, setRepeat] = useState(String(initialRepeat));
  const [mode, setMode] = useState(initialMode);
  const count = mode === 0 ? Number(repeat) : 1;
  const selected = macros.find((macro) => macro.id === id);
  const valid = available && !!selected && (mode !== 0 || repeat.trim() !== "") &&
    deviceAdmission(selected.events, count, mode).eligible;

  useEffect(() => {
    const opener = document.activeElement as HTMLElement | null;
    const dialog = dialogRef.current!;
    dialog.showModal();
    return () => {
      dialog.close();
      if (opener?.isConnected) opener.focus();
    };
  }, []);

  return (
    <dialog ref={dialogRef} className="macro-assignment-dialog card" aria-labelledby={titleID}
      onCancel={(event) => { event.preventDefault(); onCancel(); }}
      onKeyDown={(event) => {
        event.stopPropagation();
        if (event.key === "Escape") { event.preventDefault(); onCancel(); }
      }}>
      <form onSubmit={(event) => { event.preventDefault(); if (valid) onConfirm(id, count, mode); }}>
        <h2 id={titleID}>Button {button} macro assignment</h2>
        <label>
          <span>Saved macro</span>
          <GnomeSelect aria-label="Saved macro" value={id} disabled={!available || !macros.length}
            portalTarget={() => dialogRef.current}
            placeholder={id && !macros.some((macro) => macro.id === id) ? "Saved version unavailable"
              : macros.length ? "Choose a saved macro" : "No compatible saved macros"}
            options={macros.map((macro) => ({ value: macro.id, label: macro.name, direct: true }))}
            onChange={setID} />
        </label>
        <label>
          <span>Playback mode</span>
          <GnomeSelect aria-label="Playback mode" value={String(mode)} disabled={!available}
            portalTarget={() => dialogRef.current}
            options={playbackLabels.map((label, value) => ({ value: String(value), label, direct: true }))}
            onChange={(value) => setMode(Number(value) as MacroPlaybackMode)} />
        </label>
        {mode === 1 && <p className="hint">Intended behavior: press again to stop. Physical playback verification pending.</p>}
        {mode === 2 && <p className="hint">Intended behavior: repeat while the button is held. Physical playback verification pending.</p>}
        {mode === 0 && <label>
          <span>Repeat (1–255)</span>
          <input className="input" aria-label="Repeat (1–255)" type="number" min={1} max={255} step={1}
            value={repeat} disabled={!available} onChange={(event) => setRepeat(event.target.value)} />
        </label>}
        <div className="actions">
          <button className="button" type="button" onClick={onCancel}>Cancel</button>
          <button className="button primary" type="submit" disabled={!valid}>Confirm</button>
        </div>
      </form>
    </dialog>
  );
}

export function ButtonRemapPanel({
  feedbackManaged = false,
  remap,
  ready,
  onStage,
  onApply = () => {},
  onDiscard = () => {},
  feedbackFor,
  macros = [],
  libraryReady = true,
  onStageMacro,
  onClearMacro,
  error = "",
  assignmentIntent: _assignmentIntent,
  assignmentScope,
  assignmentAvailable = ready,
  onConsumeIntent: _onConsumeIntent,
}: {
  feedbackManaged?: boolean;
  remap: Remap;
  ready: boolean;
  macros?: Macro[];
  libraryReady?: boolean;
  onStageMacro?(id: string, button: number, repeat: number, mode?: MacroPlaybackMode): void;
  onClearMacro?(button: number): void;
  error?: string;
  onStage(button: number, action: Action): void;
  onApply?(): void;
  onDiscard?(): void;
  feedbackFor?: (code: string) => string;
  assignmentIntent?: { token: number; id: string };
  assignmentScope?: unknown;
  assignmentAvailable?: boolean;
  onConsumeIntent?(token: number): void;
}) {
  const isAvailable = ready && assignmentAvailable;
  const [macroDialog, setMacroDialog] = useState<{ button: number; id: string; repeat: number; mode: MacroPlaybackMode; scope: unknown } | null>(null);

  useEffect(() => {
    setMacroDialog(null);
  }, [assignmentScope]);

  const assignmentLabel = (button: Button) => {
    const draft = getButtonMacroDraft(remap, button.Button);
    if (draft) {
      return draft.Mode === 1 || draft.Mode === 2
        ? `${draft.Name} — ${playbackLabels[draft.Mode]}` : `${draft.Name} × ${draft.Repeat}`;
    }
    return button.Action ? labelFor(button.Action) : button.PreservedDefault || "Default";
  };

  const compatibleMacros = libraryReady && isAvailable ? macros.filter(compatible) : [];

  const optionsForButton = (button: Button) => {
    const actionOptions: GnomeSelectOption[] = remap.Actions.map((action) => ({
      value: action,
      label: labelFor(action),
      group: isBrowser(action) ? "Browser" : isMouseControls(action) ? "Mouse Controls" : isMultimedia(action) ? "Multimedia" : "Basic",
      disabled: button.Button === 1 && (isMultimedia(action) || isMouseControls(action)),
    }));

    return [...actionOptions, {
      value: "macro", label: "Macro", direct: true,
      disabled: !libraryReady || !onStageMacro,
    }];
  };

  const stagedDrafts = getAllStagedDrafts(remap);
  const appliedDrafts = getAllAppliedDrafts(remap);
  const macroTransport = stagedDrafts.length > 0 || appliedDrafts.length > 0;
  const draftMatchesApplied = JSON.stringify(normalizeDrafts(remap.MacroDrafts, remap.MacroPending)) ===
    JSON.stringify(normalizeDrafts(remap.MacroAppliedDrafts, remap.MacroApplied)) &&
    JSON.stringify(remap.Pending) === JSON.stringify(remap.Applied);

  const residualMacroProgress = !macroTransport && !!(remap.MacroProgress?.Assignment || remap.MacroProgress?.Upload);

  return (
    <article
      id="remapping-card"
      className="card"
      aria-labelledby="button-remap-title"
      tabIndex={0}
      onKeyDown={(event) => {
        if (!isAvailable || event.target !== event.currentTarget) return;
        if (event.key === "Enter") {
          event.preventDefault();
          onApply();
        }
        if (event.key === "Escape") {
          event.preventDefault();
          onDiscard();
        }
      }}
    >
      <h2 id="button-remap-title">Button remapping</h2>

      {!libraryReady && <p className="hint">Library unavailable. Retry in Macros.</p>}
      {libraryReady && macros.some(macro => !compatible(macro)) && <p className="hint">Some macros cannot be sent. See Macros for details.</p>}
      {!feedbackManaged && error && <p role="alert">{error}</p>}

      {remap.Pending.Buttons.map((button) => {
        const draft = getButtonMacroDraft(remap, button.Button);
        const assignedLabel = assignmentLabel(button);
        const selectValue = draft ? "" : (button.Action ?? "");

        return (
          <div className="binding remap-binding" key={button.Button}>
            <div>
              <b>
                Button {button.Button}
                {button.PreservedDefault ? ` (${button.PreservedDefault})` : ""}
              </b>
              <span>
                {assignedLabel}
              </span>
            </div>
            <div className="remap-button-controls">
              <GnomeSelect
                aria-label={`Button ${button.Button} action`}
                disabled={!isAvailable}
                value={selectValue}
                placeholder={assignedLabel}
                options={optionsForButton(button)}
                onChange={(val) => {
                    if (val === "macro") {
                      if (isAvailable && libraryReady && onStageMacro) {
                        setMacroDialog({ button: button.Button, id: draft?.ID ?? "", repeat: draft?.Repeat ?? 1, mode: draft?.Mode ?? 0, scope: assignmentScope });
                      }
                      return;
                    }
                    const action = val as Action;
                    if (!(button.Button === 1 && (isMultimedia(action) || isMouseControls(action)))) {
                      onStage(button.Button, action);
                      if (draft) {
                        onClearMacro?.(button.Button);
                      }
                    }
                }}
              />
            </div>
          </div>
        );
      })}

      {macroDialog && macroDialog.scope === assignmentScope && createPortal(
        <MacroAssignmentDialog
          button={macroDialog.button}
          initialID={macroDialog.id}
          initialRepeat={macroDialog.repeat}
          initialMode={macroDialog.mode}
          macros={compatibleMacros}
          available={isAvailable && libraryReady && !!onStageMacro}
          onCancel={() => setMacroDialog(null)}
          onConfirm={(id, repeat, mode) => {
            if (!isAvailable || !libraryReady || !compatibleMacros.some((macro) => macro.id === id)) return;
            if (mode === 0) onStageMacro?.(id, macroDialog.button, repeat);
            else onStageMacro?.(id, macroDialog.button, repeat, mode);
            setMacroDialog(null);
          }}
        />,
        document.body,
      )}

      <details className="group"><summary>Assignment summary</summary>
      <p aria-label="Remap assignment summary" className="hint" style={{ marginTop: 14 }}>
        {remap.Pending.Buttons.map(
          (button) =>
            `Button ${button.Button}: ${
              assignmentLabel(button)
            }`
        ).join(", ")}
      </p>
      </details>

      <div className="actions">
        <button type="button" className="button" disabled={!isAvailable} onClick={onDiscard}>
          Discard remap
        </button>
        <button type="button" className="button primary" disabled={!isAvailable} onClick={onApply}>
          Apply remap
        </button>
      </div>

      {residualMacroProgress && <p className="hint" title="Previous macro transport progress retained; playback and device persistence unverified.">Device state may be partial or unknown.</p>}
      {(!feedbackManaged || macroTransport) && <details className="group" open={!feedbackManaged}><summary>Device details</summary>
      <div className="status" role="status" aria-label="Button remapping status">
        {macroTransport
          ? remap.Firmware === "success"
            ? draftMatchesApplied
              ? "Remap and macro transport confirmed. Playback and device persistence unverified."
              : appliedDrafts.length > 0
                ? "Last remap and macro transport confirmed; current draft not confirmed. Playback and device persistence unverified."
                : "Macro assignment staged locally; current draft not confirmed. Playback and device persistence unverified."
            : remap.Firmware === "failed"
            ? "Macro transport not confirmed; device state may be partial or unknown. Playback and device persistence unverified."
            : "Macro assignment staged locally; no transport confirmation. Playback and device persistence unverified."
          : remap.Firmware === "success"
          ? "Button remapping applied"
          : remap.Firmware === "failed"
          ? <>
              Button remapping failed
              {remap.Error.Code && feedbackFor && <>: <span role="alert">{feedbackFor(remap.Error.Code)}</span></>}
            </>
          : "Remap draft pending confirmation"}
      </div>
      </details>}
    </article>
  );
}
