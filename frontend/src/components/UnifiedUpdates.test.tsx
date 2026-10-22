import "@testing-library/jest-dom/vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { UnifiedUpdates } from "./UnifiedUpdates";
import type { UnifiedUpdateInfo } from "../desktop-contract";

afterEach(() => { cleanup(); vi.useRealTimers(); });
const completed: UnifiedUpdateInfo = { Version: "2.0.0", AppAvailable: false, AppInstalled: false,
  AppError: "", PolicyState: "current", PolicyInstalled: true, PolicyError: "", RelaunchError: "" };
const props = { checking: false, applying: false, error: "", onCheck: vi.fn(), onApply: vi.fn() };
const advance = (ms: number) => act(() => { vi.advanceTimersByTime(ms); });
it("dismisses confirmed USB-only success at 3000ms without extending or resurrecting on rechecks", () => {
  vi.useFakeTimers();
  const view = render(<UnifiedUpdates {...props} info={completed} />);
  expect(screen.getByText("USB permissions updated.")).toBeInTheDocument();
  advance(2000);
  view.rerender(<UnifiedUpdates {...props} info={{ ...completed }} />);
  advance(999);
  expect(screen.getByRole("region", { name: "Updates" })).toBeInTheDocument();
  advance(1);
  expect(screen.queryByRole("region", { name: "Updates" })).not.toBeInTheDocument();
  view.rerender(<UnifiedUpdates {...props} info={{ ...completed }} />);
  expect(screen.queryByRole("region", { name: "Updates" })).not.toBeInTheDocument();
  view.rerender(<UnifiedUpdates {...props} info={{ ...completed, PolicyInstalled: false, PolicyState: "outdated" }} />);
  fireEvent.click(screen.getByRole("button", { name: "Apply updates" }));
  expect(props.onApply).toHaveBeenCalledTimes(1);
  view.rerender(<UnifiedUpdates {...props} info={completed} />);
  expect(screen.getByText("USB permissions updated.")).toBeInTheDocument();
  advance(3000);
  expect(screen.queryByRole("region", { name: "Updates" })).not.toBeInTheDocument();
});
it("waits for apply and check completion and cancels the timer on unmount", () => {
  vi.useFakeTimers();
  const view = render(<UnifiedUpdates {...props} info={completed} applying />);
  advance(4000);
  expect(screen.queryByText("USB permissions updated.")).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Check for updates" })).toBeDisabled();
  view.rerender(<UnifiedUpdates {...props} info={completed} checking />);
  advance(4000);
  expect(screen.getByRole("region", { name: "Updates" })).toBeInTheDocument();
  view.rerender(<UnifiedUpdates {...props} info={completed} />);
  advance(2999);
  expect(screen.getByText("USB permissions updated.")).toBeInTheDocument();
  view.unmount();
  expect(vi.getTimerCount()).toBe(0);
  advance(1);
});
it.each([
  { PolicyError: "authorization denied" }, { PolicyError: "reload failed" },
  { PolicyError: "trigger failed" }, { AppError: "download failed" },
  { RelaunchError: "relaunch failed" }, { AppAvailable: true },
  { AppInstalled: true }, { AppInstalled: true, PolicyError: "reload failed" },
])("retains unfinished or failed outcomes past three seconds: %j", (outcome) => {
  vi.useFakeTimers();
  render(<UnifiedUpdates {...props} info={{ ...completed, ...outcome }} />);
  advance(4000);
  expect(screen.getByRole("region", { name: "Updates" })).toBeInTheDocument();
});
it("retains request failures and keeps idle current startup quiet", () => {
  vi.useFakeTimers();
  const view = render(<UnifiedUpdates {...props} info={completed} error="request canceled" />);
  advance(4000);
  expect(screen.getByRole("alert")).toHaveTextContent("Update request failed: request canceled");
  view.rerender(<UnifiedUpdates {...props} info={{ ...completed, PolicyInstalled: false }} />);
  expect(screen.queryByRole("region", { name: "Updates" })).not.toBeInTheDocument();
});
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
