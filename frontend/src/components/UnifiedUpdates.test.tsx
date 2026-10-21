import "@testing-library/jest-dom/vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { UnifiedUpdates } from "./UnifiedUpdates";
import type { UnifiedUpdateInfo } from "../desktop-contract";

afterEach(cleanup);
const info: UnifiedUpdateInfo = { Version: "2.0.0", AppAvailable: true, AppInstalled: true,
  AppError: "", PolicyState: "outdated", PolicyInstalled: true, PolicyError: "reload failed", RelaunchError: "launch failed" };
it("offers explicit relaunch without hiding a completed policy write or independent failure", () => {
  const onApply = vi.fn();
  render(<UnifiedUpdates info={info} checking={false} applying={false} error="" onCheck={vi.fn()} onApply={onApply} />);
  expect(screen.getByText("USB policy written; reload or activation failed.")).toBeInTheDocument();
  expect(screen.getByText("AppImage replaced; relaunch failed.")).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Apply updates" })).not.toBeInTheDocument();
  expect(onApply).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole("button", { name: "Relaunch app" }));
  expect(onApply).toHaveBeenCalledTimes(1);
});
it("preserves known partial outcomes when the request itself rejects", () => {
  render(<UnifiedUpdates info={info} checking={false} applying={false} error="request canceled" onCheck={vi.fn()} onApply={vi.fn()} />);
  expect(screen.getByText("Update request failed: request canceled")).toBeInTheDocument();
  expect(screen.getByText("USB permissions: reload failed")).toBeInTheDocument();
  expect(screen.getByText("Relaunch: launch failed")).toBeInTheDocument();
  expect(screen.queryByText("USB permissions are current.")).not.toBeInTheDocument();
});
