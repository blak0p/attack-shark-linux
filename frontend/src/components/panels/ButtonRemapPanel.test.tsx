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

afterEach(cleanup);

describe("ButtonRemapPanel", () => {
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
