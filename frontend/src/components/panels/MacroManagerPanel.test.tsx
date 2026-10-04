import "@testing-library/jest-dom/vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { MacroManagerPanel } from "./MacroManagerPanel";
import type { Macro, MacroLibraryService } from "../../desktop-contract";

afterEach(cleanup);

it("edits, adds, reorders and removes local events before saving the same ID and name", async () => {
  const service = serviceFor();
  render(<MacroManagerPanel service={service} />);
  fireEvent.click(await screen.findByRole("button", { name: "Clicks · 2 events" }));
  await screen.findByLabelText("Event 1 button");
  expect(screen.getByRole("button", { name: "Move event 1 up" })).toBeDisabled();
  expect(screen.getByRole("button", { name: "Move event 2 down" })).toBeDisabled();
  fireEvent.change(screen.getByLabelText("Event 1 button"), { target: { value: "mouse_right" } });
  fireEvent.change(screen.getByLabelText("Event 1 action"), { target: { value: "up" } });
  fireEvent.change(screen.getByLabelText("Event 1 delay (ms)"), { target: { value: "56" } });
  fireEvent.click(screen.getByRole("button", { name: "Add event" }));
  expect(screen.getByLabelText("Event 3 delay (ms)")).toHaveValue("0");
  fireEvent.click(screen.getByRole("button", { name: "Move event 3 up" }));
  fireEvent.click(screen.getByRole("button", { name: "Move event 2 up" }));
  fireEvent.click(screen.getByRole("button", { name: "Move event 1 down" }));
  fireEvent.click(screen.getByRole("button", { name: "Remove event 3" }));
  expect(service.UpdateMacro).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole("button", { name: "Save to library" }));
  await waitFor(() => expect(service.UpdateMacro).toHaveBeenCalledWith("one", "Clicks", [
    { type: "mouse_right", action: "up", delay_ms: 56 },
    { type: "mouse_left", action: "down", delay_ms: 0 },
  ]));
});

it("rejects invalid delays locally and retains edited events for retry after a save failure", async () => {
  const service = serviceFor({ UpdateMacro: vi.fn().mockRejectedValueOnce(new Error("Disk full")).mockImplementation(async (id, name, events) => ({ id, name, events })) });
  render(<MacroManagerPanel service={service} />);
  fireEvent.click(await screen.findByRole("button", { name: "Clicks · 2 events" }));
  const delay = await screen.findByLabelText("Event 1 delay (ms)");
  for (const value of ["", "-1", "1.5", "abc", "Infinity"]) {
    fireEvent.change(delay, { target: { value } });
    expect(screen.getByRole("alert")).toHaveTextContent("Event 1 delay must be a nonnegative whole number of milliseconds.");
    expect(screen.getByRole("button", { name: "Save to library" })).toBeDisabled();
  }
  expect(service.UpdateMacro).not.toHaveBeenCalled();
  fireEvent.change(delay, { target: { value: "0" } });
  expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Save to library" }));
  expect(await screen.findByRole("alert")).toHaveTextContent("Disk full");
  expect(delay).toHaveValue("0");
  fireEvent.click(screen.getByRole("button", { name: "Save to library" }));
  await screen.findByText("Saved to library. No device changes made.");
  expect(service.UpdateMacro).toHaveBeenCalledTimes(2);
  fireEvent.click(screen.getByRole("button", { name: "Remove event 2" }));
  fireEvent.click(screen.getByRole("button", { name: "Remove event 1" }));
  expect(screen.queryByText("Saved to library. No device changes made.")).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Save to library" }));
  await waitFor(() => expect(service.UpdateMacro).toHaveBeenLastCalledWith("one", "Clicks", []));
});
it("creates edited events locally and locks event controls during save and delete confirmation", async () => {
  let resolve!: (value: Macro) => void;
  const pending = new Promise<Macro>((done) => { resolve = done; });
  const service = serviceFor({ CreateMacro: vi.fn().mockReturnValue(pending) });
  render(<MacroManagerPanel service={service} />);
  await screen.findByRole("button", { name: "Clicks · 2 events" });
  fireEvent.click(screen.getByRole("button", { name: "New macro" }));
  fireEvent.change(screen.getByLabelText("Macro name"), { target: { value: "New events" } });
  fireEvent.click(screen.getByRole("button", { name: "Add event" }));
  fireEvent.change(screen.getByLabelText("Event 1 delay (ms)"), { target: { value: "1000000" } });
  fireEvent.click(screen.getByRole("button", { name: "Save to library" }));
  const events = [{ type: "mouse_left" as const, action: "down" as const, delay_ms: 1000000 }];
  expect(service.CreateMacro).toHaveBeenCalledWith("New events", events);
  expect(screen.getByRole("button", { name: "Add event" })).toBeDisabled();
  expect(screen.getByRole("button", { name: "Remove event 1" })).toBeDisabled();
  expect(screen.getByLabelText("Event 1 delay (ms)")).toBeDisabled();
  expect(screen.getByLabelText("Event 1 button")).toBeDisabled();
  expect(screen.getByLabelText("Event 1 action")).toBeDisabled();
  await act(async () => resolve({ id: "new", name: "New events", events }));
  fireEvent.click(screen.getByRole("button", { name: "Delete macro" }));
  expect(screen.getByRole("button", { name: "Add event" })).toBeDisabled();
  expect(screen.getByRole("button", { name: "Remove event 1" })).toBeDisabled();
  fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
  expect(screen.getByRole("button", { name: "Add event" })).toBeEnabled();
});

it("does not mutate loaded library events before save and resets invalid edits on selection", async () => {
  const original = macro();
  const service = serviceFor({ ReadMacro: vi.fn().mockResolvedValue(original), ListMacros: vi.fn().mockResolvedValue([original]) });
  render(<MacroManagerPanel service={service} />);
  const card = await screen.findByRole("button", { name: "Clicks · 2 events" });
  fireEvent.click(card);
  fireEvent.change(await screen.findByLabelText("Event 1 delay (ms)"), { target: { value: "-2" } });
  fireEvent.click(screen.getByRole("button", { name: "Move event 1 down" }));
  expect(screen.getByRole("alert")).toHaveTextContent("Event 2 delay");
  expect(original.events).toEqual(macro().events);
  fireEvent.click(card);
  await waitFor(() => expect(screen.queryByRole("alert")).not.toBeInTheDocument());
  expect(screen.getByLabelText("Event 1 delay (ms)")).toHaveValue("12");
});

const macro = (id = "one", name = "Clicks"): Macro => ({ id, name, events: [
  { type: "mouse_left", action: "down", delay_ms: 12 },
  { type: "mouse_right", action: "up", delay_ms: 34 },
] });
const serviceFor = (overrides: Partial<MacroLibraryService> = {}): MacroLibraryService => ({
  ListMacros: vi.fn().mockResolvedValue([macro()]), ReadMacro: vi.fn().mockImplementation(async (id) => macro(id)),
  CreateMacro: vi.fn().mockImplementation(async (name, events) => ({ id: "new", name, events })),
  UpdateMacro: vi.fn().mockImplementation(async (id, name, events) => ({ id, name, events })),
  DeleteMacro: vi.fn().mockResolvedValue(undefined), ...overrides,
});

it("views ordered local events, creates an empty draft, and saves a rename by ID", async () => {
  const service = serviceFor();
  render(<MacroManagerPanel service={service} />);
  fireEvent.click(await screen.findByRole("button", { name: "Clicks · 2 events" }));
  expect(await screen.findByText("Left mouse · down · 12 ms")).toBeInTheDocument();
  expect(screen.getByText("Right mouse · up · 34 ms")).toBeInTheDocument();
  fireEvent.change(screen.getByLabelText("Macro name"), { target: { value: "Renamed" } });
  fireEvent.click(screen.getByRole("button", { name: "Save to library" }));
  await waitFor(() => expect(service.UpdateMacro).toHaveBeenCalledWith("one", "Renamed", macro().events));
  fireEvent.click(screen.getByRole("button", { name: "New macro" }));
  fireEvent.change(screen.getByLabelText("Macro name"), { target: { value: "Empty" } });
  fireEvent.click(screen.getByRole("button", { name: "Save to library" }));
  await waitFor(() => expect(service.CreateMacro).toHaveBeenCalledWith("Empty", []));
  expect(await screen.findByRole("button", { name: "Empty · 0 events" })).toHaveAttribute("aria-pressed", "true");
});

it("requires explicit delete confirmation, supports cancel, and preserves data/draft on failures", async () => {
  const service = serviceFor({ UpdateMacro: vi.fn().mockRejectedValue(new Error("Disk full")), DeleteMacro: vi.fn().mockRejectedValue(new Error("Denied")) });
  render(<MacroManagerPanel service={service} />);
  fireEvent.click(await screen.findByRole("button", { name: "Clicks · 2 events" }));
  await screen.findByText("Left mouse · down · 12 ms");
  fireEvent.change(screen.getByLabelText("Macro name"), { target: { value: "Draft" } });
  fireEvent.click(screen.getByRole("button", { name: "Save to library" }));
  expect(await screen.findByRole("alert")).toHaveTextContent("Disk full");
  expect(screen.getByLabelText("Macro name")).toHaveValue("Draft");
  fireEvent.click(screen.getByRole("button", { name: "Delete macro" }));
  expect(service.DeleteMacro).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Delete macro" }));
  fireEvent.click(screen.getByRole("button", { name: "Confirm delete" }));
  await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("Denied"));
  expect(screen.getByLabelText("Macro name")).toHaveValue("Draft");
  service.DeleteMacro = vi.fn().mockResolvedValue(undefined);
  fireEvent.click(screen.getByRole("button", { name: "Confirm delete" }));
  expect(await screen.findByText("No macros in your library yet.")).toBeInTheDocument();
});

it("keeps duplicate names distinct and ignores stale reads", async () => {
  let resolve!: (value: Macro) => void;
  const delayed = new Promise<Macro>((done) => { resolve = done; });
  const service = serviceFor({ ListMacros: vi.fn().mockResolvedValue([macro("one"), macro("two")]), ReadMacro: vi.fn().mockImplementation((id) => id === "one" ? delayed : Promise.resolve(macro("two", "Second"))) });
  render(<MacroManagerPanel service={service} />);
  const cards = await screen.findAllByRole("button", { name: "Clicks · 2 events" });
  fireEvent.click(cards[0]); fireEvent.click(cards[1]);
  await waitFor(() => expect(screen.getByLabelText("Macro name")).toHaveValue("Second"));
  await act(async () => resolve(macro("one", "Stale")));
  expect(screen.getByLabelText("Macro name")).toHaveValue("Second");
  expect(cards[1]).toHaveAttribute("aria-pressed", "true");
});

it("preserves the previous selection and unsaved name when another read or list refresh fails", async () => {
  const service = serviceFor({
    ListMacros: vi.fn().mockResolvedValueOnce([macro(), macro("two", "Other")]).mockRejectedValue(new Error("Refresh failed")),
    ReadMacro: vi.fn().mockResolvedValueOnce(macro()).mockRejectedValue(new Error("Read failed")),
  });
  render(<MacroManagerPanel service={service} />);
  fireEvent.click(await screen.findByRole("button", { name: "Clicks · 2 events" }));
  await screen.findByText("Left mouse · down · 12 ms");
  fireEvent.change(screen.getByLabelText("Macro name"), { target: { value: "Keep draft" } });
  fireEvent.click(screen.getByRole("button", { name: "Other · 2 events" }));
  expect(await screen.findByRole("alert")).toHaveTextContent("Read failed");
  expect(screen.getByLabelText("Macro name")).toHaveValue("Keep draft");
  expect(screen.getByRole("button", { name: "Clicks · 2 events" })).toHaveAttribute("aria-pressed", "true");
  fireEvent.click(screen.getByRole("button", { name: "Retry library" }));
  await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("Refresh failed"));
  expect(screen.getByRole("button", { name: "Clicks · 2 events" })).toBeInTheDocument();
  expect(screen.getByLabelText("Macro name")).toHaveValue("Keep draft");
});

it("ignores reads from a replaced service", async () => {
  let resolve!: (value: Macro) => void;
  const pending = new Promise<Macro>((done) => { resolve = done; });
  const first = serviceFor({ ReadMacro: vi.fn().mockReturnValue(pending) });
  const second = serviceFor({ ListMacros: vi.fn().mockResolvedValue([]) });
  const view = render(<MacroManagerPanel service={first} />);
  fireEvent.click(await screen.findByRole("button", { name: "Clicks · 2 events" }));
  view.rerender(<MacroManagerPanel service={second} />);
  await screen.findByText("No macros in your library yet.");
  fireEvent.click(screen.getByRole("button", { name: "New macro" }));
  fireEvent.change(screen.getByLabelText("Macro name"), { target: { value: "Current" } });
  await act(async () => resolve(macro("one", "Old service")));
  expect(screen.getByLabelText("Macro name")).toHaveValue("Current");
});

it("shows loading/list errors, retries, and retains a failed create draft while preventing duplicate submissions", async () => {
  let reject!: (error: Error) => void;
  const pending = new Promise<Macro>((_, fail) => { reject = fail; });
  const service = serviceFor({ ListMacros: vi.fn().mockRejectedValueOnce(new Error("Unavailable")).mockResolvedValue([]), CreateMacro: vi.fn().mockReturnValue(pending) });
  render(<MacroManagerPanel service={service} />);
  expect(screen.getByText("Loading library…")).toBeInTheDocument();
  expect(await screen.findByRole("alert")).toHaveTextContent("Unavailable");
  expect(screen.queryByText("No macros in your library yet.")).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Retry library" }));
  await screen.findByText("No macros in your library yet.");
  fireEvent.click(screen.getByRole("button", { name: "New macro" }));
  fireEvent.change(screen.getByLabelText("Macro name"), { target: { value: "Unsaved" } });
  const save = screen.getByRole("button", { name: "Save to library" });
  fireEvent.click(save); fireEvent.click(save);
  expect(service.CreateMacro).toHaveBeenCalledTimes(1);
  expect(save).toBeDisabled();
  await act(async () => reject(new Error("Cannot save")));
  expect(screen.getByLabelText("Macro name")).toHaveValue("Unsaved");
  expect(screen.getByRole("alert")).toHaveTextContent("Cannot save");
});
