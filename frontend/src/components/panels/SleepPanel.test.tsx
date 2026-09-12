import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { SleepPanel } from "./SleepPanel";

it("offers half-minute normal-sleep steps", () => {
  const onStage = vi.fn();
  render(<SleepPanel ready snapshot={{ Pending: 30.5, Applied: 30.5, Persisted: 30.5, Revision: 1, Firmware: "success", Persistence: "success", RetryAvailable: false, Error: { Code: "" } }} onStage={onStage} onRetry={vi.fn()} />);

  const slider = screen.getByRole("slider", { name: "Normal sleep duration" });
  expect(slider).toHaveAttribute("min", "0.5");
  expect(slider).toHaveAttribute("max", "60");
  expect(slider).toHaveAttribute("step", "0.5");
  fireEvent.change(slider, { target: { value: "15.5" } });
  expect(onStage).toHaveBeenCalledWith(15.5);
});
