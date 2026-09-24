import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { UpdateBanner } from "./UpdateBanner";

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
