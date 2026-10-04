import "@testing-library/jest-dom/vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { MacroManagerPanel } from "./MacroManagerPanel";
import type { Macro, MacroLibraryService } from "../../desktop-contract";

afterEach(cleanup);
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
