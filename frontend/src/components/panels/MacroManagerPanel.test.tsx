import "@testing-library/jest-dom/vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { MacroManagerPanel } from "./MacroManagerPanel";
import { useMacroLibrary } from "../../hooks/useMacroLibrary";
import type { Macro, MacroLibraryService } from "../../desktop-contract";

afterEach(() => { cleanup(); vi.restoreAllMocks(); });

function selectPolicy(policy: "minimum" | "fixed" | "recorded") {
  const label = { minimum: "Minimum · 0 local ms", fixed: "Default · 10 ms", recorded: "Recorded · between presses" }[policy];
  fireEvent.click(screen.getByRole("button", { name: "Recording delay", exact: true }));
  fireEvent.click(screen.getByRole("menuitem", { name: label, exact: true }));
}

it("uses a styled recording policy menu and preserves take policy selection", async () => {
  const service = serviceFor();
  render(<MacroManagerPanel service={service} />);
  fireEvent.click(await screen.findByRole("button", { name: "Clicks · 2 events" }));
  await screen.findByDisplayValue("Clicks");
  const trigger = screen.getByRole("button", { name: "Recording delay", exact: true });
  expect(trigger).toHaveTextContent("Default · 10 ms");
  fireEvent.click(trigger);
  const menu = screen.getByRole("menu", { name: "Recording delay categories" });
  expect(menu).toHaveClass("gnome-select-popup");
  expect(screen.getAllByRole("menuitem")).toHaveLength(3);
  fireEvent.click(screen.getByRole("menuitem", { name: "Minimum · 0 local ms" }));
  expect(trigger).toHaveTextContent("Minimum · 0 local ms");
  expect(screen.queryByRole("menu")).not.toBeInTheDocument();
  expect(trigger).toHaveFocus();
  fireEvent.click(screen.getByRole("button", { name: "Start recording" }));
  expect(trigger).toBeDisabled();
  const surface = screen.getByRole("region", { name: "Local input capture" });
  fireEvent.keyDown(surface, { code: "KeyF" });
  fireEvent.keyUp(surface, { code: "KeyF" });
  fireEvent.click(screen.getByRole("button", { name: "Stop recording" }));
  fireEvent.click(screen.getByRole("button", { name: "Save to library" }));
  await waitFor(() => expect(service.UpdateMacro).toHaveBeenCalledWith("one", "Clicks", [...macro().events,
    { type: "keyboard", key_code: "KeyF", action: "down", delay_ms: 0 },
    { type: "keyboard", key_code: "KeyF", action: "up", delay_ms: 0 }]));
});

it("shows one keypress without changing raw actions or saved delays", async () => {
  const events: Macro["events"] = [
    { type: "keyboard", key_code: "KeyF", action: "down", delay_ms: 10 },
    { type: "keyboard", key_code: "KeyF", action: "up", delay_ms: 20 },
  ];
  const stored: Macro = { id: "one", name: "Keys", events };
  const service = serviceFor({ ListMacros: vi.fn().mockResolvedValue([stored]), ReadMacro: vi.fn().mockResolvedValue(stored) });
  render(<MacroManagerPanel service={service} />);
  fireEvent.click(await screen.findByRole("button", { name: "Keys · 2 events" }));
  const list = await screen.findByRole("list", { name: "Ordered timeline" });
  expect(list.querySelectorAll("strong")).toHaveLength(1);
  expect(Array.from(list.querySelectorAll("strong"), row => row.textContent)).toEqual(["Key F"]);
  expect(list).not.toHaveTextContent(/\b(down|up)\b/);
  expect(list).toHaveTextContent("↓ 10 ms / ↑ 20 ms");
  expect(screen.getByText("1 press")).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Save to library" }));
  await waitFor(() => expect(service.UpdateMacro).toHaveBeenCalledWith("one", "Keys", events));
});

it("counts five presses during recording and after append without discarding ten raw events", async () => {
  const service = serviceFor();
  render(<MacroManagerPanel service={service} />);
  fireEvent.click(await screen.findByRole("button", { name: "Clicks · 2 events" }));
  await waitFor(() => expect(screen.getByRole("button", { name: "Start recording" })).toBeEnabled());
  fireEvent.click(screen.getByRole("button", { name: "Start recording" }));
  const surface = screen.getByRole("region", { name: "Local input capture" });
  expect(surface).toHaveTextContent("Recording · 0 presses");
  for (const [index, code] of ["KeyE", "KeyS", "KeyT", "KeyO", "KeyE"].entries()) {
    fireEvent.keyDown(surface, { code });
    expect(surface).toHaveTextContent(`Recording · ${index + 1} ${index === 0 ? "press" : "presses"}`);
    fireEvent.keyUp(surface, { code });
    expect(surface).toHaveTextContent(`Recording · ${index + 1} ${index === 0 ? "press" : "presses"}`);
  }
  fireEvent.click(screen.getByRole("button", { name: "Stop recording" }));
  expect(screen.getByRole("status")).toHaveTextContent("5 presses");
  fireEvent.click(screen.getByRole("button", { name: "Save to library" }));
  await waitFor(() => expect(service.UpdateMacro).toHaveBeenCalledWith("one", "Clicks", [...macro().events,
    ...["KeyE", "KeyS", "KeyT", "KeyO", "KeyE"].flatMap(key_code => [
      { type: "keyboard", key_code, action: "down", delay_ms: 10 },
      { type: "keyboard", key_code, action: "up", delay_ms: 10 },
    ])]));
});

it("uses uniform down/up slots for chords without reordering, merging or inventing delays", async () => {
  const events: Macro["events"] = [
    { type: "keyboard", key_code: "KeyA", action: "down", delay_ms: 4 },
    { type: "keyboard", key_code: "KeyD", action: "down", delay_ms: 10 },
    { type: "keyboard", key_code: "KeyA", action: "up", delay_ms: 7 },
    { type: "keyboard", key_code: "KeyD", action: "up", delay_ms: 8 },
  ];
  const stored = { id: "one", name: "Chord", events };
  const service = serviceFor({ ListMacros: vi.fn().mockResolvedValue([stored]), ReadMacro: vi.fn().mockResolvedValue(stored) });
  render(<MacroManagerPanel service={service} />);
  const card = await screen.findByRole("button", { name: "Chord · 4 events" });
  await act(async () => { fireEvent.click(card); });
  const list = screen.getByRole("list", { name: "Ordered timeline" });
  expect(Array.from(list.querySelectorAll("strong"), row => row.textContent)).toEqual(["Key A", "Key D", "Key A", "Key D"]);
  expect(Array.from(list.querySelectorAll("small"), row => row.textContent)).toEqual([
    "↓ 4 ms / ↑ —", "↓ 10 ms / ↑ —", "↓ — / ↑ 7 ms", "↓ — / ↑ 8 ms",
  ]);
  expect(screen.getByText("2 presses", { exact: true })).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Save to library" }));
  await waitFor(() => expect(service.UpdateMacro).toHaveBeenCalledWith("one", "Chord", events));
});

it("keeps recording visually concise with optional technical details", async () => {
  render(<MacroManagerPanel service={serviceFor()} />);
  fireEvent.click(await screen.findByRole("button", { name: "Clicks · 2 events" }));
  const options = (await screen.findByText("Recording options")).closest("details");
  expect(options).not.toHaveAttribute("open");
  expect(screen.getByRole("checkbox", { name: /Record Tab/, hidden: true }).closest("details")).toBe(options);
  expect(screen.queryByText(/46 source events/)).not.toBeInTheDocument();
  expect(screen.getByRole("region", { name: "Local input capture" })).toHaveAccessibleDescription("Press keys or click here.");
  expect(screen.getByRole("button", { name: "Start recording" })).toHaveTextContent("Record");
  expect(screen.getByRole("button", { name: "Stop recording" })).toHaveTextContent("Stop");
});

it("captures only the focused surface, locks controls and appends without rewriting prior events", async () => {
  const service = serviceFor();
  render(<MacroManagerPanel service={service} />);
  fireEvent.click(await screen.findByRole("button", { name: "Clicks · 2 events" }));
  await screen.findByDisplayValue("Clicks");
  selectPolicy("recorded");
  fireEvent.click(screen.getByRole("button", { name: "Start recording" }));
  const surface = screen.getByRole("region", { name: "Local input capture" });
  expect(surface).toHaveFocus();
  expect(screen.getByLabelText("Recording delay")).toBeDisabled();
  expect(screen.getByRole("button", { name: "Save to library" })).toBeDisabled();
  expect(screen.getByRole("button", { name: "New macro" })).toBeDisabled();
  let time = 0;
  vi.spyOn(performance, "now").mockImplementation(() => time);
  fireEvent.keyDown(surface, { code: "ControlLeft" });
  fireEvent.keyDown(surface, { code: "ControlLeft", repeat: true });
  time = 120;
  fireEvent.keyDown(surface, { code: "UnknownPhysical" });
  time = 180;
  fireEvent.keyUp(surface, { code: "UnknownPhysical" });
  fireEvent.keyDown(screen.getByLabelText("Macro name"), { code: "KeyZ" });
  fireEvent.blur(surface);
  expect(screen.getByRole("status")).toHaveTextContent("2 presses");
  selectPolicy("fixed");
  fireEvent.click(screen.getByRole("button", { name: "Save to library" }));
  await waitFor(() => expect(service.UpdateMacro).toHaveBeenCalledWith("one", "Clicks", [
    ...macro().events,
    { type: "keyboard", key_code: "ControlLeft", action: "down", delay_ms: 120 },
    { type: "keyboard", key_code: "UnknownPhysical", action: "down", delay_ms: 0 },
    { type: "keyboard", key_code: "UnknownPhysical", action: "up", delay_ms: 0 },
    { type: "keyboard", key_code: "ControlLeft", action: "up", delay_ms: 0 },
  ]));
});

it.each(buttonsForRecording())("captures mouse button %s with fixed delays and ignores stop clicks", async (button, type) => {
  const service = serviceFor();
  render(<MacroManagerPanel service={service} />);
  const card = await screen.findByRole("button", { name: "Clicks · 2 events" });
  await act(async () => { fireEvent.click(card); });
  await waitFor(() => expect(screen.getByRole("button", { name: "Start recording" })).toBeEnabled());
  fireEvent.click(screen.getByRole("button", { name: "Start recording" }));
  const surface = screen.getByRole("region", { name: "Local input capture" });
  expect(surface).toHaveFocus();
  expect(screen.getByRole("button", { name: "Stop recording" })).toBeEnabled();
  fireEvent.mouseDown(surface, { button });
  fireEvent.mouseUp(surface, { button });
  fireEvent.click(screen.getByRole("button", { name: "Stop recording" }));
  fireEvent.click(screen.getByRole("button", { name: "Save to library" }));
  await waitFor(() => expect(service.UpdateMacro).toHaveBeenCalledWith("one", "Clicks", [...macro().events,
    { type, action: "down", delay_ms: 10 }, { type, action: "up", delay_ms: 10 }]));
});
function buttonsForRecording() { return [[0, "mouse_left"], [1, "mouse_middle"], [2, "mouse_right"], [3, "mouse_back"], [4, "mouse_forward"]] as const; }

it("leaves navigation and outside input defaults intact and resets each subsequent take", async () => {
  const service = serviceFor();
  render(<MacroManagerPanel service={service} />);
  fireEvent.click(await screen.findByRole("button", { name: "Clicks · 2 events" }));
  await screen.findByDisplayValue("Clicks");
  const surface = screen.getByRole("region", { name: "Local input capture" });
  expect(fireEvent.keyDown(surface, { code: "KeyA" })).toBe(true);
  selectPolicy("minimum");
  fireEvent.click(screen.getByRole("button", { name: "Start recording" }));
  expect(fireEvent.keyDown(document.body, { code: "KeyZ" })).toBe(true);
  expect(fireEvent.keyDown(surface, { code: "Tab" })).toBe(true);
  expect(fireEvent.keyDown(surface, { code: "KeyA" })).toBe(false);
  fireEvent.blur(surface);
  selectPolicy("fixed");
  fireEvent.click(screen.getByRole("button", { name: "Start recording" }));
  fireEvent.keyDown(surface, { code: "KeyA" });
  fireEvent.keyUp(surface, { code: "KeyA" });
  fireEvent.click(screen.getByRole("button", { name: "Stop recording" }));
  fireEvent.click(screen.getByRole("button", { name: "Save to library" }));
  await waitFor(() => expect(service.UpdateMacro).toHaveBeenCalledWith("one", "Clicks", [...macro().events,
    { type: "keyboard", key_code: "KeyA", action: "down", delay_ms: 0 },
    { type: "keyboard", key_code: "KeyA", action: "up", delay_ms: 0 },
    { type: "keyboard", key_code: "KeyA", action: "down", delay_ms: 10 },
    { type: "keyboard", key_code: "KeyA", action: "up", delay_ms: 10 }]));
});

it("flushes held inputs on editor unmount into a surviving draft and cannot leak into a replacement service", async () => {
  let library!: ReturnType<typeof useMacroLibrary>;
  function Harness({ show, service }: { show: boolean; service: MacroLibraryService }) {
    library = useMacroLibrary(service);
    return show ? <MacroManagerPanel service={service} library={library} /> : null;
  }
  const service = serviceFor();
  const { rerender } = render(<Harness show service={service} />);
  fireEvent.click(await screen.findByRole("button", { name: "Clicks · 2 events" }));
  await screen.findByDisplayValue("Clicks");
  await waitFor(() => expect(screen.getByRole("button", { name: "Start recording" })).toBeEnabled());
  fireEvent.click(screen.getByRole("button", { name: "Start recording" }));
  expect(screen.getByRole("region", { name: "Local input capture" })).toHaveTextContent("Recording");
  fireEvent.keyDown(screen.getByRole("region", { name: "Local input capture" }), { code: "ShiftRight" });
  rerender(<Harness show={false} service={service} />);
  expect(library.recordingActive).toBe(false);
  expect(library.draft?.events).toEqual([...macro().events,
    { type: "keyboard", key_code: "ShiftRight", action: "down", delay_ms: 10 },
    { type: "keyboard", key_code: "ShiftRight", action: "up", delay_ms: 10 }]);
  rerender(<Harness show service={service} />);
  fireEvent.click(screen.getByRole("button", { name: "Start recording" }));
  fireEvent.keyDown(screen.getByRole("region", { name: "Local input capture" }), { code: "KeyZ" });
  const replacement = serviceFor();
  rerender(<Harness show service={replacement} />);
  await waitFor(() => expect(library.loaded).toBe(true));
  expect(library.recordingActive).toBe(false);
  act(() => library.newMacro());
  expect(library.draft?.events).toEqual([]);
  expect(service.UpdateMacro).not.toHaveBeenCalled();
  expect(replacement.CreateMacro).not.toHaveBeenCalled();
});

it("records opted-in Tab and Escape, finishes held modifiers once, and restores keyboard focus", async () => {
  const service = serviceFor();
  let queued!: FrameRequestCallback;
  vi.spyOn(window, "requestAnimationFrame").mockImplementation(callback => { queued = callback; return 42; });
  render(<MacroManagerPanel service={service} />);
  fireEvent.click(await screen.findByRole("button", { name: "Clicks · 2 events" }));
  await waitFor(() => expect(screen.getByRole("button", { name: "Start recording" })).toBeEnabled());
  const toggle = screen.getByRole("checkbox", { name: /Record Tab/ });
  const surface = screen.getByRole("region", { name: "Local input capture" });
  fireEvent.click(toggle);
  expect(fireEvent.keyDown(surface, { code: "Tab" })).toBe(true); // inactive, no interception
  selectPolicy("recorded");
  fireEvent.click(screen.getByRole("button", { name: "Start recording" }));
  expect(toggle).toBeDisabled();
  let time = 0;
  vi.spyOn(performance, "now").mockImplementation(() => time);
  fireEvent.keyDown(surface, { code: "ShiftLeft" });
  time = 50;
  fireEvent.keyDown(surface, { code: "Escape" });
  fireEvent.keyUp(surface, { code: "Escape" });
  expect(fireEvent.keyDown(screen.getByRole("button", { name: "Stop recording" }), { code: "KeyZ" })).toBe(true);
  time = 120;
  expect(fireEvent.keyDown(surface, { code: "Tab" })).toBe(false);
  expect(fireEvent.keyUp(surface, { code: "Tab" })).toBe(false);
  expect(surface).toHaveTextContent("⌨ / 🖱");
  fireEvent.blur(surface); // duplicate stop must not append anything again
  act(() => queued(0));
  expect(screen.getByRole("button", { name: "Start recording" })).toHaveFocus();
  expect(screen.getByRole("status")).toHaveTextContent("3 presses");
  fireEvent.click(screen.getByRole("button", { name: "Save to library" }));
  await waitFor(() => expect(service.UpdateMacro).toHaveBeenCalledWith("one", "Clicks", [...macro().events,
    { type: "keyboard", key_code: "ShiftLeft", action: "down", delay_ms: 50 },
    { type: "keyboard", key_code: "Escape", action: "down", delay_ms: 70 },
    { type: "keyboard", key_code: "Escape", action: "up", delay_ms: 0 },
    { type: "keyboard", key_code: "Tab", action: "down", delay_ms: 0 },
    { type: "keyboard", key_code: "Tab", action: "up", delay_ms: 0 },
    { type: "keyboard", key_code: "ShiftLeft", action: "up", delay_ms: 0 },
  ]));
});

it.each(["new", "selection", "replacement", "unmount", "new take", "outside focus"] as const)("does not run stale Tab focus restoration after %s", async boundary => {
  const service = serviceFor({ ListMacros: vi.fn().mockResolvedValue([macro(), macro("two", "Other")]) });
  let queued!: FrameRequestCallback;
  vi.spyOn(window, "requestAnimationFrame").mockImplementation(callback => { queued = callback; return 42; });
  vi.spyOn(window, "cancelAnimationFrame").mockImplementation(() => {});
  const view = render(<MacroManagerPanel service={service} />);
  fireEvent.click(await screen.findByRole("button", { name: "Clicks · 2 events" }));
  await waitFor(() => expect(screen.getByRole("button", { name: "Start recording" })).toBeEnabled());
  fireEvent.click(screen.getByRole("checkbox", { name: /Record Tab/ }));
  fireEvent.click(screen.getByRole("button", { name: "Start recording" }));
  const surface = screen.getByRole("region", { name: "Local input capture" });
  fireEvent.keyDown(surface, { code: "Tab" }); fireEvent.keyUp(surface, { code: "Tab" });
  if (boundary === "new") fireEvent.click(screen.getByRole("button", { name: "New macro" }));
  if (boundary === "selection") fireEvent.click(screen.getByRole("button", { name: "Other · 2 events" }));
  if (boundary === "replacement") view.rerender(<MacroManagerPanel service={serviceFor()} />);
  if (boundary === "unmount") view.unmount();
  if (boundary === "new take") fireEvent.click(screen.getByRole("button", { name: "Start recording" }));
  if (boundary === "outside focus") screen.getByRole("button", { name: "New macro" }).focus();
  const focused = document.activeElement;
  act(() => queued(0)); // deliberately invoke even a cancelled callback
  expect(document.activeElement).toBe(focused);
  expect(service.UpdateMacro).not.toHaveBeenCalled();
});

it("records, saves and reselects F/D/S ten events using physical code rather than typed key", async () => {
  const saved: Macro[] = [];
  const service = serviceFor({
    ListMacros: vi.fn().mockResolvedValue([]),
    CreateMacro: vi.fn().mockImplementation(async (name, events) => { const value = { id: "fds", name, events: structuredClone(events) }; saved.push(value); return value; }),
    ReadMacro: vi.fn().mockImplementation(async () => structuredClone(saved[0])),
  });
  render(<MacroManagerPanel service={service} />);
  await screen.findByText("No macros in your library yet.");
  fireEvent.click(screen.getByRole("button", { name: "New macro" }));
  fireEvent.change(screen.getByLabelText("Macro name"), { target: { value: "F/D/S ten" } });
  fireEvent.click(screen.getByRole("button", { name: "Start recording" }));
  const surface = screen.getByRole("region", { name: "Local input capture" });
  for (const code of ["KeyF", "KeyD", "KeyS", "KeyF", "KeyD"]) {
    fireEvent.keyDown(surface, { code, key: "z" }); fireEvent.keyUp(surface, { code, key: "z" });
  }
  fireEvent.click(screen.getByRole("button", { name: "Stop recording" }));
  const expected = ["KeyF", "KeyD", "KeyS", "KeyF", "KeyD"].flatMap(key_code => [
    { type: "keyboard", key_code, action: "down", delay_ms: 10 },
    { type: "keyboard", key_code, action: "up", delay_ms: 10 },
  ]);
  expect(screen.getByText(/Ready to assign/)).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Save to library" }));
  const card = await screen.findByRole("button", { name: "F/D/S ten · 10 events" });
  expect(service.CreateMacro).toHaveBeenCalledExactlyOnceWith("F/D/S ten", expected);
  fireEvent.click(card);
  await waitFor(() => expect(service.ReadMacro).toHaveBeenCalledWith("fds"));
  const rows = screen.getByRole("list", { name: "Ordered timeline" });
  expect(Array.from(rows.querySelectorAll("strong"), row => row.textContent)).toEqual(["Key F", "Key D", "Key S", "Key F", "Key D"]);
  expect(rows).not.toHaveTextContent(/\b(down|up)\b/);
  expect(saved[0].events).toEqual(expected);
});

it("keeps oversize drafts savable locally with visible source/pair/extension safeguards", async () => {
  const events: Macro["events"] = Array.from({ length: 24 }, () => [
    { type: "keyboard" as const, key_code: "KeyF", action: "down" as const, delay_ms: 10 },
    { type: "keyboard" as const, key_code: "KeyF", action: "up" as const, delay_ms: 10 },
  ]).flat();
  const stored = { id: "one", name: "Oversize", events };
  const service = serviceFor({ ListMacros: vi.fn().mockResolvedValue([stored]), ReadMacro: vi.fn().mockResolvedValue(stored) });
  render(<MacroManagerPanel service={service} />);
  fireEvent.click(await screen.findByRole("button", { name: "Oversize · 48 events" }));
  expect(await screen.findByText(/Local only/)).toHaveAttribute("title", expect.stringMatching(/46/));
  expect(screen.queryByText(/Events remain saved locally/)).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Save to library" }));
  await waitFor(() => expect(service.UpdateMacro).toHaveBeenCalledWith("one", "Oversize", events));
});

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

it("labels supported captured keyboard eligible without promising playback", async () => {
 const stored:Macro={id:"one",name:"Captured",events:[{type:"keyboard",key_code:"KeyA",action:"down",delay_ms:100},{type:"keyboard",key_code:"KeyA",action:"up",delay_ms:100}]};
 const service=serviceFor({ListMacros:vi.fn().mockResolvedValue([stored]),ReadMacro:vi.fn().mockResolvedValue(stored)});
 render(<MacroManagerPanel service={service}/>);
 fireEvent.click(await screen.findByRole("button",{name:"Captured · 2 events"}));
 expect(await screen.findByText(/Ready to assign/)).toHaveAttribute("title", "Playback remains unverified.");
 expect(service.UpdateMacro).not.toHaveBeenCalled();
});

it.each([
  // T4 expands balanced supported keys; retain unsupported-key rejection coverage.
  [{ type: "keyboard", action: "down", delay_ms: 0, key_code: "F13" }, { type: "keyboard", action: "up", delay_ms: 0, key_code: "F13" }],
  [{ type: "mouse_left", action: "down", delay_ms: 0 }, { type: "mouse_left", action: "up", delay_ms: 0 }, { type: "keyboard", action: "down", delay_ms: 0, key_code: "F13" }, { type: "keyboard", action: "up", delay_ms: 0, key_code: "F13" }],
  [{ type: "mouse_unknown", action: "down", delay_ms: 0 }, { type: "mouse_unknown", action: "up", delay_ms: 0 }],
  [{ type: "mouse_left", action: "down", delay_ms: 0, key_code: " " }, { type: "mouse_left", action: "up", delay_ms: 0 }],
  [{ type: "mouse_left", action: "down", delay_ms: 0 }, { type: "mouse_left", action: "up", delay_ms: 0, key_code: "KeyA" }],
].map((events) => ({ events })))("never labels unsupported keyboard/mixed/unknown/stale sequences eligible: %j", async ({ events: raw }) => {
  const stored = { id: "one", name: "Unsafe", events: raw } as Macro;
  const service = serviceFor({ ListMacros: vi.fn().mockResolvedValue([stored]), ReadMacro: vi.fn().mockResolvedValue(stored) });
  render(<MacroManagerPanel service={service} />);
  fireEvent.click(await screen.findByRole("button", { name: `Unsafe · ${raw.length} events` }));
  const list = await screen.findByRole("list", { name: "Ordered timeline" });
  expect(screen.getByText(/Local only/)).toBeInTheDocument();
  expect(screen.queryByText(/Eligible for assignment/)).not.toBeInTheDocument();
  if (raw.some((event) => event.type === "keyboard")) {
    expect(list).toHaveTextContent("Key F13");
    expect(Array.from(list.querySelectorAll("strong")).filter(row => row.textContent === "Key F13")).toHaveLength(1);
  }
});
it("imports, labels and exports the saved opaque keyboard copy without hardware calls", async () => {
  const events = [{ type: "keyboard", action: "down", delay_ms: 0, key_code: " UnknownPhysicalKey " },
    { type: "keyboard", action: "up", delay_ms: 7, key_code: "KeyA" }];
  const service = serviceFor();
  let blob!: Blob;
  vi.spyOn(URL, "createObjectURL").mockImplementation((value: unknown) => { blob = value as Blob; return "blob:keyboard"; });
  vi.spyOn(URL, "revokeObjectURL").mockImplementation(() => {});
  vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => {});
  render(<MacroManagerPanel service={service} />);
  await screen.findByRole("button", { name: "Clicks · 2 events" });
  upload(JSON.stringify({ version: 1, name: "Keyboard", events }));
  await screen.findByDisplayValue("Keyboard");
  expect(service.CreateMacro).toHaveBeenLastCalledWith("Keyboard", events);
  expect(screen.getByRole("list", { name: "Ordered timeline" })).toHaveTextContent("Key UnknownPhysicalKey");
  expect(screen.getByText(/Local only/)).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Export saved macro" }));
  const text = await new Promise<string>((resolve) => {
    const reader = new FileReader(); reader.onload = () => resolve(String(reader.result)); reader.readAsText(blob);
  });
  expect(JSON.parse(text)).toEqual({ version: 1, name: "Keyboard", events });
});
it("renders clean guidance text for button assignment and no cross-view assign button", async () => {
  const service = serviceFor();
  render(<MacroManagerPanel service={service} />);
  fireEvent.click(await screen.findByRole("button", { name: "Clicks · 2 events" }));
  await screen.findByDisplayValue("Clicks");
  expect(screen.queryByText("To assign this macro to a mouse button, open Button remapping.")).not.toBeInTheDocument();
  expect(screen.queryByText("Shared across devices. Available offline.")).not.toBeInTheDocument();
  expect(screen.queryByText("Local library · no device changes")).not.toBeInTheDocument();
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

it("groups adjacent mouse pairs regardless of delay without rewriting raw events", async () => {
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
  expect(list.children).toHaveLength(2);
  expect(list).toHaveTextContent("Middle click");
  expect(list).toHaveTextContent("↓ 0 ms / ↑ 0 ms");
  expect(list).toHaveTextContent("Back click");
  expect(list).toHaveTextContent("↓ 0 ms / ↑ 7 ms");
  fireEvent.click(screen.getByRole("button", { name: "Save to library" }));
  await waitFor(() => expect(service.UpdateMacro).toHaveBeenCalledWith("one", "Mixed", events));
  expect(screen.getByText(/Ready to assign/)).toBeInTheDocument();
});

it("renders expanded compatibility for complete clicks while incomplete locals remain closed", async () => {
  const service = serviceFor();
  render(<MacroManagerPanel service={service} />);
  fireEvent.click(await screen.findByRole("button", { name: "Clicks · 2 events" }));
  await screen.findByDisplayValue("Clicks");
  expect(screen.getByText(/Local only/)).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "New macro" }));
  expect(screen.getByRole("list", { name: "Ordered timeline" }).children).toHaveLength(0);
  expect(screen.getByText("0 presses", { exact: true })).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Add click" }));
  expect(screen.getByText(/Ready to assign/)).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Add click" }));
  expect(screen.getByText(/Ready to assign/)).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Add click" }));
  expect(screen.getByText(/Ready to assign/)).toBeInTheDocument();
});

it("does not render fake recording zone, advanced raw events editor, or bottom disclaimers", async () => {
  render(<MacroManagerPanel service={serviceFor()} />);
  fireEvent.click(await screen.findByRole("button", { name: "Clicks · 2 events" }));
  await screen.findByDisplayValue("Clicks");
  expect(screen.queryByText("Advanced events and recording")).not.toBeInTheDocument();
  expect(screen.queryByRole("region", { name: "Mouse recording zone" })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Arm recording" })).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Stop recording" })).toBeDisabled();
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

it.each(["new", "select"] as const)("recovers from a stalled read via %s without admitting recording or save", async (recovery) => {
  let resolve!: (value: Macro) => void;
  const delayed = new Promise<Macro>((done) => { resolve = done; });
  const service = serviceFor({
    ListMacros: vi.fn().mockResolvedValue([macro("one"), macro("two")]),
    ReadMacro: vi.fn().mockResolvedValueOnce(macro("one"))
      .mockImplementation((id) => (id === "one" ? delayed : Promise.resolve(macro("two", "Second")))),
  });
  render(<MacroManagerPanel service={service} />);
  const cards = await screen.findAllByRole("button", { name: "Clicks · 2 events" });
  fireEvent.click(cards[0]);
  await waitFor(() => expect(screen.getByRole("button", { name: "Start recording" })).toBeEnabled());
  fireEvent.click(cards[0]);
  expect(screen.getByRole("status", { name: "Operation status" })).toBeInTheDocument();
  const start = screen.getByRole("button", { name: "Start recording" });
  const save = screen.getByRole("button", { name: "Save to library" });
  expect(start).toBeDisabled();
  expect(save).toBeDisabled();
  fireEvent.click(start);
  fireEvent.click(save);
  expect(screen.getByRole("region", { name: "Local input capture" })).toHaveTextContent("⌨ / 🖱");
  expect(service.UpdateMacro).not.toHaveBeenCalled();
  expect(screen.getByRole("button", { name: "New macro" })).toBeEnabled();
  expect(cards[1]).toBeEnabled();
  if (recovery === "new") {
    fireEvent.click(screen.getByRole("button", { name: "New macro" }));
    fireEvent.change(screen.getByLabelText("Macro name"), { target: { value: "Recovered draft" } });
  } else {
    fireEvent.click(cards[1]);
  }
  const name = recovery === "new" ? "Recovered draft" : "Second";
  await waitFor(() => expect(screen.getByLabelText("Macro name")).toHaveValue(name));
  expect(screen.queryByRole("status", { name: "Operation status" })).not.toBeInTheDocument();
  await act(async () => resolve(macro("one", "Late stale read")));
  expect(screen.getByLabelText("Macro name")).toHaveValue(name);
  expect(screen.getByRole("list", { name: "Ordered timeline" }).children).toHaveLength(recovery === "new" ? 0 : 2);
  expect(cards[1]).toHaveAttribute("aria-pressed", String(recovery === "select"));
  expect(service.ReadMacro).toHaveBeenCalledTimes(recovery === "new" ? 2 : 3);
  expect(service.CreateMacro).not.toHaveBeenCalled();
  expect(service.UpdateMacro).not.toHaveBeenCalled();
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
  expect(screen.getByRole("status", { name: "Operation status" })).toBeInTheDocument();
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
