import "@testing-library/jest-dom/vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { GnomeSelect } from "./GnomeSelect";

const options = [
  { value: "left", label: "Left", group: "Basic" },
  { value: "fire", label: "Fire", group: "Basic" },
  { value: "play_pause", label: "Play/Pause", group: "Multimedia" },
  { value: "mute", label: "Mute", group: "Multimedia", disabled: true },
];

afterEach(cleanup);

describe("GnomeSelect", () => {
  it("opens with categories only and opens an adjacent submenu by pointer", () => {
    render(<GnomeSelect aria-label="Button action" value="left" options={options} onChange={vi.fn()} />);

    fireEvent.click(screen.getByRole("button", { name: "Button action" }));
    const categoryMenu = screen.getByRole("menu", { name: "Button action categories" });
    expect(categoryMenu).toBeInTheDocument();
    expect(categoryMenu.parentElement).toBe(document.body);
    expect(screen.getAllByRole("menuitem").map((item) => item.textContent)).toEqual(["Basic▸", "Multimedia▸"]);
    expect(screen.queryByRole("menu", { name: "Basic actions" })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("menuitem", { name: "Multimedia" }));
    expect(screen.getByRole("menu", { name: "Multimedia actions" })).toBeInTheDocument();
    expect(screen.getAllByRole("menuitem").map((item) => item.textContent)).toEqual(["Basic▸", "Multimedia▸", "Play/Pause", "Mute"]);
  });

  it("anchors the portaled menu to the trigger and updates it when a scroll ancestor moves", () => {
    render(<GnomeSelect aria-label="Button action" value="left" options={options} onChange={vi.fn()} />);
    const trigger = screen.getByRole("button", { name: "Button action" });
    let bounds = { left: 24, bottom: 140 };
    Object.defineProperty(trigger, "getBoundingClientRect", {
      configurable: true,
      value: () => ({ ...bounds, top: bounds.bottom - 40, right: bounds.left + 160, width: 160, height: 40 }),
    });

    fireEvent.click(trigger);
    const categoryMenu = screen.getByRole("menu", { name: "Button action categories" });
    expect(categoryMenu).toHaveStyle({ top: "144px", left: "24px" });

    bounds = { left: 48, bottom: 180 };
    fireEvent.scroll(window);
    expect(categoryMenu).toHaveStyle({ top: "184px", left: "48px" });
  });

  it("traverses categories and actions by keyboard, closes levels, and restores trigger focus", () => {
    const onChange = vi.fn();
    render(<GnomeSelect aria-label="Button action" value="left" options={options} onChange={onChange} />);
    const trigger = screen.getByRole("button", { name: "Button action" });
    trigger.focus();

    fireEvent.keyDown(trigger, { key: "ArrowDown" });
    fireEvent.keyDown(trigger, { key: "ArrowDown" });
    fireEvent.keyDown(trigger, { key: "ArrowRight" });
    expect(screen.getByRole("menu", { name: "Multimedia actions" })).toBeInTheDocument();

    fireEvent.keyDown(trigger, { key: "ArrowDown" });
    fireEvent.keyDown(trigger, { key: "Enter" });
    expect(onChange).not.toHaveBeenCalled();

    fireEvent.keyDown(trigger, { key: "ArrowUp" });
    fireEvent.keyDown(trigger, { key: "Enter" });
    expect(onChange).toHaveBeenCalledWith("play_pause");
    expect(screen.queryByRole("menu", { name: "Button action categories" })).not.toBeInTheDocument();
    expect(trigger).toHaveFocus();

    fireEvent.keyDown(trigger, { key: "Enter" });
    fireEvent.keyDown(trigger, { key: "ArrowRight" });
    fireEvent.keyDown(trigger, { key: "ArrowLeft" });
    expect(screen.queryByRole("menu", { name: "Basic actions" })).not.toBeInTheDocument();
    fireEvent.keyDown(trigger, { key: "Escape" });
    expect(screen.queryByRole("menu", { name: "Button action categories" })).not.toBeInTheDocument();
    expect(trigger).toHaveFocus();
  });

  it("closes on outside click and does not select disabled actions", () => {
    const onChange = vi.fn();
    render(<><GnomeSelect aria-label="Button action" value="left" options={options} onChange={onChange} /><button>Outside</button></>);

    fireEvent.click(screen.getByRole("button", { name: "Button action" }));
    fireEvent.mouseEnter(screen.getByRole("menuitem", { name: "Multimedia" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Mute" }));
    expect(onChange).not.toHaveBeenCalled();
    expect(screen.getByRole("menu", { name: "Multimedia actions" })).toBeInTheDocument();

    fireEvent.mouseDown(screen.getByRole("button", { name: "Outside" }));
    expect(screen.queryByRole("menu", { name: "Button action categories" })).not.toBeInTheDocument();
  });
});
