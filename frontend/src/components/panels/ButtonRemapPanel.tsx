import { GnomeSelect } from "./GnomeSelect";

type Action = "off" | "left" | "right" | "middle" | "forward" | "backward" | "double_click" | "fire";
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
  })[action];

export function ButtonRemapPanel({
  remap,
  ready,
  onStage,
  onApply = () => {},
  onDiscard = () => {},
}: {
  remap: Remap;
  ready: boolean;
  onStage(button: number, action: Action): void;
  onApply?(): void;
  onDiscard?(): void;
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
            }))}
            onChange={(val) => onStage(button.Button, val as Action)}
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

      <div className="status" role="status">
        {remap.Firmware === "success"
          ? "Button remapping applied"
          : remap.Firmware === "failed"
          ? `Button remapping failed: ${remap.Error.Code}`
          : "Remap draft pending confirmation"}
      </div>
    </article>
  );
}
