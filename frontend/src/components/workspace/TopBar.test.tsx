import "@testing-library/jest-dom/vitest";
import { readFileSync } from "node:fs";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { TopBar } from "./TopBar";

const { minimise, close } = vi.hoisted(() => ({ minimise: vi.fn().mockResolvedValue(undefined), close: vi.fn().mockResolvedValue(undefined) }));
vi.mock("@wailsio/runtime", () => ({ Window: { Minimise: minimise, Close: close } }));
afterEach(() => { cleanup(); vi.clearAllMocks(); });

describe("TopBar window controls", () => {
  it("reserves fixed control space while allowing long children to scroll", () => {
    const css = readFileSync("src/styles/layout.css", "utf8");
    expect(css).toMatch(/\.top-bar \.brand\s*\{[^}]*min-width:\s*0;[^}]*overflow:\s*hidden;/s);
    expect(css).toMatch(/\.top-bar-actions\s*\{[^}]*min-width:\s*0;/s);
    expect(css).toMatch(/\.top-bar-children\s*\{[^}]*min-width:\s*0;[^}]*overflow-x:\s*auto;/s);
    expect(css).toMatch(/\.window-controls\s*\{[^}]*flex:\s*none;/s);
  });

  it("exposes keyboard-accessible labelled controls and retains children", () => {
    render(<TopBar><span>Connection status</span></TopBar>);
    expect(screen.getByText("Connection status")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Minimise window" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Close window" })).toBeInTheDocument();
  });

  it("routes each click only to its native window action", () => {
    render(<TopBar />);
    fireEvent.click(screen.getByRole("button", { name: "Minimise window" }));
    expect(minimise).toHaveBeenCalledTimes(1);
    expect(close).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Close window" }));
    expect(close).toHaveBeenCalledTimes(1);
    expect(minimise).toHaveBeenCalledTimes(1);
  });
});
