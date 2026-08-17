import "@testing-library/jest-dom/vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { App, type ConfigurationEvent, type DesktopService, type Snapshot } from "./App";
import type { Binding } from "../bindings/github.com/blak0p/attack-shark-linux/internal/desktop/models";

afterEach(cleanup);

const configuration = (firstDPI = 800) => ({
  DPI: [firstDPI, 1200, 1600, 2400, 3200, 6400, 12800, 26000],
  ActiveStage: 4,
  StageMask: 255,
  LiftDistance: 1,
  Colors: [[255, 0, 0], [0, 255, 0], [0, 0, 255], [0, 255, 255], [255, 255, 0], [255, 0, 255], [0, 0, 0], [255, 255, 255]],
});

const snapshot = (overrides: Partial<Snapshot> = {}): Snapshot => ({
  Connection: "dongle",
  Battery: 84,
  Applied: configuration(),
  Pending: configuration(),
  Factory: configuration(),
  Revision: 0,
  Error: { Code: "" },
  ...overrides,
});

const selectedDevice = { ID: { VendorID: 0x1D57, ProductID: 0xFA60, Serial: "alpha" }, Profile: "attack-shark-x6", ProfileID: "attack-shark-x6", Path: "/dev/hidraw0", Eligible: true, InventoryRevision: 0, SessionOnly: false };

const serviceFor = (initial: Snapshot, overrides: Partial<DesktopService> = {}): DesktopService => ({
  GetSnapshot: vi.fn().mockResolvedValue(initial),
  RefreshStatus: vi.fn().mockResolvedValue(initial),
  RefreshInventory: vi.fn().mockResolvedValue({ Devices: [selectedDevice], Selected: selectedDevice, Error: { Code: "" } }),
  SelectDevice: vi.fn().mockResolvedValue({ Devices: [], Selected: null, Error: { Code: "" } }),
	StageDPI: vi.fn().mockImplementation(async (next) => ({ ...initial, Pending: next, Revision: initial.Revision + 1 })),
  RetryPersistence: vi.fn().mockResolvedValue(initial),
  OnStatusEvent: vi.fn().mockReturnValue(() => {}),
	OnConfiguration: vi.fn().mockReturnValue(() => {}),
  ...overrides,
});

describe("App", () => {
  it("shows available connection and supplied battery information", async () => {
    render(<App service={serviceFor(snapshot())} />);

    expect(await screen.findByText("Device available")).toBeInTheDocument();
    expect(screen.getByText("Battery 84%")).toBeInTheDocument();
  });

  it("shows unavailable status without inventing a battery value", async () => {
    const service = serviceFor(snapshot({ Connection: "", Battery: undefined, Error: { Code: "device_unavailable" } }), {
      RefreshInventory: vi.fn().mockResolvedValue({ Devices: [], Selected: null, Error: { Code: "device_unavailable" } }),
    });
    render(<App service={service} />);

    expect(await screen.findByText("Device unavailable")).toBeInTheDocument();
    expect(screen.getByText("Battery unavailable")).toBeInTheDocument();
		expect(screen.getByRole("alert")).toHaveTextContent("device unavailable");
	});

  it("requests live status on mount instead of trusting a hardware-free cached snapshot", async () => {
    const service = serviceFor(snapshot({ Connection: "", Battery: null }), {
      RefreshStatus: vi.fn().mockResolvedValue(snapshot({ Connection: "dongle", Battery: 84 })),
    });
    render(<App service={service} />);

    expect(await screen.findByText("Device available")).toBeInTheDocument();
    expect(screen.getByText("Battery 84%")).toBeInTheDocument();
    expect(service.RefreshStatus).toHaveBeenCalled();
  });

  it("shows an unavailable battery when the generated binding supplies null", async () => {
		render(<App service={serviceFor(snapshot({ Battery: null }))} />);

		expect(await screen.findByText("Battery unavailable")).toBeInTheDocument();
		expect(screen.queryByText("Battery null%")).not.toBeInTheDocument();
	});

  it("derives the active DPI stage from the snapshot instead of hardcoding it", async () => {
    render(<App service={serviceFor(snapshot())} />);

    expect(await screen.findByText("Device available")).toBeInTheDocument();
    expect(screen.getByText("Active DPI: 2400")).toBeInTheDocument();
  });

  it("renders a physical stage from its acknowledged mapping and keeps missing mappings unknown", async () => {
    const mapped = snapshot({ ObservedStage: 3, ObservedDPI: 1600 } as Snapshot);
    const { rerender } = render(<App service={serviceFor(mapped)} />);

    expect(await screen.findByText("Active DPI: 1600")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Stage 3, active" })).toBeInTheDocument();

    rerender(<App service={serviceFor(snapshot({ ObservedStage: 3, ObservedDPI: null } as Snapshot))} />);
    expect(await screen.findByText("Active DPI: unknown")).toBeInTheDocument();
  });

  it("renders only the stages enabled by the firmware StageMask", async () => {
    const masked = snapshot({ Pending: { ...configuration(), StageMask: 0x0f } });
    render(<App service={serviceFor(masked)} />);

    await screen.findByText("Device available");
    expect(screen.getByRole("slider", { name: "Stage 4 DPI" })).toBeInTheDocument();
    expect(screen.queryByRole("slider", { name: "Stage 5 DPI" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Stage 5" })).not.toBeInTheDocument();
  });

  it("colors each stage slider with its real stage color", async () => {
    render(<App service={serviceFor(snapshot())} />);

    const slider = (await screen.findByRole("slider", { name: "Stage 1 DPI" })) as HTMLInputElement;
    expect(slider.style.getPropertyValue("--stage-color")).toBe("rgb(255, 0, 0)");
  });

  it("colors each stage dot with its real stage color", async () => {
    render(<App service={serviceFor(snapshot())} />);

    await screen.findByText("Device available");
    const dot = screen.getByRole("button", { name: "Stage 1" }) as HTMLElement;
    expect(dot.style.getPropertyValue("--dot-color")).toBe("rgb(255, 0, 0)");
  });

  it("marks the active stage circle as pressed", async () => {
    render(<App service={serviceFor(snapshot())} />);

    await screen.findByText("Device available");
    expect(screen.getByRole("button", { name: "Stage 4, active" })).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("button", { name: "Stage 1" })).toHaveAttribute("aria-pressed", "false");
  });

  it("switches the active stage by staging a config change without applying it", async () => {
    const service = serviceFor(snapshot());
    render(<App service={service} />);

    fireEvent.click(await screen.findByRole("button", { name: "Stage 1" }));

    await waitFor(() => expect(service.StageDPI).toHaveBeenCalledWith(expect.objectContaining({ ActiveStage: 1 })));
	expect(screen.getByText("Synchronization queued. It will apply after one second of inactivity.")).toBeInTheDocument();
  });

  it("stages factory defaults on Reset without applying them", async () => {
    const service = serviceFor(snapshot());
    render(<App service={service} />);

    fireEvent.click(await screen.findByRole("button", { name: /Reset to factory/ }));

    await waitFor(() => expect(service.StageDPI).toHaveBeenCalledWith(snapshot().Factory));
	expect(screen.getByText("Synchronization queued. It will apply after one second of inactivity.")).toBeInTheDocument();
  });

  it("queues a DPI edit for automatic synchronization without an explicit Save control", async () => {
    const service = serviceFor(snapshot());
    render(<App service={service} />);

    const input = await screen.findByRole("slider", { name: "Stage 1 DPI" });
    fireEvent.change(input, { target: { value: "1600" } });

    await waitFor(() => expect(service.StageDPI).toHaveBeenCalledWith(expect.objectContaining({ DPI: expect.arrayContaining([1600]) })));
	expect(screen.queryByRole("button", { name: /Save to Device/ })).not.toBeInTheDocument();
    expect(screen.getByText("Synchronization queued. It will apply after one second of inactivity.")).toBeInTheDocument();
  });

  it("renders distinct firmware and persistence outcomes with a persistence-only retry", async () => {
    const service = serviceFor(snapshot({ Firmware: "success", Persistence: "failed", RetryAvailable: true }) as Snapshot, { RetryPersistence: vi.fn().mockResolvedValue(snapshot({ Firmware: "success", Persistence: "success", RetryAvailable: false }) as Snapshot) });
    render(<App service={service} />);

    expect(await screen.findByText("Firmware applied")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Retry local persistence" }));
    await waitFor(() => expect(service.RetryPersistence).toHaveBeenCalledTimes(1));
    expect(screen.getByText("Persistence saved")).toBeInTheDocument();
  });

  it("renders pending firmware as queued feedback during automatic synchronization", async () => {
    render(<App service={serviceFor(snapshot({ Firmware: "pending" }))} />);

    expect(await screen.findByRole("status", { name: "Firmware synchronization queued" })).toBeInTheDocument();
    expect(screen.queryByText("Firmware synchronization failed")).not.toBeInTheDocument();
  });

  it("refreshes acknowledged colors without replacing editable pending controls", async () => {
    const listeners: Array<(event: ConfigurationEvent) => void> = [];
    const acknowledgedColors = [[12, 34, 56], [23, 45, 67], [34, 56, 78], [45, 67, 89], [56, 78, 90], [67, 89, 101], [78, 90, 112], [89, 101, 123]];
    const acknowledged = { ...configuration(), Colors: acknowledgedColors };
    const pending = configuration(1600);
    const service = serviceFor(snapshot(), {
      OnConfiguration: vi.fn().mockImplementation((callback) => { listeners.push(callback); return () => {}; }),
    });
    render(<App service={service} />);

    await screen.findByText("Device available");
    await act(async () => listeners[0]({ Binding: selectedDevice, Snapshot: snapshot({ Applied: acknowledged, Pending: pending, Revision: 1 }) }));

    expect((screen.getByRole("slider", { name: "Stage 1 DPI" }) as HTMLInputElement).value).toBe("1600");
    expect((screen.getByRole("slider", { name: "Stage 1 DPI" }) as HTMLInputElement).style.getPropertyValue("--stage-color")).toBe("rgb(12, 34, 56)");
    expect((screen.getByRole("button", { name: "Stage 1" }) as HTMLElement).style.getPropertyValue("--dot-color")).toBe("rgb(12, 34, 56)");
    expect((document.querySelector(".color-swatch") as HTMLElement).style.background).toBe("rgb(45, 67, 89)");
  });

  it("ignores acknowledged color events from a mismatched binding", async () => {
    const listeners: Array<(event: ConfigurationEvent) => void> = [];
    const service = serviceFor(snapshot(), {
      OnConfiguration: vi.fn().mockImplementation((callback) => { listeners.push(callback); return () => {}; }),
    });
    render(<App service={service} />);

    await screen.findByText("Device available");
    await act(async () => listeners[0]({
      Binding: { ...selectedDevice, ID: { ...selectedDevice.ID, Serial: "bravo" }, Path: "/dev/hidraw1" },
      Snapshot: snapshot({ Applied: { ...configuration(), Colors: [[12, 34, 56], ...configuration().Colors!.slice(1)] } }),
    }));

    expect((screen.getByRole("slider", { name: "Stage 1 DPI" }) as HTMLInputElement).style.getPropertyValue("--stage-color")).toBe("rgb(255, 0, 0)");
    expect((document.querySelector(".color-swatch") as HTMLElement).style.background).toBe("rgb(0, 255, 255)");
  });

  it("restores the last applied DPI state when the desktop service restarts", async () => {
    const restored = snapshot({ Applied: configuration(1600), Pending: configuration(1600), Revision: 2 });
    render(<App service={serviceFor(restored)} />);

    expect(await screen.findByRole("slider", { name: "Stage 1 DPI" })).toHaveValue("1600");
	expect(screen.queryByRole("button", { name: /Save to Device/ })).not.toBeInTheDocument();
  });

  it("subscribes to status events on mount and applies heartbeats and stage presses", async () => {
    const listeners: Array<(event: { Connection?: string; Battery?: number | null; ActiveStage?: number | null }) => void> = [];
    const service = serviceFor(snapshot({ Battery: undefined }), {
      OnStatusEvent: vi.fn().mockImplementation((callback) => { listeners.push(callback); return () => { }; }),
    });
    render(<App service={service} />);

    expect(await screen.findByText("Device available")).toBeInTheDocument();
    expect(screen.getByText("Battery unavailable")).toBeInTheDocument();

    listeners[0]({ Connection: "dongle", Battery: 90 });
    expect(await screen.findByText("Battery 90%")).toBeInTheDocument();

    listeners[0]({ ActiveStage: 2 });
    expect(await screen.findByRole("button", { name: "Stage 2, active" })).toHaveAttribute("aria-pressed", "true");

    listeners[0]({ Connection: "", Battery: null });
    expect(await screen.findByText("Device available")).toBeInTheDocument();
  });

  it("keeps its subscription active across status events without re-mounting", async () => {
    const listeners: Array<(event: { Connection?: string; Battery?: number | null; ActiveStage?: number | null }) => void> = [];
    const service = serviceFor(snapshot(), {
      OnStatusEvent: vi.fn().mockImplementation((callback) => { listeners.push(callback); return () => { }; }),
    });
    const { unmount } = render(<App service={service} />);

    await screen.findByText("Device available");
    expect(service.OnStatusEvent).toHaveBeenCalledTimes(1);

    listeners[0]({ Battery: 55 });
    expect(await screen.findByText("Battery 55%")).toBeInTheDocument();

    unmount();
    expect(listeners.length).toBe(1);
  });

  it("hides the selector for one selected device and shows it when explicit selection is required", async () => {
    const alpha = { ID: { VendorID: 0x1D57, ProductID: 0xFA60, Serial: "alpha" }, Path: "/dev/hidraw0", Profile: "attack-shark-x6", Eligible: true };
    const bravo = { ID: { VendorID: 0x1D57, ProductID: 0xFA60, Serial: "bravo" }, Path: "/dev/hidraw1", Profile: "attack-shark-x6", Eligible: true };
    const oneDevice = {
      ...serviceFor(snapshot()),
      RefreshInventory: vi.fn().mockResolvedValue({ Devices: [alpha], Selected: alpha, Error: { Code: "" } }),
      SelectDevice: vi.fn(),
    } as unknown as DesktopService;
    const { rerender } = render(<App service={oneDevice} />);

    expect(await screen.findByText("Device available")).toBeInTheDocument();
    expect((oneDevice as unknown as { RefreshInventory: ReturnType<typeof vi.fn> }).RefreshInventory).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("combobox", { name: "Mouse device" })).not.toBeInTheDocument();

    const manyDevices = {
      ...serviceFor(snapshot()),
      RefreshInventory: vi.fn().mockResolvedValue({ Devices: [alpha, bravo], Selected: null, Error: { Code: "selection_required" } }),
      SelectDevice: vi.fn().mockResolvedValue({ Devices: [alpha, bravo], Selected: bravo, Error: { Code: "" } }),
    } as unknown as DesktopService;
    rerender(<App service={manyDevices} />);

    const selector = await screen.findByRole("combobox", { name: "Mouse device" });
    expect(screen.getByRole("alert")).toHaveTextContent("selection required");
    fireEvent.change(selector, { target: { value: "bravo" } });
    await waitFor(() => expect((manyDevices as unknown as { SelectDevice: ReturnType<typeof vi.fn> }).SelectDevice).toHaveBeenCalledWith(bravo.ID));
  });

  it("treats a selected session-only device as ready without showing ambiguity", async () => {
    const sessionOnly = {
      ID: { VendorID: 0x1D57, ProductID: 0xFA60, Serial: "session-1234" },
      Path: "/dev/hidraw3",
      Profile: "attack-shark-x6",
      Eligible: true,
      Warning: "session-only (no serial)",
      Connection: "dongle",
    };
    const selectedBinding: Binding = {
      ID: sessionOnly.ID,
      ProfileID: sessionOnly.Profile,
      Path: sessionOnly.Path,
      InventoryRevision: 1,
      SessionOnly: true,
    };
    const service = serviceFor(snapshot(), {
      RefreshInventory: vi.fn().mockResolvedValue({ Devices: [sessionOnly], Selected: selectedBinding, Error: { Code: "" } }),
    });
    render(<App service={service} />);

    expect(await screen.findByText("Device available")).toBeInTheDocument();
    expect(screen.queryByText(/ambiguous identity/)).not.toBeInTheDocument();
	expect(screen.getByRole("button", { name: /Reset to factory/ })).toBeEnabled();
    expect(screen.getAllByRole("button", { name: /Stage/ }).every((button) => !(button as HTMLButtonElement).disabled)).toBe(true);
    expect(screen.getAllByRole("slider").every((slider) => !(slider as HTMLInputElement).disabled)).toBe(true);
  });

  it("shows full eligible identity and actionable stale-binding recovery feedback", async () => {
    const alpha = { ID: { VendorID: 0x1D57, ProductID: 0xFA60, Serial: "alpha" }, Path: "/dev/hidraw0", Profile: "attack-shark-x6", Eligible: true };
    const unavailable = { ID: { VendorID: 0x1D57, ProductID: 0xFA60, Serial: "" }, Path: "/dev/hidraw2", Profile: "attack-shark-x6", Eligible: false, Warning: "ambiguous identity", Connection: "dongle" };
    const service = serviceFor(snapshot({ Error: { Code: "stale_binding" } }), {
      RefreshInventory: vi.fn().mockResolvedValue({ Devices: [alpha, unavailable], Selected: alpha, Error: { Code: "stale_binding" } }),
    });
    render(<App service={service} />);

    expect(await screen.findByText("VID 1D57 · PID FA60 · Serial alpha")).toBeInTheDocument();
    expect(screen.getByText("Connection: dongle")).toBeInTheDocument();
    expect(screen.getByText("ambiguous identity")).toBeInTheDocument();
    expect(screen.getByRole("alert")).toHaveTextContent("Device connection changed. Refresh the device list and select the mouse again before saving.");
  });

  it("shows an ineligible receiver without enabling configuration controls", async () => {
    const unavailable = { ID: { VendorID: 0x1D57, ProductID: 0xFA60, Serial: "" }, Path: "/dev/hidraw2", Profile: "attack-shark-x6", Eligible: false, Warning: "ambiguous identity", Connection: "dongle" };
    const service = serviceFor(snapshot(), {
      RefreshInventory: vi.fn().mockResolvedValue({ Devices: [unavailable], Selected: null, Error: { Code: "ambiguous_identity" } }),
    });
    render(<App service={service} />);

    expect(await screen.findByText("Receiver detected, configuration unavailable.")).toBeInTheDocument();
    expect(screen.getByText("VID 1D57 · PID FA60 · Serial unavailable")).toBeInTheDocument();
    expect(screen.getByText("Connection: dongle")).toBeInTheDocument();
    expect(screen.getAllByText("ambiguous identity")).toHaveLength(2);
	expect(screen.getByRole("button", { name: /Reset to factory/ })).toBeDisabled();
    expect(screen.getAllByRole("button", { name: /Stage/ }).every((button) => (button as HTMLButtonElement).disabled)).toBe(true);
    expect(screen.getAllByRole("slider").every((slider) => (slider as HTMLInputElement).disabled)).toBe(true);
    expect(service.StageDPI).not.toHaveBeenCalled();
  });

  it("applies status events only when their identity matches the selected device", async () => {
    const listeners: Array<(event: { ID: { VendorID: number; ProductID: number; Serial: string }; Path: string; InventoryRevision: number; Battery?: number }) => void> = [];
    const alpha = { ID: { VendorID: 0x1D57, ProductID: 0xFA60, Serial: "alpha" }, Path: "/dev/hidraw0", InventoryRevision: 1 };
    const service = serviceFor(snapshot(), {
      OnStatusEvent: vi.fn().mockImplementation((callback) => { listeners.push(callback); return () => {}; }),
    });
    const selectedService = {
      ...service,
      RefreshInventory: vi.fn().mockResolvedValue({ Devices: [alpha], Selected: alpha, Error: { Code: "" } }),
      SelectDevice: vi.fn(),
    } as unknown as DesktopService;
    render(<App service={selectedService} />);

    expect(await screen.findByText("Battery 84%")).toBeInTheDocument();
    await act(async () => {
      listeners[0]({ ID: { ...alpha.ID, Serial: "bravo" }, Path: "/dev/hidraw1", InventoryRevision: 1, Battery: 10 });
    });
    expect(screen.getByText("Battery 84%")).toBeInTheDocument();

    await act(async () => {
      listeners[0]({ ...alpha, Battery: 90 });
    });
    expect(await screen.findByText("Battery 90%")).toBeInTheDocument();
  });

  it("exposes only implemented controls and no deferred configuration", async () => {
    render(<App service={serviceFor(snapshot())} />);

    await screen.findByText("Device available");
    expect(screen.queryByRole("button", { name: /macro|profile|remap|lighting/i })).not.toBeInTheDocument();
	expect(screen.queryByRole("button", { name: /Save to Device/ })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Reset to factory/ })).toBeInTheDocument();
  });
});
