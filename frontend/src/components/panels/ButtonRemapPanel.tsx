import { useState } from "react";
import type { Macro, MacroDraft, MacroProgress } from "../../desktop-contract";
import { GnomeSelect } from "./GnomeSelect";

import type { RemapAction as Action } from "../../desktop-contract";
type Button = { Button: number; Action: Action | null; PreservedDefault: string };
type Remap = {
  Pending: { Buttons: Button[] };
  Applied: { Buttons: Button[] };
  Actions: Action[];
  Firmware: string;
  Error: { Code: string };
  MacroPending?: MacroDraft | null;
  MacroApplied?: MacroDraft | null;
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
  error = "",
}: {
  remap: Remap;
  ready: boolean;
  macros?: Macro[];
  libraryReady?: boolean;
  onStageMacro?(id: string, button: number, repeat: number): void;
  error?: string;
  onStage(button: number, action: Action): void;
  onApply?(): void;
  onDiscard?(): void;
  feedbackFor?: (code: string) => string;
}) {
  const [target, setTarget] = useState(1);
  const [macroID, setMacroID] = useState("");
  const [repeat, setRepeat] = useState("1");
  const selectedMacro = macros.find((macro) => macro.id === macroID);
  const validRepeat = repeat.trim() !== "" && Number.isInteger(Number(repeat)) && Number(repeat) >= 1 && Number(repeat) <= 255;
  const canStage = ready && libraryReady && !!onStageMacro && !!selectedMacro && compatible(selectedMacro) && validRepeat;
  const assignmentLabel = (button: Button) => remap.MacroPending?.Button === button.Button
    ? `${remap.MacroPending.Name} × ${remap.MacroPending.Repeat}`
    : button.Action ? labelFor(button.Action) : button.PreservedDefault || "Default";
  const draftMatchesApplied = JSON.stringify(remap.MacroPending ?? null) === JSON.stringify(remap.MacroApplied ?? null) &&
    JSON.stringify(remap.Pending) === JSON.stringify(remap.Applied);
  const macroTransport = !!(remap.MacroPending || remap.MacroApplied);
  // Native ordinary apply retains historical macro progress, not macro confirmation.
  const residualMacroProgress = !macroTransport && !!(remap.MacroProgress?.Assignment || remap.MacroProgress?.Upload);
  return (
    <article
      id="remapping-card"
      className="card"
      aria-labelledby="button-remap-title"
      tabIndex={0}
      onKeyDown={(event) => {
        if (!ready || event.target !== event.currentTarget) return;
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

      <fieldset disabled={!ready || !libraryReady} onKeyDown={(event) => event.stopPropagation()}>
        <legend>Saved macro assignment</legend>
        <label>Macro target button
          <select aria-label="Macro target button" className="select" value={target} onChange={(event) => setTarget(Number(event.target.value))}>
            {Array.from({ length: 7 }, (_, index) => <option key={index + 1} value={index + 1}>Button {index + 1}</option>)}
          </select>
        </label>
        <label>Saved macro
          <select aria-label="Saved macro" className="select" value={selectedMacro ? macroID : ""} onChange={(event) => setMacroID(event.target.value)}>
            <option value="">Choose a saved macro</option>
            {macros.map((macro) => <option key={macro.id} value={macro.id}>{macro.name}{compatible(macro) ? "" : " (incompatible)"}</option>)}
          </select>
        </label>
        <label>Fixed repetitions
          <input className="input" type="number" min={1} max={255} step={1} value={repeat} onChange={(event) => setRepeat(event.target.value)} />
        </label>
        <button type="button" className="button" disabled={!canStage} onClick={() => { if (canStage) onStageMacro!(macroID, target, Number(repeat)); }}>Stage macro assignment</button>
      </fieldset>
      <p className="hint">Save locally in Macros, then stage here and use Apply remap. Backend admission: one or two complete ordered down/up clicks using left/right/middle/back/forward, with zero delay on every event and fixed repetitions 1–255. The target button is independent of the macro actions. One macro overlay per device; staging another replaces it. Playback and device persistence remain unverified.</p>
      {!libraryReady && <p className="hint">Saved library unavailable or loading. Open Macros to retry.</p>}
      {error && <p role="alert">{error}</p>}

      {remap.Pending.Buttons.map((button) => (
        <div className="binding" key={button.Button}>
          <div>
            <b>
              Button {button.Button}
              {button.PreservedDefault ? ` (${button.PreservedDefault})` : ""}
            </b>
            <span>
              {assignmentLabel(button)}
            </span>
          </div>
          <GnomeSelect
            aria-label={`Button ${button.Button} action`}
            disabled={!ready}
            value={button.Action ?? ""}
            placeholder={button.PreservedDefault || "Default"}
            options={remap.Actions.map((action) => ({
              value: action,
              label: labelFor(action),
              group: isBrowser(action) ? "Browser" : isMouseControls(action) ? "Mouse Controls" : isMultimedia(action) ? "Multimedia" : "Basic",
              disabled: button.Button === 1 && (isMultimedia(action) || isMouseControls(action)),
            }))}
            onChange={(val) => {
              const action = val as Action;
              if (!(button.Button === 1 && (isMultimedia(action) || isMouseControls(action)))) onStage(button.Button, action);
            }}
          />
        </div>
      ))}

      <p aria-label="Remap assignment summary" className="hint" style={{ marginTop: 14 }}>
        {remap.Pending.Buttons.map(
          (button) =>
            `Button ${button.Button}: ${
              assignmentLabel(button)
            }`
        ).join(", ")}
      </p>

      <div className="actions">
        <button type="button" className="button" disabled={!ready} onClick={onDiscard}>
          Discard remap
        </button>
        <button type="button" className="button primary" disabled={!ready} onClick={onApply}>
          Apply remap
        </button>
      </div>

      {residualMacroProgress && <p className="hint">Prior macro transport progress retained; macro device state may be partial or unknown. Playback and device persistence unverified.</p>}
      <div className="status" role="status" aria-label="Button remapping status">
        {macroTransport
          ? remap.Firmware === "success"
            ? draftMatchesApplied
              ? "Remap and macro transport confirmed. Playback and device persistence unverified."
              : remap.MacroApplied
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
