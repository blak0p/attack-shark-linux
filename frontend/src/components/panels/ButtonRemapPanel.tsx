import { GnomeSelect } from "./GnomeSelect";

type Action = "off" | "left" | "right" | "middle" | "forward" | "backward" | "double_click" | "fire" | "media_player" | "play_pause" | "stop" | "previous_track" | "next_track" | "volume_up" | "volume_down" | "mute" | "scroll_up" | "scroll_down" | "dpi_cycle" | "dpi_plus" | "dpi_minus";
type Button = { Button: number; Action: Action | null; PreservedDefault: string };
type Remap = {
  Pending: { Buttons: Button[] };
  Applied: { Buttons: Button[] };
  Actions: Action[];
  Firmware: string;
  Error: { Code: string };
};

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
  })[action];

const isMultimedia = (action: Action) => ["media_player", "play_pause", "stop", "previous_track", "next_track", "volume_up", "volume_down", "mute"].includes(action);
const isMouseControls = (action: Action) => ["scroll_up", "scroll_down", "dpi_cycle", "dpi_plus", "dpi_minus"].includes(action);

export function ButtonRemapPanel({
  remap,
  ready,
  onStage,
  onApply = () => {},
  onDiscard = () => {},
  feedbackFor,
}: {
  remap: Remap;
  ready: boolean;
  onStage(button: number, action: Action): void;
  onApply?(): void;
  onDiscard?(): void;
  feedbackFor?: (code: string) => string;
}) {
  return (
    <article
      id="remapping-card"
      className="card"
      aria-labelledby="button-remap-title"
      tabIndex={0}
      onKeyDown={(event) => {
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

      {remap.Pending.Buttons.map((button) => (
        <div className="binding" key={button.Button}>
          <div>
            <b>
              Button {button.Button}
              {button.PreservedDefault ? ` (${button.PreservedDefault})` : ""}
            </b>
            <span>
              {button.Action ? labelFor(button.Action) : button.PreservedDefault || "Default"}
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
              group: isMouseControls(action) ? "Mouse Controls" : isMultimedia(action) ? "Multimedia" : "Basic",
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
              button.Action ? labelFor(button.Action) : button.PreservedDefault
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

      <div className="status" role="status" aria-label="Button remapping status">
        {remap.Firmware === "success"
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
