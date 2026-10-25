import "@testing-library/jest-dom/vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { OperationFeedback, type FeedbackResult } from "./OperationFeedback";

afterEach(() => { cleanup(); vi.useRealTimers(); });
const result = (version: string, extra: Partial<FeedbackResult> = {}): FeedbackResult => ({ id: "dpi", label: "DPI", version, success: "Applied", ...extra });

it("does not announce successful snapshots on startup, refresh, or device switch", () => {
  const view = render(<OperationFeedback scope="a" results={[result("1")]} />);
  expect(screen.queryByRole("status")).not.toBeInTheDocument();
  view.rerender(<OperationFeedback scope="a" results={[result("1")]} />);
  view.rerender(<OperationFeedback scope="b" results={[result("2")]} />);
  expect(screen.queryByRole("status")).not.toBeInTheDocument();
});

it("uses one pending spinner and a transient Applied confirmation in the same region", () => {
  vi.useFakeTimers();
  const view = render(<OperationFeedback scope="a" results={[result("1")]} />);
  view.rerender(<OperationFeedback scope="a" busy results={[result("1")]} />);
  const progress = screen.getByRole("status", { name: "Operation status" });
  expect(progress).toHaveTextContent("Working…");
  expect(progress.querySelectorAll(".pending-spinner")).toHaveLength(1);
  view.rerender(<OperationFeedback scope="a" results={[result("2")]} />);
  expect(screen.getByRole("status", { name: "Operation status" })).toHaveTextContent("Applied");
  expect(screen.getByRole("status").querySelector(".pending-spinner")).toBeNull();
  act(() => vi.advanceTimersByTime(3000));
  expect(screen.queryByRole("status")).not.toBeInTheDocument();
});

it("keeps the latest repeated confirmation alive and does not expire it during pending work", () => {
  vi.useFakeTimers();
  const view = render(<OperationFeedback scope="a" results={[result("1")]} />);
  view.rerender(<OperationFeedback scope="a" results={[result("2")]} />);
  act(() => vi.advanceTimersByTime(2000));
  view.rerender(<OperationFeedback scope="a" busy results={[result("3")]} />);
  act(() => vi.advanceTimersByTime(5000));
  expect(screen.getByRole("status")).toHaveTextContent("Working…");
  view.rerender(<OperationFeedback scope="a" results={[result("3")]} />);
  act(() => vi.advanceTimersByTime(2999));
  expect(screen.getByRole("status")).toHaveTextContent("Applied");
  act(() => vi.advanceTimersByTime(1));
  expect(screen.queryByRole("status")).not.toBeInTheDocument();
});

it("distinguishes local Saved from device Applied without replaying the other source", () => {
  const dpi = result("1");
  const library = result("", { id: "library", label: "Macros", success: undefined });
  const view = render(<OperationFeedback scope="a" results={[dpi, library]} />);
  view.rerender(<OperationFeedback scope="a" results={[dpi, { ...library, version: "saved", success: "Saved" }]} />);
  expect(screen.getByRole("status")).toHaveTextContent("Saved");
  expect(screen.getAllByRole("status")).toHaveLength(1);
  view.rerender(<OperationFeedback scope="a" results={[result("2"), { ...library, version: "saved", success: "Saved" }]} />);
  expect(screen.getByRole("status")).toHaveTextContent("Applied");
});

it("shows errors and retries initially then dismisses them without replay on unchanged rerender", () => {
  vi.useFakeTimers();
  const retry = vi.fn();
  const view = render(<OperationFeedback scope="a" results={[result("1")]} />);
  view.rerender(<OperationFeedback scope="a" results={[
    result("2", { error: "Applied to mouse; local save failed.", retry }),
    result("3", { id: "lighting", label: "Lighting", error: "Reconnect the mouse." }),
  ]} />);
  act(() => vi.advanceTimersByTime(7999));
  expect(screen.queryByRole("status")).not.toBeInTheDocument();
  expect(screen.getAllByRole("alert")).toHaveLength(2);
  const error = screen.getAllByRole("alert")[0];
  expect(error).toHaveTextContent("local save failed");
  fireEvent.click(within(error).getByRole("button", { name: "Retry" }));
  expect(retry).toHaveBeenCalledTimes(1);
  expect(screen.getAllByRole("alert")[1]).toHaveTextContent("Reconnect the mouse.");
  act(() => vi.advanceTimersByTime(1));
  expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  view.rerender(<OperationFeedback scope="a" results={[result('2', { error: 'Applied to mouse; local save failed.', retry }), result('3', { id: 'lighting', label: 'Lighting', error: 'Reconnect the mouse.' })]} />);
  expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  view.rerender(<OperationFeedback scope="a" results={[result('4', { error: 'Reconnect the mouse.' })]} />);
  expect(screen.getByRole('alert')).toHaveTextContent('Reconnect the mouse.');
  view.unmount();
  expect(vi.getTimerCount()).toBe(0);
});


it("confirms a local save while an independent disconnected-device error remains visible", () => {
  vi.useFakeTimers();
  const offline = result("offline", { error: "Reconnect the mouse.", success: undefined });
  const library = result("", { id: "library", label: "Macros", success: undefined });
  const view = render(<OperationFeedback scope="offline" results={[offline, library]} />);
  view.rerender(<OperationFeedback scope="offline" results={[offline, { ...library, version: "saved", success: "Saved" }]} />);
  expect(screen.getByRole("status", { name: "Operation status" })).toHaveTextContent("Saved");
  expect(screen.getByRole("alert")).toHaveTextContent("Reconnect the mouse.");
  act(() => vi.advanceTimersByTime(3000));
  expect(screen.queryByRole("status")).not.toBeInTheDocument();
  expect(screen.getByRole("alert")).toHaveTextContent("Reconnect the mouse.");
});
