import "@testing-library/jest-dom/vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { App, type DesktopService, type Snapshot } from "./App";

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

const serviceFor = (initial: Snapshot, overrides: Partial<DesktopService> = {}): DesktopService => ({
  GetSnapshot: vi.fn().mockResolvedValue(initial),
  RefreshStatus: vi.fn().mockResolvedValue(initial),
  StageDPI: vi.fn().mockImplementation(async (next) => ({ ...initial, Pending: next, Revision: initial.Revision + 1 })),
  ApplyDPI: vi.fn().mockResolvedValue(initial),
  OnStatusEvent: vi.fn().mockReturnValue(() => {}),
  ...overrides,
});

describe("App", () => {
  it("shows available connection and supplied battery information", async () => {
    render(<App service={serviceFor(snapshot())} />);

    expect(await screen.findByText("Device available")).toBeInTheDocument();
    expect(screen.getByText("Battery 84%")).toBeInTheDocument();
  });

	it("shows unavailable status without inventing a battery value", async () => {
		render(<App service={serviceFor(snapshot({ Connection: "", Battery: undefined, Error: { Code: "device_unavailable" } }))} />);

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
    expect(service.ApplyDPI).not.toHaveBeenCalled();
    expect(screen.getByText("Changes are staged locally. Select Save to Device to send them.")).toBeInTheDocument();
  });

  it("stages factory defaults on Reset without applying them", async () => {
    const service = serviceFor(snapshot());
    render(<App service={service} />);

    fireEvent.click(await screen.findByRole("button", { name: /Reset to factory/ }));

    await waitFor(() => expect(service.StageDPI).toHaveBeenCalledWith(snapshot().Factory));
    expect(service.ApplyDPI).not.toHaveBeenCalled();
    expect(screen.getByText("Factory defaults staged. Select Save to Device to send them.")).toBeInTheDocument();
  });

  it("stages a DPI edit locally without applying it", async () => {
    const service = serviceFor(snapshot());
    render(<App service={service} />);

    const input = await screen.findByRole("slider", { name: "Stage 1 DPI" });
    fireEvent.change(input, { target: { value: "1600" } });

    await waitFor(() => expect(service.StageDPI).toHaveBeenCalledWith(expect.objectContaining({ DPI: expect.arrayContaining([1600]) })));
    expect(service.ApplyDPI).not.toHaveBeenCalled();
    expect(screen.getByText("Changes are staged locally. Select Save to Device to send them.")).toBeInTheDocument();
  });

  it("reports a successful Save only after the service returns success", async () => {
    const service = serviceFor(snapshot(), { ApplyDPI: vi.fn().mockResolvedValue(snapshot({ Applied: configuration(1600), Pending: configuration(1600) })) });
    render(<App service={service} />);

    fireEvent.click(await screen.findByRole("button", { name: /Save to Device/ }));

    expect(await screen.findByText("DPI configuration applied and saved.")).toBeInTheDocument();
    expect(service.ApplyDPI).toHaveBeenCalledTimes(1);
  });

  it("preserves staged values and reports an unsuccessful Save", async () => {
    const failed = snapshot({ Applied: configuration(), Pending: configuration(1600), Error: { Code: "apply_failed" } });
    const service = serviceFor(snapshot({ Pending: configuration(1600) }), { ApplyDPI: vi.fn().mockResolvedValue(failed) });
    render(<App service={service} />);

    fireEvent.click(await screen.findByRole("button", { name: /Save to Device/ }));

    expect(await screen.findByText("Save failed. Your staged values are still available.")).toBeInTheDocument();
    expect(screen.getByRole("slider", { name: "Stage 1 DPI" })).toHaveValue("1600");
  });

  it("restores the last applied DPI state when the desktop service restarts", async () => {
    const restored = snapshot({ Applied: configuration(1600), Pending: configuration(1600), Revision: 2 });
    render(<App service={serviceFor(restored)} />);

    expect(await screen.findByRole("slider", { name: "Stage 1 DPI" })).toHaveValue("1600");
    expect(screen.getByRole("button", { name: /Save to Device/ })).toBeEnabled();
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
    expect(await screen.findByText("Device unavailable")).toBeInTheDocument();
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

  it("exposes only implemented controls and no deferred configuration", async () => {
    render(<App service={serviceFor(snapshot())} />);

    await screen.findByText("Device available");
    expect(screen.queryByRole("button", { name: /macro|profile|remap|lighting/i })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Save to Device/ })).toBeEnabled();
    expect(screen.getByRole("button", { name: /Reset to factory/ })).toBeInTheDocument();
  });
});
