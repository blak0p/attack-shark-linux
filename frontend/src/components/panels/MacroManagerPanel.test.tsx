import "@testing-library/jest-dom/vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { MacroManagerPanel } from "./MacroManagerPanel";
import type { Macro, MacroLibraryService } from "../../desktop-contract";

afterEach(() => { cleanup(); vi.restoreAllMocks(); });

const openOptions = () => {
  const summary = screen.getByText("Macro options");
  if (!(summary.parentElement as HTMLDetailsElement).open) fireEvent.click(summary);
};

const upload = (text: string) => {
  openOptions();
  const file = new File([text], "macro.json", { type: "application/json" });
  Object.defineProperty(file, "text", { value: async () => text });
  fireEvent.change(screen.getByLabelText("Import macro JSON"), { target: { files: [file] } });
};

const fileText = JSON.stringify({ version: 1, name: "Clicks", events: [{ type: "mouse_left", action: "up", delay_ms: 5 }] });

const buttons = [
  [0, "mouse_left", "Left mouse"],
  [1, "mouse_middle", "Middle mouse"],
  [2, "mouse_right", "Right mouse"],
  [3, "mouse_back", "Back mouse"],
  [4, "mouse_forward", "Forward mouse"],
] as const;

const macro = (id = "one", name = "Clicks"): Macro => ({
  id,
  name,
  events: [
    { type: "mouse_left", action: "down", delay_ms: 12 },
    { type: "mouse_right", action: "up", delay_ms: 34 },
  ],
});

const serviceFor = (overrides: Partial<MacroLibraryService> = {}): MacroLibraryService => ({
  ListMacros: vi.fn().mockResolvedValue([macro()]),
  ReadMacro: vi.fn().mockImplementation(async (id) => macro(id)),
  CreateMacro: vi.fn().mockImplementation(async (name, events) => ({ id: "new", name, events })),
  UpdateMacro: vi.fn().mockImplementation(async (id, name, events) => ({ id, name, events })),
  DeleteMacro: vi.fn().mockResolvedValue(undefined),
  ...overrides,
});

it("renders clean guidance text for button assignment and no cross-view assign button", async () => {
  const service = serviceFor();
  render(<MacroManagerPanel service={service} />);
  fireEvent.click(await screen.findByRole("button", { name: "Clicks · 2 events" }));
  await screen.findByDisplayValue("Clicks");
  expect(screen.getByText("To assign this macro to a mouse button, open Button remapping.")).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Assign to button" })).not.toBeInTheDocument();
});

it("renders styled Import JSON button that triggers hidden file input dialog", async () => {
  const service = serviceFor();
  render(<MacroManagerPanel service={service} />);
  await screen.findByRole("button", { name: "Clicks · 2 events" });
  openOptions();
  const fileInput = screen.getByLabelText("Import macro JSON");
  expect(fileInput).toHaveAttribute("type", "file");
  expect(fileInput).toHaveStyle({ display: "none" });
  const clickSpy = vi.spyOn(fileInput, "click");
  const importBtn = screen.getByRole("button", { name: "Import JSON" });
  expect(importBtn).toBeInTheDocument();
  fireEvent.click(importBtn);
  expect(clickSpy).toHaveBeenCalledTimes(1);
});

it("authors repeated complete five-button clicks atomically via composer without implicit save", async () => {
  const service = serviceFor();
  render(<MacroManagerPanel service={service} />);
  await screen.findByRole("button", { name: "Clicks · 2 events" });
  fireEvent.click(screen.getByRole("button", { name: "New macro" }));
  fireEvent.change(screen.getByLabelText("Macro name"), { target: { value: "Composer" } });
  for (const [, type, label] of buttons) {
    fireEvent.click(screen.getByRole("button", { name: `${label.replace(" mouse", "")} click`, exact: true }));
    fireEvent.click(screen.getByRole("button", { name: "Add click" }));
  }
  expect(service.CreateMacro).not.toHaveBeenCalled();
  const list = screen.getByRole("list", { name: "Ordered timeline" });
  expect(list.children).toHaveLength(5);
  fireEvent.click(screen.getByRole("button", { name: "Save to library" }));
  await waitFor(() =>
    expect(service.CreateMacro).toHaveBeenCalledWith(
      "Composer",
      buttons.flatMap(([, type]) => [
        { type, action: "down", delay_ms: 0 },
        { type, action: "up", delay_ms: 0 },
      ])
    )
  );
});

it("groups adjacent zero-delay pairs in timeline and displays raw events with preserved delays", async () => {
  const events: Macro["events"] = [
    { type: "mouse_middle", action: "down", delay_ms: 0 },
    { type: "mouse_middle", action: "up", delay_ms: 0 },
    { type: "mouse_back", action: "down", delay_ms: 0 },
    { type: "mouse_back", action: "up", delay_ms: 7 },
  ];
  const mixed = { id: "one", name: "Mixed", events };
  const service = serviceFor({ ListMacros: vi.fn().mockResolvedValue([mixed]), ReadMacro: vi.fn().mockResolvedValue(mixed) });
  render(<MacroManagerPanel service={service} />);
  fireEvent.click(await screen.findByRole("button", { name: "Mixed · 4 events" }));
  const list = await screen.findByRole("list", { name: "Ordered timeline" });
  expect(list.children).toHaveLength(3);
  expect(list).toHaveTextContent("Middle click");
  expect(list).toHaveTextContent("Complete click · zero local delay");
  expect(list).toHaveTextContent("Back · up");
  expect(list).toHaveTextContent("Raw event 4 · 7 ms");
  expect(screen.getByText(/Local sequence only/)).toBeInTheDocument();
});

it("renders compatibility indicator for 1 or 2 complete clicks and local-only for others", async () => {
  const service = serviceFor();
  render(<MacroManagerPanel service={service} />);
  fireEvent.click(await screen.findByRole("button", { name: "Clicks · 2 events" }));
  await screen.findByDisplayValue("Clicks");
  expect(screen.getByText(/Local sequence only/)).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "New macro" }));
  expect(screen.getByText("No actions yet. Add a click above. Empty macros can be saved.")).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Add click" }));
  expect(screen.getByText(/Eligible for assignment after saving/)).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Add click" }));
  expect(screen.getByText(/Eligible for assignment after saving/)).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Add click" }));
  expect(screen.getByText(/Local sequence only/)).toBeInTheDocument();
});

it("does not render fake recording zone, advanced raw events editor, or bottom disclaimers", async () => {
  render(<MacroManagerPanel service={serviceFor()} />);
  fireEvent.click(await screen.findByRole("button", { name: "Clicks · 2 events" }));
  await screen.findByDisplayValue("Clicks");
  expect(screen.queryByText("Advanced events and recording")).not.toBeInTheDocument();
  expect(screen.queryByRole("region", { name: "Mouse recording zone" })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Arm recording" })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Stop recording" })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Add event" })).not.toBeInTheDocument();
  expect(screen.queryByText("Saving and assignment")).not.toBeInTheDocument();
});

it("imports duplicates as new copies, preserves drafts on errors, and never calls device APIs", async () => {
  const device = vi.fn();
  const service = {
    ...serviceFor({
      CreateMacro: vi.fn()
        .mockRejectedValueOnce(new Error("Disk full"))
        .mockImplementation(async (name, events) => ({ id: "fresh", name, events })),
    }),
    ApplyRemap: device,
  };
  render(<MacroManagerPanel service={service} />);
  fireEvent.click(await screen.findByRole("button", { name: "Clicks · 2 events" }));
  await screen.findByDisplayValue("Clicks");
  fireEvent.change(await screen.findByLabelText("Macro name"), { target: { value: "Unsaved" } });
  upload("{");
  expect(await screen.findByRole("alert")).toHaveTextContent("JSON");
  expect(service.CreateMacro).not.toHaveBeenCalled();
  expect(screen.getByLabelText("Macro name")).toHaveValue("Unsaved");
  upload(fileText);
  expect(await screen.findByRole("alert")).toHaveTextContent("Disk full");
  expect(screen.getByLabelText("Macro name")).toHaveValue("Unsaved");
  upload(fileText);
  await screen.findByRole("button", { name: "Clicks · 1 events" });
  expect(service.CreateMacro).toHaveBeenLastCalledWith("Clicks", [{ type: "mouse_left", action: "up", delay_ms: 5 }]);
  expect(device).not.toHaveBeenCalled();
});

it("rejects oversized uploads before reading and preserves library", async () => {
  const service = serviceFor();
  render(<MacroManagerPanel service={service} />);
  await screen.findByRole("button", { name: "Clicks · 2 events" });
  openOptions();
  const input = screen.getByLabelText("Import macro JSON");
  const file = new File([], "large.json");
  const read = vi.fn().mockRejectedValue(new Error("File unreadable"));
  Object.defineProperty(file, "size", { configurable: true, value: 1024 * 1024 + 1 });
  Object.defineProperty(file, "text", { value: read });
  fireEvent.change(input, { target: { files: [file] } });
  expect(await screen.findByRole("alert")).toHaveTextContent("too large");
  expect(read).not.toHaveBeenCalled();
});

it("locks pending imports and ignores file read after service replacement", async () => {
  let resolve!: (text: string) => void;
  const pending = new Promise<string>((done) => { resolve = done; });
  const first = serviceFor();
  const second = serviceFor({ ListMacros: vi.fn().mockResolvedValue([]) });
  const view = render(<MacroManagerPanel service={first} />);
  await screen.findByRole("button", { name: "Clicks · 2 events" });
  const file = new File([], "macro.json");
  Object.defineProperty(file, "text", { value: () => pending });
  openOptions();
  fireEvent.change(screen.getByLabelText("Import macro JSON"), { target: { files: [file] } });
  expect(screen.getByRole("button", { name: "Import JSON" })).toBeDisabled();
  expect(screen.getByRole("button", { name: "New macro" })).toBeDisabled();
  view.rerender(<MacroManagerPanel service={second} />);
  await screen.findByText("No macros in your library yet.");
  await act(async () => resolve(fileText));
  expect(first.CreateMacro).not.toHaveBeenCalled();
  expect(second.CreateMacro).not.toHaveBeenCalled();
});

it("downloads only the saved macro without ID", async () => {
  let blob!: Blob;
  vi.spyOn(URL, "createObjectURL").mockImplementation((value: unknown) => { blob = value as Blob; return "blob:macro"; });
  const revoke = vi.spyOn(URL, "revokeObjectURL").mockImplementation(() => {});
  const click = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => {});
  const service = serviceFor();
  render(<MacroManagerPanel service={service} />);
  fireEvent.click(await screen.findByRole("button", { name: "Clicks · 2 events" }));
  await screen.findByDisplayValue("Clicks");
  openOptions();
  fireEvent.click(screen.getByRole("button", { name: "Export saved macro" }));
  const text = await new Promise<string>((resolve) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result));
    reader.readAsText(blob);
  });
  expect(JSON.parse(text)).toEqual({ version: 1, name: "Clicks", events: macro().events });
  expect(click).toHaveBeenCalledTimes(1);
  expect(revoke).toHaveBeenCalledWith("blob:macro");
});

it("requires explicit delete confirmation, supports cancel, and deletes macro", async () => {
  const service = serviceFor({ DeleteMacro: vi.fn().mockResolvedValue(undefined) });
  render(<MacroManagerPanel service={service} />);
  fireEvent.click(await screen.findByRole("button", { name: "Clicks · 2 events" }));
  await screen.findByDisplayValue("Clicks");
  fireEvent.click(screen.getByRole("button", { name: "Delete macro" }));
  expect(screen.getByRole("dialog", { name: "Delete macro confirmation" })).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  expect(service.DeleteMacro).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole("button", { name: "Delete macro" }));
  fireEvent.click(screen.getByRole("button", { name: "Confirm delete" }));
  await waitFor(() => expect(service.DeleteMacro).toHaveBeenCalledWith("one"));
});

it("creates empty draft, renames and saves macro by ID", async () => {
  const service = serviceFor();
  render(<MacroManagerPanel service={service} />);
  fireEvent.click(await screen.findByRole("button", { name: "Clicks · 2 events" }));
  await screen.findByDisplayValue("Clicks");
  fireEvent.change(screen.getByLabelText("Macro name"), { target: { value: "Renamed" } });
  fireEvent.click(screen.getByRole("button", { name: "Save to library" }));
  await waitFor(() => expect(service.UpdateMacro).toHaveBeenCalledWith("one", "Renamed", macro().events));
  fireEvent.click(screen.getByRole("button", { name: "New macro" }));
  fireEvent.change(screen.getByLabelText("Macro name"), { target: { value: "Empty" } });
  fireEvent.click(screen.getByRole("button", { name: "Save to library" }));
  await waitFor(() => expect(service.CreateMacro).toHaveBeenCalledWith("Empty", []));
});

it("keeps duplicate names distinct and ignores stale reads", async () => {
  let resolve!: (value: Macro) => void;
  const delayed = new Promise<Macro>((done) => { resolve = done; });
  const service = serviceFor({
    ListMacros: vi.fn().mockResolvedValue([macro("one"), macro("two")]),
    ReadMacro: vi.fn().mockImplementation((id) => (id === "one" ? delayed : Promise.resolve(macro("two", "Second")))),
  });
  render(<MacroManagerPanel service={service} />);
  const cards = await screen.findAllByRole("button", { name: "Clicks · 2 events" });
  fireEvent.click(cards[0]);
  fireEvent.click(cards[1]);
  await waitFor(() => expect(screen.getByLabelText("Macro name")).toHaveValue("Second"));
  await act(async () => resolve(macro("one", "Stale")));
  expect(screen.getByLabelText("Macro name")).toHaveValue("Second");
  expect(cards[1]).toHaveAttribute("aria-pressed", "true");
});

it("preserves selection and unsaved name when another read or list refresh fails", async () => {
  const service = serviceFor({
    ListMacros: vi.fn().mockResolvedValueOnce([macro(), macro("two", "Other")]).mockRejectedValue(new Error("Refresh failed")),
    ReadMacro: vi.fn().mockResolvedValueOnce(macro()).mockRejectedValue(new Error("Read failed")),
  });
  render(<MacroManagerPanel service={service} />);
  fireEvent.click(await screen.findByRole("button", { name: "Clicks · 2 events" }));
  await screen.findByDisplayValue("Clicks");
  fireEvent.change(screen.getByLabelText("Macro name"), { target: { value: "Keep draft" } });
  fireEvent.click(screen.getByRole("button", { name: "Other · 2 events" }));
  expect(await screen.findByRole("alert")).toHaveTextContent("Read failed");
  expect(screen.getByLabelText("Macro name")).toHaveValue("Keep draft");
  expect(screen.getByRole("button", { name: "Clicks · 2 events" })).toHaveAttribute("aria-pressed", "true");
  fireEvent.click(screen.getByRole("button", { name: "Retry library" }));
  await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("Refresh failed"));
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

it("shows loading/list errors, retries, and retains draft while preventing duplicate submissions", async () => {
  let reject!: (error: Error) => void;
  const pending = new Promise<Macro>((_, fail) => { reject = fail; });
  const service = serviceFor({
    ListMacros: vi.fn().mockRejectedValueOnce(new Error("Unavailable")).mockResolvedValue([]),
    CreateMacro: vi.fn().mockReturnValue(pending),
  });
  render(<MacroManagerPanel service={service} />);
  expect(screen.getByText("Loading library…")).toBeInTheDocument();
  expect(await screen.findByRole("alert")).toHaveTextContent("Unavailable");
  fireEvent.click(screen.getByRole("button", { name: "Retry library" }));
  await screen.findByText("No macros in your library yet.");
  fireEvent.click(screen.getByRole("button", { name: "New macro" }));
  fireEvent.change(screen.getByLabelText("Macro name"), { target: { value: "Unsaved" } });
  const save = screen.getByRole("button", { name: "Save to library" });
  fireEvent.click(save);
  fireEvent.click(save);
  expect(service.CreateMacro).toHaveBeenCalledTimes(1);
  expect(save).toBeDisabled();
  await act(async () => reject(new Error("Cannot save")));
  expect(screen.getByLabelText("Macro name")).toHaveValue("Unsaved");
  expect(screen.getByRole("alert")).toHaveTextContent("Cannot save");
});

it("displays and saves imported five-button raw events and delays losslessly in timeline", async () => {
  const service = serviceFor();
  const events: Macro["events"] = buttons.map(([, type], index) => ({
    type, action: index % 2 ? "up" : "down", delay_ms: index * 17,
  }));
  render(<MacroManagerPanel service={service} />);
  await screen.findByRole("button", { name: "Clicks · 2 events" });
  upload(JSON.stringify({ version: 1, name: "Raw five", events }));
  await screen.findByRole("button", { name: "Raw five · 5 events" });
  const list = screen.getByRole("list", { name: "Ordered timeline" });
  buttons.forEach(([, , label]) => {
    expect(list).toHaveTextContent(label.replace(" mouse", ""));
  });
  expect(service.CreateMacro).toHaveBeenCalledWith("Raw five", events);
  fireEvent.click(screen.getByRole("button", { name: "Save to library" }));
  await waitFor(() => expect(service.UpdateMacro).toHaveBeenCalledWith("new", "Raw five", events));
});

it("locks composer controls during save and delete confirmation", async () => {
  let resolve!: (value: Macro) => void;
  const pending = new Promise<Macro>((done) => { resolve = done; });
  const service = serviceFor({ CreateMacro: vi.fn().mockReturnValue(pending) });
  render(<MacroManagerPanel service={service} />);
  await screen.findByRole("button", { name: "Clicks · 2 events" });
  fireEvent.click(screen.getByRole("button", { name: "New macro" }));
  fireEvent.change(screen.getByLabelText("Macro name"), { target: { value: "New click" } });
  fireEvent.click(screen.getByRole("button", { name: "Add click" }));
  fireEvent.click(screen.getByRole("button", { name: "Save to library" }));
  expect(screen.getByRole("button", { name: "Add click" })).toBeDisabled();
  expect(screen.getByRole("button", { name: "Left click" })).toBeDisabled();
  await act(async () =>
    resolve({
      id: "new",
      name: "New click",
      events: [
        { type: "mouse_left", action: "down", delay_ms: 0 },
        { type: "mouse_left", action: "up", delay_ms: 0 },
      ],
    })
  );
  fireEvent.click(screen.getByRole("button", { name: "Delete macro" }));
  expect(screen.getByRole("button", { name: "Add click" })).toBeDisabled();
  fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
  expect(screen.getByRole("button", { name: "Add click" })).toBeEnabled();
});

it("disables save when macro name is empty or only whitespace", async () => {
  render(<MacroManagerPanel service={serviceFor()} />);
  fireEvent.click(await screen.findByRole("button", { name: "Clicks · 2 events" }));
  await screen.findByDisplayValue("Clicks");
  fireEvent.change(screen.getByLabelText("Macro name"), { target: { value: "   " } });
  expect(screen.getByRole("button", { name: "Save to library" })).toBeDisabled();
  fireEvent.change(screen.getByLabelText("Macro name"), { target: { value: "Valid name" } });
  expect(screen.getByRole("button", { name: "Save to library" })).toBeEnabled();
});

it("resets unsaved edits when selecting another macro", async () => {
  const original = macro();
  const service = serviceFor({ ReadMacro: vi.fn().mockResolvedValue(original), ListMacros: vi.fn().mockResolvedValue([original]) });
  render(<MacroManagerPanel service={service} />);
  const card = await screen.findByRole("button", { name: "Clicks · 2 events" });
  fireEvent.click(card);
  await screen.findByDisplayValue("Clicks");
  fireEvent.change(screen.getByLabelText("Macro name"), { target: { value: "Unsaved Name" } });
  fireEvent.click(card);
  await waitFor(() => expect(screen.getByLabelText("Macro name")).toHaveValue("Clicks"));
});
