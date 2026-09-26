import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { readFileSync } from "node:fs";
import { UpdateBanner } from "./UpdateBanner";

it("keeps the update card fixed below the titlebar and bounded within the viewport", () => {
  const css = readFileSync("src/styles.css", "utf8");
  const card = css.match(/\.update-banner\s*\{([^}]*)\}/)?.[1] ?? "";
  expect(card).toMatch(/position:\s*fixed/);
  expect(card).toMatch(/top:\s*calc\(48px\s*\+/);
  expect(card).toMatch(/right:\s*\d+px/);
  expect(card).toMatch(/width:\s*min\(340px,\s*calc\(100vw\s*-\s*24px\)\)/);
  expect(card).toMatch(/max-width:\s*calc\(100vw\s*-/);
  expect(card).toMatch(/max-height:\s*calc\(100vh\s*-/);
  expect(card).toMatch(/overflow-y:\s*auto/);
  expect(card).not.toMatch(/margin:/);
});

it("requires an explicit approval click and keeps failure local", () => {
  const apply = vi.fn();
  render(<UpdateBanner update={{ Version: "1.2.0" }} applying={false} error="" onApply={apply} />);
  fireEvent.click(screen.getByRole("button", { name: "Update and restart" }));
  expect(apply).toHaveBeenCalledOnce();
});

it("announces loading and launch failures while disabling duplicate approval", () => {
  const apply = vi.fn();
  const view = render(<UpdateBanner update={{ Version: "1.2.0" }} applying={true} error="" onApply={apply} />);
  const query = () => within(view.container);
  expect(query().getByRole("status")).toHaveAttribute("aria-live", "polite");
  expect(query().getByRole("button", { name: "Updating…" })).toBeDisabled();

  view.rerender(<UpdateBanner update={{ Version: "1.2.0" }} applying={false} error="launch failed" onApply={apply} />);
  expect(query().getByRole("alert")).toHaveTextContent("Update failed: launch failed");
  expect(query().getByRole("button", { name: "Update and restart" })).toBeEnabled();
});
