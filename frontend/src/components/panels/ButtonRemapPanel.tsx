type Action = "off" | "left" | "right" | "middle" | "forward" | "backward" | "double_click" | "fire";
type Button = { Button: number; Action: Action | null; PreservedDefault: string };
type Remap = { Pending: { Buttons: Button[] }; Applied: { Buttons: Button[] }; Actions: Action[]; Firmware: string; Error: { Code: string } };

const labelFor = (action: Action) => ({ off: "Off", left: "Left", right: "Right", middle: "Middle", forward: "Forward", backward: "Backward", double_click: "Double Click", fire: "Fire" })[action];

export function ButtonRemapPanel({ remap, ready, onStage, onApply = () => {}, onDiscard = () => {} }: { remap: Remap; ready: boolean; onStage(button: number, action: Action): void; onApply?(): void; onDiscard?(): void }) {
	  return <section className="panel-section" aria-labelledby="button-remap-title" tabIndex={0} onKeyDown={(event) => { if (event.key === "Enter") { event.preventDefault(); onApply(); } if (event.key === "Escape") { event.preventDefault(); onDiscard(); } }}>
    <h3 id="button-remap-title">Button remapping</h3>
	    <p>Review the complete assignment below, then press Enter to apply or Escape to discard.</p>
    {remap.Pending.Buttons.map((button) => <label key={button.Button}>
      <span>Button {button.Button}{button.PreservedDefault ? ` (${button.PreservedDefault})` : ""}</span>
      <select aria-label={`Button ${button.Button} action`} disabled={!ready} value={button.Action ?? ""} onChange={(event) => onStage(button.Button, event.target.value as Action)}>
        {remap.Actions.map((action) => <option key={action} value={action}>{labelFor(action)}</option>)}
      </select>
	    </label>)}
	    <p aria-label="Remap assignment summary">{remap.Pending.Buttons.map((button) => `Button ${button.Button}: ${button.Action ? labelFor(button.Action) : button.PreservedDefault}`).join(", ")}</p>
	    <button type="button" disabled={!ready} onClick={onApply}>Apply remap</button>
	    <button type="button" disabled={!ready} onClick={onDiscard}>Discard remap</button>
	    <div role="status">{remap.Firmware === "success" ? "Button remapping applied" : remap.Firmware === "failed" ? `Button remapping failed: ${remap.Error.Code}` : "Remap draft pending confirmation"}</div>
  </section>;
}
