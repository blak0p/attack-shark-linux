import "@testing-library/jest-dom/vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { WorkspaceShell } from "./WorkspaceShell";
import { WorkspaceView } from "./WorkspaceView";

afterEach(cleanup);

describe("WorkspaceShell", () => {
  it("activates a named view, hides the other mounted views, and focuses its heading", () => {
    render(<WorkspaceShell><WorkspaceView id="performance" title="Performance"><button>Performance control</button></WorkspaceView><WorkspaceView id="lighting" title="Lighting"><button>Lighting control</button></WorkspaceView><WorkspaceView id="controls" title="Controls"><button>Controls control</button></WorkspaceView><WorkspaceView id="device" title="Device"><button>Device control</button></WorkspaceView></WorkspaceShell>);

    const lighting = screen.getByRole("link", { name: "Lighting" });
    expect(lighting).toHaveAttribute("aria-controls", "workspace-view-lighting");
    fireEvent.click(lighting);

    expect(lighting).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("region", { name: "Lighting" })).toHaveAttribute("data-active", "true");
    expect(document.getElementById("workspace-view-performance")).toHaveAttribute("data-active", "false");
    expect(screen.getByRole("heading", { name: "Lighting" })).toHaveFocus();
  });

  it("keeps native controls queryable when a different view is active", () => {
    render(<WorkspaceShell><WorkspaceView id="performance" title="Performance"><input type="range" aria-label="DPI" /></WorkspaceView><WorkspaceView id="lighting" title="Lighting"><select aria-label="Lighting effect"><option>Fixed</option></select></WorkspaceView><WorkspaceView id="controls" title="Controls"><input type="checkbox" aria-label="Angle snap" /></WorkspaceView><WorkspaceView id="device" title="Device"><button>Reset to factory</button></WorkspaceView></WorkspaceShell>);

    fireEvent.click(screen.getByRole("link", { name: "Device" }));
    expect(screen.getByRole("slider", { name: "DPI", hidden: true })).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "Lighting effect", hidden: true })).toBeInTheDocument();
    expect(screen.getByRole("checkbox", { name: "Angle snap", hidden: true })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Reset to factory" })).toBeInTheDocument();
  });

  it("keeps every section mounted while navigation changes the current section", () => {
    render(<WorkspaceShell><WorkspaceView id="performance" title="Performance"><button>DPI control</button><button>Polling control</button></WorkspaceView><WorkspaceView id="lighting" title="Lighting"><button>Lighting control</button></WorkspaceView><WorkspaceView id="controls" title="Controls"><button>Feature control</button></WorkspaceView><WorkspaceView id="device" title="Device"><button>Device control</button><button>Reset control</button></WorkspaceView></WorkspaceShell>);

    fireEvent.click(screen.getByRole("link", { name: "Lighting" }));
    expect(screen.getByRole("link", { name: "Lighting" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("button", { name: "DPI control" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Reset control" })).toBeInTheDocument();
  });

  it("offers keyboard-operable rails that retain their controlled content when collapsed", () => {
    render(<WorkspaceShell><WorkspaceView id="performance" title="Performance"><button>Primary control</button></WorkspaceView></WorkspaceShell>);
    const toggle = screen.getByRole("button", { name: "Toggle navigation rail" });
    toggle.focus();
    fireEvent.keyDown(toggle, { key: "Enter" });

    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(toggle).toHaveAttribute("aria-controls", "workspace-navigation-rail");
    expect(screen.getByRole("link", { name: "Performance" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Primary control" })).toBeInTheDocument();
  });

  it("keeps only the navigation rail and never renders a utility tools rail", () => {
    render(<WorkspaceShell><WorkspaceView id="performance" title="Performance"><button>Primary control</button></WorkspaceView></WorkspaceShell>);

    expect(screen.getByRole("button", { name: "Toggle navigation rail" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Toggle utility rail/i })).not.toBeInTheDocument();
    expect(screen.queryByText(/Workspace tools/i)).not.toBeInTheDocument();
  });

  it("preserves the same reachable controls at supported desktop sizes with reduced motion", () => {
    Object.defineProperty(window, "matchMedia", { configurable: true, value: () => ({ matches: true, addEventListener: () => {}, removeEventListener: () => {} }) });
    window.innerWidth = 640;
    window.innerHeight = 720;
    render(<WorkspaceShell><WorkspaceView id="performance" title="Performance"><button>Reachable control</button></WorkspaceView></WorkspaceShell>);

    expect(screen.getByRole("main")).toHaveAttribute("data-motion", "reduced");
    expect(screen.getByRole("button", { name: "Reachable control" })).toBeInTheDocument();
  });
});
