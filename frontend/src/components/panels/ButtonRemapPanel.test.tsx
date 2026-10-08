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
  it.each(["mouse_left", "mouse_right", "mouse_middle", "mouse_back", "mouse_forward"])("stages one or two complete %s clicks by saved ID", (type) => {
    const onStageMacro = vi.fn();
    const events = ["down", "up"].map((action) => ({ type, action, delay_ms: 0 }));
    const macros = [{ id: "single", name: "Same name", events }, { id: "mixed", name: "Same name", events: [...events, ...clickMacro.events] }];
    render(<ButtonRemapPanel remap={remap} ready macros={macros as never} onStage={vi.fn()} onStageMacro={onStageMacro} />);
    fireEvent.change(screen.getByLabelText("Macro target button"), { target: { value: "4" } });
    for (const id of ["single", "mixed"]) {
      fireEvent.change(screen.getByLabelText("Saved macro"), { target: { value: id } });
      expect(screen.getByRole("button", { name: "Stage macro assignment" })).toBeEnabled();
      fireEvent.click(screen.getByRole("button", { name: "Stage macro assignment" }));
      expect(onStageMacro).toHaveBeenLastCalledWith(id, 4, 1);
    }
  });

  it("rechecks the selected saved ID after library changes", () => {
    const onStageMacro = vi.fn();
    const props = { remap, ready: true, onStage: vi.fn(), onStageMacro };
    const view = render(<ButtonRemapPanel {...props} macros={[clickMacro] as never} />);
    fireEvent.change(screen.getByLabelText("Saved macro"), { target: { value: "click" } });
    view.rerender(<ButtonRemapPanel {...props} macros={[{ ...clickMacro, events: [] }] as never} />);
    expect(screen.getByRole("button", { name: "Stage macro assignment" })).toBeDisabled();
    view.rerender(<ButtonRemapPanel {...props} macros={[]} />);
    expect(screen.getByLabelText("Saved macro")).toHaveValue("");
    expect(onStageMacro).not.toHaveBeenCalled();
  });
  it("stages a saved name and fixed repeat without applying, rejecting invalid repeats", () => {
    const onStageMacro = vi.fn(); const onApply = vi.fn();
    render(<ButtonRemapPanel remap={remap} ready macros={[clickMacro] as never} onStage={vi.fn()} onStageMacro={onStageMacro} onApply={onApply} />);
    fireEvent.change(screen.getByLabelText("Macro target button"), { target: { value: "7" } });
    fireEvent.change(screen.getByLabelText("Saved macro"), { target: { value: "click" } });
    for (const value of ["0", "256", "1.5", ""]) {
      fireEvent.change(screen.getByLabelText("Fixed repetitions"), { target: { value } });
      expect(screen.getByRole("button", { name: "Stage macro assignment" })).toBeDisabled();
    }
    fireEvent.change(screen.getByLabelText("Fixed repetitions"), { target: { value: "255" } });
    fireEvent.click(screen.getByRole("button", { name: "Stage macro assignment" }));
    expect(onStageMacro).toHaveBeenCalledWith("click", 7, 255);
    expect(onApply).not.toHaveBeenCalled();
  });

  it("rejects timing and sequence projection and isolates input keyboard shortcuts", () => {
    const onApply = vi.fn(); const onDiscard = vi.fn(); const onStageMacro = vi.fn();
    const unsupported = { ...clickMacro, id: "timed", events: clickMacro.events.map((event) => ({ ...event, delay_ms: 2 })) };
    render(<ButtonRemapPanel remap={remap} ready macros={[unsupported] as never} onStage={vi.fn()} onStageMacro={onStageMacro} onApply={onApply} onDiscard={onDiscard} />);
    fireEvent.change(screen.getByLabelText("Saved macro"), { target: { value: "timed" } });
    expect(screen.getByRole("button", { name: "Stage macro assignment" })).toBeDisabled();
    expect(screen.getByText(/Backend admission/)).toHaveTextContent("one or two complete");
    fireEvent.keyDown(screen.getByLabelText("Fixed repetitions"), { key: "Enter" });
    fireEvent.keyDown(screen.getByLabelText("Saved macro"), { key: "Escape" });
    expect(onApply).not.toHaveBeenCalled(); expect(onDiscard).not.toHaveBeenCalled();
  });

  it("shows the singular overlay instead of the underlying ordinary action and reports partial transport", () => {
    render(<ButtonRemapPanel remap={{ ...remap, MacroPending: { ID: "click", Name: "Saved click", Button: 6, Repeat: 2, Events: [] }, MacroProgress: { Assignment: 2, Upload: 1 }, Firmware: "failed" } as never} ready onStage={vi.fn()} />);
    expect(screen.getByLabelText("Remap assignment summary")).toHaveTextContent("Button 6: Saved click × 2");
    expect(screen.getByRole("status")).toHaveTextContent(/partial|unknown/i);
    expect(screen.getByRole("status")).toHaveTextContent(/playback.*unverified/i);
  });
  it.each([
    [],
    [...clickMacro.events, ...clickMacro.events, ...clickMacro.events],
    [clickMacro.events[0]],
    [...clickMacro.events, clickMacro.events[0]],
    [clickMacro.events[0], clickMacro.events[0]],
    [clickMacro.events[0], { ...clickMacro.events[1], delay_ms: 1 }],
    [{ ...clickMacro.events[0], type: "keyboard" }, { ...clickMacro.events[1], type: "keyboard" }],
    [...clickMacro.events, clickMacro.events[1], clickMacro.events[0]],
    [clickMacro.events[1], clickMacro.events[0]],
    [clickMacro.events[0], { ...clickMacro.events[1], type: "mouse_right" }],
  ].map((events) => ({ events })))("does not stage unsupported event layout %j", ({ events }) => {
    const onStageMacro = vi.fn();
    render(<ButtonRemapPanel remap={remap} ready macros={[{ ...clickMacro, events }] as never} onStage={vi.fn()} onStageMacro={onStageMacro} />);
    fireEvent.change(screen.getByLabelText("Saved macro"), { target: { value: "click" } });
    fireEvent.click(screen.getByRole("button", { name: "Stage macro assignment" }));
    expect(onStageMacro).not.toHaveBeenCalled();
  });

  it("disables staging and panel shortcuts while disconnected or busy", () => {
    const onApply = vi.fn(); const onDiscard = vi.fn();
    render(<ButtonRemapPanel remap={remap} ready={false} macros={[clickMacro] as never} onStage={vi.fn()} onStageMacro={vi.fn()} onApply={onApply} onDiscard={onDiscard} />);
    expect(screen.getByLabelText("Saved macro")).toBeDisabled();
    const panel = screen.getByRole("heading", { name: "Button remapping" }).parentElement!;
    fireEvent.keyDown(panel, { key: "Enter" }); fireEvent.keyDown(panel, { key: "Escape" });
    expect(onApply).not.toHaveBeenCalled(); expect(onDiscard).not.toHaveBeenCalled();
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
