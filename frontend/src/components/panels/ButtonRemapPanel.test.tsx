import "@testing-library/jest-dom/vitest";
import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
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

afterEach(cleanup);

describe("ButtonRemapPanel", () => {
  it("makes saved macro and repeat controls discoverable for every button before assignment", () => {
    const onStageMacro = vi.fn(); const onApply = vi.fn();
    render(<ButtonRemapPanel remap={remap as never} ready macros={[clickMacro] as never} onStage={vi.fn()} onStageMacro={onStageMacro} onApply={onApply} />);
    for (let button = 1; button <= 7; button++) {
      expect(screen.getByRole("combobox", { name: `Button ${button} saved macro` })).toBeEnabled();
      expect(screen.getByLabelText(`Button ${button} repetitions`)).toHaveValue(1);
      expect(screen.getByLabelText(`Button ${button} repetitions`)).toBeDisabled();
    }
    fireEvent.change(screen.getByRole("combobox", { name: "Button 1 saved macro" }), { target: { value: "click" } });
    expect(onStageMacro).toHaveBeenCalledWith("click", 1, 1);
    expect(onApply).not.toHaveBeenCalled();
  });
  it("admits only compatible saved macros and clears only the chosen button", () => {
    const onClearMacro = vi.fn(); const onStageMacro = vi.fn();
    const draft = { ID: "click", Name: "Saved click", Button: 6, Repeat: 2, Events: [...clickMacro.events] };
    render(<ButtonRemapPanel remap={{ ...remap, MacroDrafts: { 6: draft, 7: { ...draft, Button: 7, Repeat: 5 } } } as never}
      ready macros={[clickMacro, { ...clickMacro, id: "timed", name: "Timed", events: clickMacro.events.map((event) => ({ ...event, delay_ms: 10 })) }] as never}
      onStage={vi.fn()} onStageMacro={onStageMacro} onClearMacro={onClearMacro} />);
    const selector = screen.getByRole("combobox", { name: "Button 6 saved macro" });
    expect(within(selector).queryByRole("option", { name: "Timed" })).not.toBeInTheDocument();
    fireEvent.change(selector, { target: { value: "" } });
    expect(onClearMacro).toHaveBeenCalledExactlyOnceWith(6);
    expect(screen.getByLabelText("Button 7 repetitions")).toHaveValue(5);
    expect(onStageMacro).not.toHaveBeenCalled();
  });

  it("disables macro controls while the library is unavailable or assignment is busy", () => {
    const { rerender } = render(<ButtonRemapPanel remap={remap as never} ready libraryReady={false} macros={[clickMacro] as never} onStage={vi.fn()} onStageMacro={vi.fn()} />);
    expect(screen.getByRole("combobox", { name: "Button 2 saved macro" })).toBeDisabled();
    rerender(<ButtonRemapPanel remap={remap as never} ready assignmentAvailable={false} macros={[clickMacro] as never} onStage={vi.fn()} onStageMacro={vi.fn()} />);
    expect(screen.getByRole("combobox", { name: "Button 2 saved macro" })).toBeDisabled();
  });

  it("stages a saved macro directly within its button", () => {
    const onStageMacro = vi.fn();
    render(<ButtonRemapPanel remap={remap} ready macros={[clickMacro] as never} onStage={vi.fn()} onStageMacro={onStageMacro} />);

    fireEvent.change(screen.getByRole("combobox", { name: "Button 6 saved macro" }), { target: { value: "click" } });

    expect(onStageMacro).toHaveBeenCalledWith("click", 6, 1);
  });

  it.each(["mouse_left", "mouse_right", "mouse_middle", "mouse_back", "mouse_forward"])("stages one or two complete %s clicks from button dropdown", (type) => {
    const onStageMacro = vi.fn();
    const events = ["down", "up"].map((action) => ({ type, action, delay_ms: 0 }));
    const macros = [{ id: "single", name: "Single Click", events }, { id: "mixed", name: "Double Click", events: [...events, ...clickMacro.events] }];
    render(<ButtonRemapPanel remap={remap} ready macros={macros as never} onStage={vi.fn()} onStageMacro={onStageMacro} />);

    fireEvent.change(screen.getByRole("combobox", { name: "Button 4 saved macro" }), { target: { value: "mixed" } });

    expect(onStageMacro).toHaveBeenCalledWith("mixed", 4, 1);
  });

  it("shows disabled 'No saved macros' item when no compatible macros exist", () => {
    render(<ButtonRemapPanel remap={remap} ready macros={[]} onStage={vi.fn()} />);
    const selector = screen.getByRole("combobox", { name: "Button 6 saved macro" });
    expect(within(selector).getAllByRole("option")).toHaveLength(1);
    expect(selector).toHaveTextContent("No compatible saved macros");
  });

  it("displays inline repetitions input when a button has a macro assigned and tunes 1-255", () => {
    const onStageMacro = vi.fn();
    const remapWithMacro = {
      ...remap,
      MacroDrafts: {
        6: { ID: "click", Name: "Saved click", Button: 6, Repeat: 1, Events: [...clickMacro.events] },
      },
    };
    render(<ButtonRemapPanel remap={remapWithMacro as never} ready macros={[clickMacro] as never} onStage={vi.fn()} onStageMacro={onStageMacro} />);

    const repsInput = screen.getByLabelText("Button 6 repetitions");
    expect(repsInput).toHaveValue(1);

    fireEvent.change(repsInput, { target: { value: "5" } });
    expect(onStageMacro).toHaveBeenCalledWith("click", 6, 5);

    // Invalid repetitions do not trigger onStageMacro
    onStageMacro.mockClear();
    for (const invalid of ["0", "256", "abc", ""]) {
      fireEvent.change(repsInput, { target: { value: invalid } });
      expect(onStageMacro).not.toHaveBeenCalled();
    }

    fireEvent.change(repsInput, { target: { value: "255" } });
    expect(onStageMacro).toHaveBeenCalledWith("click", 6, 255);
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

    expect(screen.getByRole("combobox", { name: "Button 6 saved macro" })).toHaveValue("click1");
    expect(screen.getByRole("combobox", { name: "Button 7 saved macro" })).toHaveValue("click2");

    expect(screen.getByLabelText("Button 6 repetitions")).toHaveValue(2);
    expect(screen.getByLabelText("Button 7 repetitions")).toHaveValue(5);

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
