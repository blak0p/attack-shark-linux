import "@testing-library/jest-dom/vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
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

afterEach(cleanup);

describe("ButtonRemapPanel", () => {
  it("renders exactly seven closed-action selectors and preserves DPI markers", () => {
    render(<ButtonRemapPanel remap={remap} ready onStage={vi.fn()} />);
    expect(screen.getAllByRole("combobox")).toHaveLength(7);
    expect(screen.getByText(/Button 6 \(DPI\+\)/)).toBeInTheDocument();
    expect(screen.getByText(/Button 7 \(DPI-\)/)).toBeInTheDocument();
  });

  it("keeps selections in the draft and shows the complete confirmation summary", () => {
    const onStage = vi.fn();
    const onApply = vi.fn();
    render(<ButtonRemapPanel remap={{ ...remap, Firmware: "pending" }} ready onStage={onStage} onApply={onApply} />);
    const combobox = screen.getAllByRole("combobox")[0];
    fireEvent.click(combobox);
    const option = screen.getByRole("option", { name: "Fire" });
    fireEvent.click(option);
    expect(onStage).toHaveBeenCalledWith(1, "fire");
    expect(onApply).not.toHaveBeenCalled();
    expect(screen.getByLabelText("Remap assignment summary")).toHaveTextContent("Button 1: Left, Button 2: Right, Button 3: Middle, Button 4: Forward, Button 5: Backward, Button 6: DPI+, Button 7: DPI-");
    expect(screen.getByRole("status")).toHaveTextContent("draft pending confirmation");
  });

  it("groups multimedia actions and disables them only for Button 1", () => {
    const onStage = vi.fn();
    const multimediaRemap = {
      ...remap,
      Actions: [...remap.Actions, "media_player", "play_pause", "stop", "previous_track", "next_track", "volume_up", "volume_down", "mute"],
    };
    render(<ButtonRemapPanel remap={multimediaRemap as never} ready onStage={onStage} />);

    const selectors = screen.getAllByRole("combobox");
    fireEvent.click(selectors[0]);
    expect(screen.getByRole("group", { name: "Multimedia" })).toBeInTheDocument();
    const disabledMediaPlayer = screen.getByRole("option", { name: "Media Player" });
    expect(disabledMediaPlayer).toHaveAttribute("aria-disabled", "true");
    fireEvent.click(disabledMediaPlayer);
    expect(onStage).not.toHaveBeenCalled();
    fireEvent.keyDown(selectors[0], { key: "End" });
    expect(selectors[0]).toHaveAttribute("aria-activedescendant", expect.stringMatching(/-opt-7$/));

    fireEvent.click(selectors[1]);
    const enabledMediaPlayer = Array.from(screen.getByRole("listbox", { name: "Button 2 action" }).querySelectorAll('[role="option"]')).find((option) => option.textContent === "Media Player")!;
    expect(enabledMediaPlayer).toHaveAttribute("aria-disabled", "false");
    fireEvent.click(enabledMediaPlayer);
    expect(onStage).toHaveBeenCalledWith(2, "media_player");
  });

  it("preserves ordered groups and disabled semantics for all seven selectors", () => {
    const multimediaRemap = {
      ...remap,
      Actions: [...remap.Actions, "media_player", "play_pause", "stop", "previous_track", "next_track", "volume_up", "volume_down", "mute"],
    };
    render(<ButtonRemapPanel remap={multimediaRemap as never} ready onStage={vi.fn()} />);

    const selectors = screen.getAllByRole("combobox");
    selectors.forEach((selector) => fireEvent.click(selector));
    const listboxes = screen.getAllByRole("listbox");
    expect(listboxes).toHaveLength(7);
    listboxes.forEach((listbox, index) => {
      const groups = Array.from(listbox.querySelectorAll('[role="group"]')).map((group) => group.getAttribute("aria-label"));
      expect(groups).toEqual(["Basic", "Multimedia"]);
      const multimedia = Array.from(listbox.querySelectorAll('[role="group"][aria-label="Multimedia"] [role="option"]'));
      expect(multimedia.map((option) => option.textContent)).toEqual(["Media Player", "Play/Pause", "Stop", "Previous Track", "Next Track", "Volume Up", "Volume Down", "Mute"]);
      expect(multimedia.every((option) => option.getAttribute("aria-disabled") === (index === 0 ? "true" : "false"))).toBe(true);
    });
  });

  it("applies on Enter and discards on Escape only while the panel is focused", () => {
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
