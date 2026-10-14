import "@testing-library/jest-dom/vitest";
import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import { ButtonRemapPanel } from "./ButtonRemapPanel";

const remap = {
  Pending: { Buttons: [
    { Button: 1, Action: "left", PreservedDefault: "" }, { Button: 2, Action: "right", PreservedDefault: "" },
    { Button: 3, Action: "middle", PreservedDefault: "" }, { Button: 4, Action: "forward", PreservedDefault: "" },
    { Button: 5, Action: "backward", PreservedDefault: "" }, { Button: 6, Action: null, PreservedDefault: "DPI+" }, { Button: 7, Action: null, PreservedDefault: "DPI-" },
  ] },
  Actions: ["off", "left", "right", "middle", "forward", "backward", "double_click", "fire"], Firmware: "", Error: { Code: "" },
};

const multimediaRemap = {
  ...remap,
  Actions: [...remap.Actions, "media_player", "play_pause", "stop", "previous_track", "next_track", "volume_up", "volume_down", "mute"],
};
const mouseControlsRemap = {
  ...multimediaRemap,
  Actions: [...multimediaRemap.Actions, "scroll_up", "scroll_down", "dpi_cycle", "dpi_plus", "dpi_minus"],
};

const browserActions = ["browser_calculator", "browser_email", "browser_forward", "browser_backward", "browser_stop", "browser_my_computer", "browser_refresh", "browser_home", "browser_search"];
const browserRemap = { ...mouseControlsRemap, Actions: [...mouseControlsRemap.Actions, ...browserActions] };

const clickMacro = { id: "click", name: "Saved click", events: [
  { type: "mouse_left", action: "down", delay_ms: 0 },
  { type: "mouse_left", action: "up", delay_ms: 0 },
] } as const;

beforeAll(() => {
  // jsdom has no top-layer dialog behavior; Chromium checks the real modal.
  HTMLDialogElement.prototype.showModal = function () { this.open = true; };
  HTMLDialogElement.prototype.close = function () { this.open = false; };
});
afterEach(cleanup);

function openMacro(button: number) {
  fireEvent.click(screen.getByRole("button", { name: `Button ${button} action` }));
  fireEvent.click(screen.getByRole("menuitem", { name: "Macro", exact: true }));
  return screen.getByRole("dialog", { name: `Button ${button} macro assignment` });
}
function selectSavedMacro(dialog: HTMLElement, name: string) {
  fireEvent.click(within(dialog).getByRole("button", { name: "Saved macro", exact: true }));
  fireEvent.click(within(dialog).getByRole("menuitem", { name, exact: true }));
}
function chooseMacro(button: number, name: string) {
  const dialog = openMacro(button);
  selectSavedMacro(dialog, name);
  fireEvent.click(within(dialog).getByRole("button", { name: "Confirm" }));
}

describe("ButtonRemapPanel", () => {
  it("offers saved F/D/S ten-event macro and stages only on Confirm, never Apply", () => {
    const events = ["KeyF", "KeyD", "KeyS", "KeyF", "KeyD"].flatMap(key_code => [
      { type: "keyboard", key_code, action: "down", delay_ms: 10 },
      { type: "keyboard", key_code, action: "up", delay_ms: 10 },
    ]);
    const macro = { id: "fds", name: "F/D/S ten", events };
    const onStageMacro = vi.fn(), onApply = vi.fn();
    render(<ButtonRemapPanel remap={remap as never} ready macros={[macro] as never} onStage={vi.fn()} onStageMacro={onStageMacro} onApply={onApply} />);
    const dialog = openMacro(7);
    selectSavedMacro(dialog, macro.name);
    expect(onStageMacro).not.toHaveBeenCalled();
    fireEvent.click(within(dialog).getByRole("button", { name: "Confirm" }));
    expect(onStageMacro).toHaveBeenCalledExactlyOnceWith("fds", 7, 1);
    expect(onApply).not.toHaveBeenCalled();
    expect(macro.events).toEqual(events);
  });
  it.each([
    [["Enter", "down"], ["NumpadEnter", "down"], ["Enter", "up"], ["NumpadEnter", "up"]],
    [["Enter", "down"], ["NumpadEnter", "up"]],
    [["Enter", "down"], ["Enter", "down"], ["Enter", "up"]],
    [["constructor", "down"], ["constructor", "up"]],
  ])("does not offer ambiguous or inherited identities: %j", (...codes) => {
    const events = codes.map(([key_code, action]) => ({ type: "keyboard", key_code, action, delay_ms: 10 }));
    const macro = { id: "invalid", name: "Invalid retained", events };
    const before = structuredClone(macro);
    const onStageMacro = vi.fn(), onApply = vi.fn();
    render(<ButtonRemapPanel remap={remap as never} ready macros={[clickMacro, macro] as never} onStage={vi.fn()} onStageMacro={onStageMacro} onApply={onApply} />);
    const dialog = openMacro(7);
    fireEvent.click(within(dialog).getByRole("button", { name: "Saved macro" }));
    expect(within(dialog).queryByRole("menuitem", { name: macro.name })).not.toBeInTheDocument();
    expect(onStageMacro).not.toHaveBeenCalled(); expect(onApply).not.toHaveBeenCalled();
    expect(macro).toEqual(before);
  });
  it("stages supported timed keyboard while retaining unsupported saved locals", () => {
    const onStageMacro=vi.fn();
    const events=[{type:"keyboard",key_code:"ShiftLeft",action:"down",delay_ms:100},{type:"keyboard",key_code:"KeyA",action:"down",delay_ms:100},{type:"keyboard",key_code:"KeyA",action:"up",delay_ms:100},{type:"keyboard",key_code:"ShiftLeft",action:"up",delay_ms:100}];
    const macros=[{id:"chord",name:"Captured chord",events},{id:"unknown",name:"Unsupported key",events:events.map(e=>({...e,key_code:"F13"}))}];
    render(<ButtonRemapPanel remap={remap as never} ready macros={macros as never} onStage={vi.fn()} onStageMacro={onStageMacro}/>);
    chooseMacro(6,"Captured chord");
    expect(onStageMacro).toHaveBeenCalledExactlyOnceWith("chord",6,1);
    expect(macros).toHaveLength(2);
  });
  it("keeps opaque custom selector menus inside the modal top layer", () => {
    render(<ButtonRemapPanel remap={remap as never} ready macros={[clickMacro] as never} onStage={vi.fn()} onStageMacro={vi.fn()} />);
    const dialog = openMacro(6);
    const selector = within(dialog).getByRole("button", { name: "Playback mode", exact: true });
    fireEvent.click(selector);
    const popup = within(dialog).getByRole("menu", { name: "Playback mode categories" });
    expect(popup).toHaveClass("gnome-select-popup");
    expect(within(popup).getAllByRole("menuitem")).toHaveLength(3);
    fireEvent.keyDown(selector, { key: "Escape" });
    expect(within(dialog).queryByRole("menu")).not.toBeInTheDocument();
    expect(selector).toHaveFocus();
    expect(dialog).toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole("button", { name: "Saved macro", exact: true }));
    expect(within(dialog).getByRole("menu", { name: "Saved macro categories" })).toHaveClass("gnome-select-popup");
  });
  it("names playback modes by how they repeat and stop", () => {
    render(<ButtonRemapPanel remap={remap as never} ready macros={[clickMacro] as never} onStage={vi.fn()} onStageMacro={vi.fn()} />);
    const dialog = openMacro(6);
    expect(within(dialog).getByLabelText("Playback mode")).toHaveTextContent("Repeat a set number of times");
    fireEvent.click(within(dialog).getByLabelText("Playback mode"));
    expect(within(dialog).getByRole("menuitem", { name: "Repeat until pressed again", exact: true })).toBeInTheDocument();
    expect(within(dialog).getByRole("menuitem", { name: "Repeat while button is held", exact: true })).toBeInTheDocument();
  });
  it.each([[1, "Repeat until pressed again"], [2, "Repeat while button is held"]])("restores mode %s and stages repeat one without a fixed count", (mode, label) => {
    const onStageMacro = vi.fn();
    const draft = { ID: "click", Name: "Saved click", Button: 6, Repeat: 255, Mode: mode, Events: [...clickMacro.events] };
    render(<ButtonRemapPanel remap={{ ...remap, MacroDrafts: { 6: draft } } as never} ready macros={[clickMacro] as never} onStage={vi.fn()} onStageMacro={onStageMacro} />);
    expect(screen.getByRole("button", { name: "Button 6 action" })).toHaveTextContent(label);
    const dialog = openMacro(6);
    expect(within(dialog).getByLabelText("Playback mode")).toHaveTextContent(label);
    expect(within(dialog).queryByLabelText("Repeat (1–255)")).not.toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole("button", { name: "Confirm" }));
    expect(onStageMacro).toHaveBeenCalledExactlyOnceWith("click", 6, 1, mode);
  });
  it("defaults legacy drafts to fixed count and can select continuous", () => {
    const onStageMacro = vi.fn();
    render(<ButtonRemapPanel remap={remap as never} ready macros={[clickMacro] as never} onStage={vi.fn()} onStageMacro={onStageMacro} />);
    const dialog = openMacro(6);
    expect(within(dialog).getByLabelText("Playback mode")).toHaveTextContent("Repeat a set number of times");
    selectSavedMacro(dialog, "Saved click");
    fireEvent.click(within(dialog).getByLabelText("Playback mode"));
    fireEvent.click(within(dialog).getByRole("menuitem", { name: "Repeat until pressed again", exact: true }));
    expect(within(dialog).getByText(/press again to stop/i)).toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole("button", { name: "Confirm" }));
    expect(onStageMacro).toHaveBeenCalledWith("click", 6, 1, 1);
  });
  it("opens a direct Macro dialog and stages only on Confirm", () => {
    const onStageMacro = vi.fn(); const onApply = vi.fn();
    render(<ButtonRemapPanel remap={remap as never} ready macros={[clickMacro] as never} onStage={vi.fn()} onStageMacro={onStageMacro} onApply={onApply} />);
    expect(screen.queryAllByRole("combobox")).toHaveLength(0);
    fireEvent.click(screen.getByRole("button", { name: "Button 6 action" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Macro", exact: true }));
    const dialog = screen.getByRole("dialog", { name: "Button 6 macro assignment" });
    selectSavedMacro(dialog, "Saved click");
    fireEvent.change(within(dialog).getByLabelText("Repeat (1–255)"), { target: { value: "255" } });
    expect(onStageMacro).not.toHaveBeenCalled();
    fireEvent.click(within(dialog).getByRole("button", { name: "Confirm" }));
    expect(onStageMacro).toHaveBeenCalledExactlyOnceWith("click", 6, 255);
    expect(onApply).not.toHaveBeenCalled();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("makes Macro discoverable for every button without persistent macro columns", () => {
    const onStageMacro = vi.fn(); const onApply = vi.fn();
    render(<ButtonRemapPanel remap={remap as never} ready macros={[clickMacro] as never} onStage={vi.fn()} onStageMacro={onStageMacro} onApply={onApply} />);
    for (let button = 1; button <= 7; button++) {
      const dialog = openMacro(button);
      expect(within(dialog).getByLabelText("Repeat (1–255)")).toHaveValue(1);
      fireEvent.click(within(dialog).getByRole("button", { name: "Cancel" }));
    }
    chooseMacro(1, "Saved click");
    expect(onStageMacro).toHaveBeenCalledExactlyOnceWith("click", 1, 1);
    expect(onApply).not.toHaveBeenCalled();
  });
  it("admits only compatible saved macros and clears only the chosen button", () => {
    const onClearMacro = vi.fn(); const onStageMacro = vi.fn();
    const draft = { ID: "click", Name: "Saved click", Button: 6, Repeat: 2, Events: [...clickMacro.events] };
    render(<ButtonRemapPanel remap={{ ...remap, MacroDrafts: { 6: draft, 7: { ...draft, Button: 7, Repeat: 5 } } } as never}
      ready macros={[clickMacro, { ...clickMacro, id: "overflow", name: "Overflow", events: clickMacro.events.map((event) => ({ ...event, delay_ms: 51200 })) }] as never}
      onStage={vi.fn()} onStageMacro={onStageMacro} onClearMacro={onClearMacro} />);
    const dialog = openMacro(6);
    fireEvent.click(within(dialog).getByRole("button", { name: "Saved macro", exact: true }));
    expect(within(dialog).queryByRole("menuitem", { name: "Overflow", exact: true })).not.toBeInTheDocument();
    fireEvent.keyDown(within(dialog).getByRole("button", { name: "Saved macro", exact: true }), { key: "Escape" });
    fireEvent.change(within(dialog).getByLabelText("Repeat (1–255)"), { target: { value: "7" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Cancel" }));
    expect(onClearMacro).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Button 7 action" })).toHaveTextContent("Saved click × 5");
    expect(onStageMacro).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Button 6 action" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Basic" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Fire" }));
    expect(onClearMacro).toHaveBeenCalledExactlyOnceWith(6);
  });

  it("disables macro controls while the library is unavailable or assignment is busy", () => {
    const { rerender } = render(<ButtonRemapPanel remap={remap as never} ready libraryReady={false} macros={[clickMacro] as never} onStage={vi.fn()} onStageMacro={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: "Button 2 action" }));
    expect(screen.getByRole("menuitem", { name: "Macro" })).toHaveAttribute("aria-disabled", "true");
    fireEvent.click(screen.getByRole("menuitem", { name: "Macro" }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    rerender(<ButtonRemapPanel remap={remap as never} ready assignmentAvailable={false} macros={[clickMacro] as never} onStage={vi.fn()} onStageMacro={vi.fn()} />);
    expect(screen.getByRole("button", { name: "Button 2 action" })).toBeDisabled();
  });

  it("Escape discards local edits and reopening restores the staged assignment", () => {
    const onStageMacro = vi.fn(); const onDiscard = vi.fn();
    const draft = { ID: "click", Name: "Saved click", Button: 6, Repeat: 2, Events: [...clickMacro.events] };
    render(<ButtonRemapPanel remap={{ ...remap, MacroDrafts: { 6: draft } } as never} ready macros={[clickMacro] as never}
      onStage={vi.fn()} onStageMacro={onStageMacro} onDiscard={onDiscard} />);
    const dialog = openMacro(6);
    fireEvent.change(within(dialog).getByLabelText("Repeat (1–255)"), { target: { value: "255" } });
    fireEvent.keyDown(dialog, { key: "Escape" });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(onStageMacro).not.toHaveBeenCalled(); expect(onDiscard).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Button 6 action" })).toHaveFocus();
    expect(within(openMacro(6)).getByLabelText("Repeat (1–255)")).toHaveValue(2);
  });

  it("invalidates modal edits on device scope changes and guards busy or changed library state", () => {
    const onStageMacro = vi.fn(); const scope = {};
    const props = { remap: remap as never, ready: true, macros: [clickMacro] as never, onStage: vi.fn(), onStageMacro, assignmentScope: scope };
    const { rerender } = render(<ButtonRemapPanel {...props} />);
    const dialog = openMacro(6);
    selectSavedMacro(dialog, "Saved click");
    rerender(<ButtonRemapPanel {...props} ready={false} />);
    expect(within(dialog).getByRole("button", { name: "Confirm" })).toBeDisabled();
    expect(within(dialog).getByLabelText("Repeat (1–255)")).toBeDisabled();
    rerender(<ButtonRemapPanel {...props} macros={[]} />);
    expect(within(dialog).getByRole("button", { name: "Confirm" })).toBeDisabled();
    rerender(<ButtonRemapPanel {...props} assignmentScope={{}} />);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(onStageMacro).not.toHaveBeenCalled();
  });

  it("stages a saved macro directly within its button", () => {
    const onStageMacro = vi.fn();
    render(<ButtonRemapPanel remap={remap} ready macros={[clickMacro] as never} onStage={vi.fn()} onStageMacro={onStageMacro} />);

    chooseMacro(6, "Saved click");

    expect(onStageMacro).toHaveBeenCalledWith("click", 6, 1);
  });

  it.each(["mouse_left", "mouse_right", "mouse_middle", "mouse_back", "mouse_forward"])("stages one or two complete %s clicks from button dropdown", (type) => {
    const onStageMacro = vi.fn();
    const events = ["down", "up"].map((action) => ({ type, action, delay_ms: 0 }));
    const macros = [{ id: "single", name: "Single Click", events }, { id: "mixed", name: "Double Click", events: [...events, ...clickMacro.events] }];
    render(<ButtonRemapPanel remap={remap} ready macros={macros as never} onStage={vi.fn()} onStageMacro={onStageMacro} />);

    chooseMacro(4, "Double Click");

    expect(onStageMacro).toHaveBeenCalledWith("mixed", 4, 1);
  });

  it("shows no compatible macros without permitting confirmation", () => {
    render(<ButtonRemapPanel remap={remap} ready macros={[]} onStage={vi.fn()} onStageMacro={vi.fn()} />);
    const dialog = openMacro(6);
    const selector = within(dialog).getByRole("button", { name: "Saved macro", exact: true });
    expect(selector).toHaveTextContent("No compatible saved macros");
    expect(selector).toBeDisabled();
    expect(within(dialog).getByRole("button", { name: "Confirm" })).toBeDisabled();
  });

  it("reopens an assignment and validates local repetitions 1–255 before Confirm", () => {
    const onStageMacro = vi.fn();
    const remapWithMacro = {
      ...remap,
      MacroDrafts: {
        6: { ID: "click", Name: "Saved click", Button: 6, Repeat: 1, Events: [...clickMacro.events] },
      },
    };
    render(<ButtonRemapPanel remap={remapWithMacro as never} ready macros={[clickMacro] as never} onStage={vi.fn()} onStageMacro={onStageMacro} />);

    const dialog = openMacro(6);
    const repsInput = within(dialog).getByLabelText("Repeat (1–255)");
    expect(repsInput).toHaveValue(1);

    fireEvent.change(repsInput, { target: { value: "5" } });
    expect(onStageMacro).not.toHaveBeenCalled();

    // Invalid local repetitions disable confirmation without staging.
    for (const invalid of ["0", "256", "abc", ""]) {
      fireEvent.change(repsInput, { target: { value: invalid } });
      expect(onStageMacro).not.toHaveBeenCalled();
      expect(within(dialog).getByRole("button", { name: "Confirm" })).toBeDisabled();
    }

    fireEvent.change(repsInput, { target: { value: "255" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Confirm" }));
    expect(onStageMacro).toHaveBeenCalledExactlyOnceWith("click", 6, 255);
  });

  it("supports multiple buttons having macros assigned simultaneously", () => {
    const onStageMacro = vi.fn();
    const remapWithTwoMacros = {
      ...remap,
      MacroDrafts: {
        6: { ID: "click1", Name: "Macro One", Button: 6, Repeat: 2, Events: [...clickMacro.events] },
        7: { ID: "click2", Name: "Macro Two", Button: 7, Repeat: 5, Events: [...clickMacro.events] },
      },
    };
    render(<ButtonRemapPanel remap={remapWithTwoMacros as never} ready macros={[
      { id: "click1", name: "Macro One", events: [...clickMacro.events] },
      { id: "click2", name: "Macro Two", events: [...clickMacro.events] },
    ] as never} onStage={vi.fn()} onStageMacro={onStageMacro} />);

    expect(screen.getByRole("button", { name: "Button 6 action" })).toHaveTextContent("Macro One × 2");
    expect(screen.getByRole("button", { name: "Button 7 action" })).toHaveTextContent("Macro Two × 5");
    const dialog = openMacro(6);
    expect(within(dialog).getByRole("button", { name: "Saved macro", exact: true })).toHaveTextContent("Macro One");
    expect(within(dialog).getByLabelText("Repeat (1–255)")).toHaveValue(2);
    fireEvent.click(within(dialog).getByRole("button", { name: "Cancel" }));
    const otherDialog = openMacro(7);
    expect(within(otherDialog).getByRole("button", { name: "Saved macro", exact: true })).toHaveTextContent("Macro Two");
    expect(within(otherDialog).getByLabelText("Repeat (1–255)")).toHaveValue(5);
    fireEvent.click(within(otherDialog).getByRole("button", { name: "Cancel" }));

    const summary = screen.getByLabelText("Remap assignment summary").textContent;
    expect(summary).toContain("Button 6: Macro One × 2");
    expect(summary).toContain("Button 7: Macro Two × 5");
  });

  it("switches a button from a macro back to an ordinary action and clears macro", () => {
    const onStage = vi.fn();
    const onClearMacro = vi.fn();
    const remapWithMacro = {
      ...remap,
      MacroDrafts: {
        6: { ID: "click", Name: "Saved click", Button: 6, Repeat: 1, Events: [...clickMacro.events] },
      },
    };
    render(<ButtonRemapPanel remap={remapWithMacro as never} ready macros={[clickMacro] as never} onStage={onStage} onClearMacro={onClearMacro} />);

    fireEvent.click(screen.getByRole("button", { name: "Button 6 action" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Basic" }));
    const submenu = screen.getByRole("menu", { name: "Basic actions" });
    fireEvent.click(within(submenu).getByRole("menuitem", { name: "Fire" }));

    expect(onStage).toHaveBeenCalledWith(6, "fire");
    expect(onClearMacro).toHaveBeenCalledWith(6);
  });

  it("disables action selectors when disconnected or busy", () => {
    const onApply = vi.fn(); const onDiscard = vi.fn();
    render(<ButtonRemapPanel remap={remap} ready={false} macros={[clickMacro] as never} onStage={vi.fn()} onApply={onApply} onDiscard={onDiscard} />);

    expect(screen.getByRole("button", { name: "Button 1 action" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Button 6 action" })).toBeDisabled();

    const panel = screen.getByRole("heading", { name: "Button remapping" }).parentElement!;
    fireEvent.keyDown(panel, { key: "Enter" }); fireEvent.keyDown(panel, { key: "Escape" });
    expect(onApply).not.toHaveBeenCalled(); expect(onDiscard).not.toHaveBeenCalled();
  });

  it("shows overlay in summary and reports partial transport when upload fails", () => {
    render(<ButtonRemapPanel remap={{ ...remap, MacroPending: { ID: "click", Name: "Saved click", Button: 6, Repeat: 2, Events: [] }, MacroProgress: { Assignment: 2, Upload: 1 }, Firmware: "failed" } as never} ready onStage={vi.fn()} />);
    expect(screen.getByLabelText("Remap assignment summary")).toHaveTextContent("Button 6: Saved click × 2");
    expect(screen.getByRole("status")).toHaveTextContent(/partial|unknown/i);
    expect(screen.getByRole("status")).toHaveTextContent(/playback.*unverified/i);
  });

  it("does not reuse prior transport success for a changed macro draft", () => {
    const draft = { ID: "click", Name: "Saved click", Button: 6, Repeat: 2, Events: [] };
    render(<ButtonRemapPanel remap={{ ...remap, Applied: remap.Pending, MacroPending: { ...draft, Button: 7 }, MacroApplied: draft, Firmware: "success" } as never} ready onStage={vi.fn()} />);
    expect(screen.getByRole("status")).toHaveTextContent("current draft not confirmed");
  });

  it.each(["success", "failed", "pending"])("does not treat residual macro progress as an active macro when ordinary transport is %s", (Firmware) => {
    render(<ButtonRemapPanel remap={{ ...remap, Applied: remap.Pending, MacroPending: null, MacroApplied: null,
      MacroProgress: { Assignment: 2, Upload: 1 }, Firmware } as never} ready onStage={vi.fn()} />);
    const status = screen.getByRole("status");
    expect(status).not.toHaveTextContent("Remap and macro transport confirmed");
    expect(status).not.toHaveTextContent("Macro assignment staged locally");
    expect(status).toHaveTextContent(Firmware === "success" ? "Button remapping applied" : Firmware === "failed" ? "Button remapping failed" : "Remap draft pending confirmation");
    expect(screen.getByText(/Prior macro transport progress/)).toHaveTextContent("partial or unknown");
  });

  it("still confirms a matching applied macro rather than ordinary transport", () => {
    const draft = { ID: "click", Name: "Saved click", Button: 6, Repeat: 2, Events: clickMacro.events };
    render(<ButtonRemapPanel remap={{ ...remap, Applied: remap.Pending, MacroPending: draft, MacroApplied: draft,
      MacroProgress: { Assignment: 2, Upload: 2 }, Firmware: "success" } as never} ready onStage={vi.fn()} />);
    expect(screen.getByRole("status")).toHaveTextContent("Remap and macro transport confirmed");
    expect(screen.queryByText(/Prior macro transport progress/)).not.toBeInTheDocument();
  });

  it("renders exactly seven closed-action selectors and preserves DPI markers", () => {
    render(<ButtonRemapPanel remap={remap} ready onStage={vi.fn()} />);
    expect(screen.getAllByRole("button", { name: /Button \d action/ })).toHaveLength(7);
    expect(screen.getByText(/Button 6 \(DPI\+\)/)).toBeInTheDocument();
    expect(screen.getByText(/Button 7 \(DPI-\)/)).toBeInTheDocument();
  });

  it("stages Basic choices without applying and shows the complete confirmation summary", () => {
    const onStage = vi.fn();
    const onApply = vi.fn();
    render(<ButtonRemapPanel remap={{ ...remap, Firmware: "pending" }} ready onStage={onStage} onApply={onApply} />);
    const selector = screen.getByRole("button", { name: "Button 1 action" });
    fireEvent.click(selector);
    fireEvent.click(screen.getByRole("menuitem", { name: "Basic" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Fire" }));
    expect(onStage).toHaveBeenCalledWith(1, "fire");
    expect(onApply).not.toHaveBeenCalled();
    expect(screen.getByLabelText("Remap assignment summary")).toHaveTextContent("Button 1: Left, Button 2: Right, Button 3: Middle, Button 4: Forward, Button 5: Backward, Button 6: DPI+, Button 7: DPI-");
    expect(screen.getByRole("status")).toHaveTextContent("draft pending confirmation");
  });

  it("keeps Multimedia visible but unavailable for Button 1", () => {
    const onStage = vi.fn();
    render(<ButtonRemapPanel remap={multimediaRemap as never} ready onStage={onStage} />);

    fireEvent.click(screen.getByRole("button", { name: "Button 1 action" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Multimedia" }));
    const submenu = screen.getByRole("menu", { name: "Multimedia actions" });
    const disabledMediaPlayer = within(submenu).getByRole("menuitem", { name: "Media Player" });
    expect(disabledMediaPlayer).toHaveAttribute("aria-disabled", "true");
    fireEvent.click(disabledMediaPlayer);
    expect(onStage).not.toHaveBeenCalled();
  });

  it("keeps Mouse Controls visible, ordered, and inaccessible for Button 1", () => {
    const onStage = vi.fn();
    render(<ButtonRemapPanel remap={mouseControlsRemap as never} ready onStage={onStage} />);
    fireEvent.click(screen.getByRole("button", { name: "Button 1 action" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Mouse Controls" }));
    const submenu = screen.getByRole("menu", { name: "Mouse Controls actions" });
    const entries = ["Scroll Up", "Scroll Down", "DPI Cycle", "DPI+", "DPI−"];
    expect(within(submenu).getAllByRole("menuitem").map((entry) => entry.textContent)).toEqual(entries);
    for (const label of entries) {
      const entry = within(submenu).getByRole("menuitem", { name: label });
      expect(entry).toHaveAttribute("aria-disabled", "true");
      fireEvent.click(entry);
    }
    const trigger = screen.getByRole("button", { name: "Button 1 action" });
    fireEvent.keyDown(trigger, { key: "End" });
    fireEvent.keyDown(trigger, { key: "Enter" });
    fireEvent.keyDown(trigger, { key: " " });
    expect(onStage).not.toHaveBeenCalled();
  });

  it("stages a Mouse Controls action for an eligible button", () => {
    const onStage = vi.fn();
    render(<ButtonRemapPanel remap={mouseControlsRemap as never} ready onStage={onStage} />);
    fireEvent.click(screen.getByRole("button", { name: "Button 2 action" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Mouse Controls" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "DPI Cycle" }));
    expect(onStage).toHaveBeenCalledWith(2, "dpi_cycle");
  });

  it("allows Buttons 2–7 to stage Multimedia actions", () => {
    const onStage = vi.fn();
    render(<ButtonRemapPanel remap={multimediaRemap as never} ready onStage={onStage} />);

    for (let button = 2; button <= 7; button++) {
      fireEvent.click(screen.getByRole("button", { name: `Button ${button} action` }));
      fireEvent.click(screen.getByRole("menuitem", { name: "Multimedia" }));
      const submenu = screen.getByRole("menu", { name: "Multimedia actions" });
      const mediaPlayer = within(submenu).getByRole("menuitem", { name: "Media Player" });
      expect(mediaPlayer).toHaveAttribute("aria-disabled", "false");
      fireEvent.click(mediaPlayer);
    }
    expect(onStage).toHaveBeenCalledTimes(6);
    expect(onStage).toHaveBeenNthCalledWith(1, 2, "media_player");
    expect(onStage).toHaveBeenNthCalledWith(6, 7, "media_player");
  });

  it("shows nine ordered Browser choices and stages Button 1 without applying", () => {
    const onStage = vi.fn();
    const onApply = vi.fn();
    render(<ButtonRemapPanel remap={browserRemap as never} ready onStage={onStage} onApply={onApply} />);
    fireEvent.click(screen.getByRole("button", { name: "Button 1 action" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Browser" }));
    const menu = screen.getByRole("menu", { name: "Browser actions" });
    expect(within(menu).getAllByRole("menuitem").map((entry) => entry.textContent)).toEqual([
      "Calculator", "Email", "Forward", "Backward", "Stop", "My Computer", "Refresh", "Home", "Search",
    ]);
    const action = within(menu).getByRole("menuitem", { name: "Search" });
    expect(action).toHaveAttribute("aria-disabled", "false");
    fireEvent.click(action);
    expect(onStage).toHaveBeenCalledWith(1, "browser_search");
    expect(onApply).not.toHaveBeenCalled();
  });

  it("does not bubble selector keys into panel Apply or Discard shortcuts", () => {
    const onApply = vi.fn();
    const onDiscard = vi.fn();
    render(<ButtonRemapPanel remap={multimediaRemap as never} ready onStage={vi.fn()} onApply={onApply} onDiscard={onDiscard} />);
    const selector = screen.getByRole("button", { name: "Button 2 action" });

    fireEvent.keyDown(selector, { key: "Enter" });
    fireEvent.keyDown(selector, { key: "Escape" });
    expect(onApply).not.toHaveBeenCalled();
    expect(onDiscard).not.toHaveBeenCalled();
  });

  it("applies on Enter and discards on Escape while the panel is focused", () => {
    const onApply = vi.fn();
    const onDiscard = vi.fn();
    render(<ButtonRemapPanel remap={remap} ready onStage={vi.fn()} onApply={onApply} onDiscard={onDiscard} />);
    const panel = screen.getByRole("heading", { name: "Button remapping" }).parentElement!;

    fireEvent.keyDown(panel, { key: "Enter" });
    fireEvent.keyDown(panel, { key: "Escape" });

    expect(onApply).toHaveBeenCalledTimes(1);
    expect(onDiscard).toHaveBeenCalledTimes(1);
  });
});
