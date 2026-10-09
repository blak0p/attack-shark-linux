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

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe("GnomeSelect", () => {
  it.each(["pointer", "keyboard"])("selects a direct category entry by %s without a submenu", (mode) => {
    const onChange = vi.fn();
    render(<GnomeSelect aria-label="Button action" value="left" options={[...options, { value: "macro", label: "Macro", direct: true }]} onChange={onChange} />);
    const trigger = screen.getByRole("button", { name: "Button action" });
    trigger.focus();
    fireEvent.click(trigger);
    const macro = screen.getByRole("menuitem", { name: "Macro" });
    expect(macro).not.toHaveAttribute("aria-haspopup");
    fireEvent.mouseEnter(macro);
    expect(screen.queryByRole("menu", { name: "Macro actions" })).not.toBeInTheDocument();
    if (mode === "pointer") fireEvent.click(macro);
    else fireEvent.keyDown(trigger, { key: "Enter" });
    expect(onChange).toHaveBeenCalledExactlyOnceWith("macro");
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
    expect(trigger).toHaveFocus();
  });

  it("does not activate a disabled direct entry", () => {
    const onChange = vi.fn();
    render(<GnomeSelect aria-label="Button action" value="left" options={[...options, { value: "macro", label: "Macro", direct: true, disabled: true }]} onChange={onChange} />);
    fireEvent.click(screen.getByRole("button", { name: "Button action" }));
    const macro = screen.getByRole("menuitem", { name: "Macro" });
    expect(macro).toHaveAttribute("aria-disabled", "true");
    fireEvent.click(macro);
    expect(onChange).not.toHaveBeenCalled();
  });
  it.each([200, 900])("keeps a %ipx submenu inside the vertical viewport near Button 7", (height) => {
    vi.stubGlobal("innerHeight", 600);
    vi.stubGlobal("innerWidth", 620);
    vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockImplementation(function () {
      const top = this.classList.contains("gnome-select-submenu") ? 510 : 550;
      const measuredHeight = this.classList.contains("gnome-select-submenu")
        && this.style.maxHeight ? Math.min(height, 220) : height;
      return { x: 460, y: top, left: 460, right: 620, top, bottom: top + measuredHeight,
        width: 160, height: measuredHeight, toJSON: () => ({}) };
    });
    const onChange = vi.fn();
    render(<GnomeSelect aria-label="Button 7 action" value="left" options={options} onChange={onChange} />);
    const trigger = screen.getByRole("button", { name: "Button 7 action" });
    trigger.focus();
    fireEvent.keyDown(trigger, { key: "Enter" });
    fireEvent.keyDown(trigger, { key: "ArrowRight" });
    const submenu = screen.getByRole("menu", { name: "Basic actions" });
    // Two categories anchor the popup at 510px; the taller action menu must move up.
    const top = 510 + Number.parseFloat(submenu.style.top || "0");
    const visibleHeight = submenu.getBoundingClientRect().height;
    expect(submenu.style.maxHeight).toContain("220px");
    expect(submenu.style.maxHeight).toContain("100vh");
    expect(top).toBeGreaterThanOrEqual(8);
    expect(top + visibleHeight).toBeLessThanOrEqual(592);
    expect(submenu).toHaveClass("flipped");
    vi.stubGlobal("innerHeight", 500);
    fireEvent.resize(window);
    const popup = screen.getByRole("menu", { name: "Button 7 action categories" });
    const resizedTop = Number.parseFloat(popup.style.top) + Number.parseFloat(submenu.style.top);
    expect(resizedTop).toBeGreaterThanOrEqual(8);
    expect(resizedTop + visibleHeight).toBeLessThanOrEqual(492);
    fireEvent.keyDown(trigger, { key: "ArrowDown" });
    fireEvent.keyDown(trigger, { key: "Enter" });
    expect(onChange).toHaveBeenCalledWith("fire");
    expect(trigger).toHaveFocus();
  });

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
