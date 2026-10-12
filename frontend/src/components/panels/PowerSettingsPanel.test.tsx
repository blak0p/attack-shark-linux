import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { PowerSettingsPanel } from "./PowerSettingsPanel";

it("offers supported key-response-time steps", () => {
  const onChange = vi.fn();
  render(<PowerSettingsPanel value={8} onChange={onChange} />);

  const slider = screen.getByRole("slider", { name: "Key response time" });
  expect(slider).toHaveAttribute("min", "2");
  expect(slider).toHaveAttribute("max", "25");
  expect(slider).toHaveAttribute("step", "1");
  expect(screen.getByRole("status")).toHaveTextContent("Applied 8 ms");
  for (const value of [2, 3, 12, 25]) {
    fireEvent.change(slider, { target: { value: String(value) } });
    expect(onChange).toHaveBeenLastCalledWith(value);
  }
});
