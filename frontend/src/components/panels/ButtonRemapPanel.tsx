import { useEffect, useId, useRef, useState } from "react";
import { createPortal } from "react-dom";
import type { Macro, MacroDraft, MacroProgress, RemapAction as Action } from "../../desktop-contract";
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

const compatible = (macro: Macro) => [2, 4].includes(macro.events.length) &&
  macro.events.every((event, index, events) =>
    ["mouse_left", "mouse_right", "mouse_middle", "mouse_back", "mouse_forward"].includes(event.type) &&
    event.delay_ms === 0 && (index % 2 === 0
      ? event.action === "down" && event.type === events[index + 1].type
      : event.action === "up"));

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
    return keys.map((k) => draftsMap[k]);
  }
  if (single) return [single];
  return [];
};

function MacroAssignmentDialog({ button, initialID, initialRepeat, macros, available, onCancel, onConfirm }: {
  button: number;
  initialID: string;
  initialRepeat: number;
  macros: Macro[];
  available: boolean;
  onCancel(): void;
  onConfirm(id: string, repeat: number): void;
}) {
  const dialogRef = useRef<HTMLDialogElement>(null);
  const titleID = useId();
  const [id, setID] = useState(initialID);
  const [repeat, setRepeat] = useState(String(initialRepeat));
  const count = Number(repeat);
  const valid = available && macros.some((macro) => macro.id === id) &&
    repeat.trim() !== "" && Number.isInteger(count) && count >= 1 && count <= 255;

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
      <form onSubmit={(event) => { event.preventDefault(); if (valid) onConfirm(id, count); }}>
        <h2 id={titleID}>Button {button} macro assignment</h2>
        <label>
          <span>Saved macro</span>
          <select className="select" aria-label="Saved macro" value={id} disabled={!available || !macros.length}
            onChange={(event) => setID(event.target.value)}>
            <option value="">{macros.length ? "Choose a saved macro" : "No compatible saved macros"}</option>
            {id && !macros.some((macro) => macro.id === id) && <option value={id} disabled>Saved version unavailable</option>}
            {macros.map((macro) => <option key={macro.id} value={macro.id}>{macro.name}</option>)}
          </select>
        </label>
        <label>
          <span>Repeat (1–255)</span>
          <input className="input" aria-label="Repeat (1–255)" type="number" min={1} max={255} step={1}
            value={repeat} disabled={!available} onChange={(event) => setRepeat(event.target.value)} />
        </label>
        <div className="actions">
          <button className="button" type="button" onClick={onCancel}>Cancel</button>
          <button className="button primary" type="submit" disabled={!valid}>Confirm</button>
        </div>
      </form>
    </dialog>
  );
}

export function ButtonRemapPanel({
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
  remap: Remap;
  ready: boolean;
  macros?: Macro[];
  libraryReady?: boolean;
  onStageMacro?(id: string, button: number, repeat: number): void;
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
  const [macroDialog, setMacroDialog] = useState<{ button: number; id: string; repeat: number; scope: unknown } | null>(null);

  useEffect(() => {
    setMacroDialog(null);
  }, [assignmentScope]);

  const assignmentLabel = (button: Button) => {
    const draft = getButtonMacroDraft(remap, button.Button);
    if (draft) {
      return `${draft.Name} × ${draft.Repeat}`;
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
      <p className="hint">Review the complete assignment below, then apply or discard it.</p>

      {!libraryReady && <p className="hint">Saved library unavailable or loading. Open Macros to retry.</p>}
      {error && <p role="alert">{error}</p>}

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
                        setMacroDialog({ button: button.Button, id: draft?.ID ?? "", repeat: draft?.Repeat ?? 1, scope: assignmentScope });
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
          macros={compatibleMacros}
          available={isAvailable && libraryReady && !!onStageMacro}
          onCancel={() => setMacroDialog(null)}
          onConfirm={(id, repeat) => {
            if (!isAvailable || !libraryReady || !compatibleMacros.some((macro) => macro.id === id)) return;
            onStageMacro?.(id, macroDialog.button, repeat);
            setMacroDialog(null);
          }}
        />,
        document.body,
      )}

      <p aria-label="Remap assignment summary" className="hint" style={{ marginTop: 14 }}>
        {remap.Pending.Buttons.map(
          (button) =>
            `Button ${button.Button}: ${
              assignmentLabel(button)
            }`
        ).join(", ")}
      </p>

      <div className="actions">
        <button type="button" className="button" disabled={!isAvailable} onClick={onDiscard}>
          Discard remap
        </button>
        <button type="button" className="button primary" disabled={!isAvailable} onClick={onApply}>
          Apply remap
        </button>
      </div>

      {residualMacroProgress && <p className="hint">Prior macro transport progress retained; macro device state may be partial or unknown. Playback and device persistence unverified.</p>}
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
    </article>
  );
}
