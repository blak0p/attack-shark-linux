import "@testing-library/jest-dom/vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import type { WorkspaceModel } from "../desktop-contract";
import { WorkspaceFeedback } from "./WorkspaceFeedback";
import { settingsResult } from "./OperationFeedback";

const model: WorkspaceModel = { ready: true, notice: "", resetConfirmation: false, automaticApplyBusy: false, updateApplying: false, updateError: "" };
const library = { loading: false, reading: false, busy: false, error: "", notice: "" };
const retries = { dpi: vi.fn(), polling: vi.fn(), debounce: vi.fn(), sleep: vi.fn() };
const props = { model, library, retries, feedbackFor: (code: string) => code };
afterEach(cleanup);

it.each(["automaticApplyBusy", "remapBusy", "updateApplying"] as const)("uses the shared animation for %s", (field) => {
  render(<WorkspaceFeedback {...props} model={{ ...model, [field]: true }} />);
  expect(screen.getByRole("status", { name: "Operation status" })).toHaveTextContent("Working…");
  expect(document.querySelectorAll(".pending-spinner")).toHaveLength(1);
});
it.each(["loading", "reading", "busy"] as const)("uses the shared animation for library %s", (field) => {
  render(<WorkspaceFeedback {...props} library={{ ...library, [field]: true }} />);
  expect(screen.getByRole("status")).toHaveTextContent("Working…");
  expect(document.querySelectorAll(".pending-spinner")).toHaveLength(1);
});
it("does not announce Applied when authoritative snapshots first finish loading", () => {
  const view = render(<WorkspaceFeedback {...props} />);
  view.rerender(<WorkspaceFeedback {...props} model={{ ...model, polling: {
    Desired: 500, Applied: 500, Factory: 1000, Revision: 0, Firmware: "success", Persistence: "success", Error: { Code: "" },
  } }} />);
  expect(screen.queryByRole("status")).not.toBeInTheDocument();
});
it.each(["snapshot", "polling", "debounce", "normalSleep", "lighting"] as const)("shows native queued progress for %s", (field) => {
  render(<WorkspaceFeedback {...props} model={{ ...model, [field]: { Firmware: "pending", Error: { Code: "" } } } as WorkspaceModel} />);
  expect(screen.getByRole("status")).toHaveTextContent("Working…");
});
it("retains reset failures without turning them into a successful device apply", () => {
  render(<WorkspaceFeedback {...props} model={{ ...model, notice: "Factory reset failed: ack_timeout." }} />);
  expect(screen.getByRole("alert")).toHaveTextContent("ack_timeout");
  expect(screen.queryByRole("status")).not.toBeInTheDocument();
});
it("maps firmware and local persistence errors separately with a local retry", () => {
  const retry = vi.fn();
  expect(settingsResult("dpi", "DPI", { Revision: 1, Firmware: "success", Persistence: "failed", Error: { Code: "disk_full" }, RetryAvailable: true }, code => code, retry)).toMatchObject({ error: "Applied to mouse; local save failed.", retry, saveFailed: true });
  expect(settingsResult("dpi", "DPI", { Revision: 1, Firmware: "failed", Error: { Code: "disconnect" } }, code => code)).toMatchObject({ error: "disconnect", retry: undefined });
});


it("does not call an ordinary remap a macro send because of residual macro progress", () => {
  const config = { Buttons: Array.from({ length: 7 }, (_, index) => ({ Button: index + 1, Action: "left" as const, PreservedDefault: "" as const })) };
  const remap = { Pending: config, Applied: config, Factory: config, Actions: ["left" as const], Revision: 1,
    Firmware: "failed", Persistence: "", RetryAvailable: false, Error: { Code: "apply_failed" },
    MacroPending: null, MacroApplied: null, MacroDrafts: {}, MacroAppliedDrafts: {}, MacroProgress: { Assignment: 2 as const, Upload: 1 as const } };
  const view = render(<WorkspaceFeedback {...props} model={{ ...model, remap }} />);
  view.rerender(<WorkspaceFeedback {...props} model={{ ...model, remap: { ...remap, Revision: 2, Firmware: "success", Persistence: "success", Error: { Code: "" } } }} />);
  expect(screen.getByRole("status", { name: "Operation status" })).toHaveTextContent("Applied");
  expect(screen.getByRole("status")).not.toHaveTextContent("Sent");
});
