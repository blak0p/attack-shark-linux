import "@testing-library/jest-dom/vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { LightingEffect } from "../../desktop-contract";
import { LightingEffectSelect } from "./LightingEffectSelect";

afterEach(cleanup);

const effects: LightingEffect[] = [
  { Mode: 0x00, Label: "Off", DefaultTemplateID: "off", SpeedVariants: [], ColorTemplates: [] },
  { Mode: 0x10, Label: "Fixed", DefaultTemplateID: "fixed-green", SpeedVariants: [], ColorTemplates: [] },
  { Mode: 0x20, Label: "Breathing", DefaultTemplateID: "breathing-green", SpeedVariants: [], ColorTemplates: [] },
];

describe("LightingEffectSelect", () => {
  it("renders an opaque custom listbox instead of a native select", () => {
    render(<LightingEffectSelect effects={effects} value={0x10} onChange={vi.fn()} />);

    const combobox = screen.getByRole("combobox", { name: "Lighting effect" });
    expect(combobox).toBeInTheDocument();
    expect(combobox.tagName).not.toBe("SELECT");
    fireEvent.click(combobox);

    const listbox = screen.getByRole("listbox", { name: "Lighting effect" });
    expect(listbox).toBeInTheDocument();
    expect(listbox).toHaveAttribute("aria-label", "Lighting effect");
    expect(screen.getAllByRole("option")).toHaveLength(3);
    expect(screen.getByRole("option", { name: "Fixed" })).toHaveAttribute("aria-selected", "true");
    expect(listbox.querySelector("select")).not.toBeInTheDocument();
  });

  it("stages the selected effect when an option is clicked", () => {
    const onChange = vi.fn();
    render(<LightingEffectSelect effects={effects} value={0x00} onChange={onChange} />);

    fireEvent.click(screen.getByRole("combobox", { name: "Lighting effect" }));
    fireEvent.click(screen.getByRole("option", { name: "Breathing" }));

    expect(onChange).toHaveBeenCalledWith(effects[2]);
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
  });

  it("supports keyboard navigation with Enter and closes with Escape", () => {
    const onChange = vi.fn();
    render(<LightingEffectSelect effects={effects} value={0x00} onChange={onChange} />);

    const combobox = screen.getByRole("combobox", { name: "Lighting effect" });
    fireEvent.keyDown(combobox, { key: "ArrowDown" });
    expect(screen.getByRole("listbox")).toBeInTheDocument();
    fireEvent.keyDown(combobox, { key: "ArrowDown" });
    fireEvent.keyDown(combobox, { key: "Enter" });

    expect(onChange).toHaveBeenCalledWith(effects[1]);
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();

    fireEvent.keyDown(combobox, { key: "ArrowDown" });
    fireEvent.keyDown(combobox, { key: "Escape" });
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
  });

  it("does not open or stage when disabled", () => {
    const onChange = vi.fn();
    render(<LightingEffectSelect effects={effects} value={0x00} onChange={onChange} disabled />);

    const combobox = screen.getByRole("combobox", { name: "Lighting effect" });
    expect(combobox).toBeDisabled();
    fireEvent.click(combobox);
    fireEvent.keyDown(combobox, { key: "ArrowDown" });

    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
    expect(onChange).not.toHaveBeenCalled();
  });

  it("closes when focus moves outside the selector", () => {
    render(<LightingEffectSelect effects={effects} value={0x00} onChange={vi.fn()} />);

    fireEvent.click(screen.getByRole("combobox", { name: "Lighting effect" }));
    expect(screen.getByRole("listbox")).toBeInTheDocument();
    fireEvent.mouseDown(document.body);

    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
  });
});
