import "@testing-library/jest-dom/vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { App, type DesktopService, type Snapshot } from "./App";

afterEach(cleanup);

const configuration = (firstDPI = 800) => ({
  DPI: [firstDPI, 1200, 1600, 2400, 3200, 6400, 12800, 26000],
  ActiveStage: 0,
  StageMask: 255,
  LiftDistance: 1,
});

const snapshot = (overrides: Partial<Snapshot> = {}): Snapshot => ({
  Connection: "dongle",
  Battery: 84,
  Applied: configuration(),
  Pending: configuration(),
  Revision: 0,
  Error: { Code: "" },
  ...overrides,
});

const serviceFor = (initial: Snapshot, overrides: Partial<DesktopService> = {}): DesktopService => ({
  GetSnapshot: vi.fn().mockResolvedValue(initial),
  RefreshStatus: vi.fn().mockResolvedValue(initial),
  StageDPI: vi.fn().mockImplementation(async (next) => ({ ...initial, Pending: next, Revision: initial.Revision + 1 })),
  ApplyDPI: vi.fn().mockResolvedValue(initial),
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

  it("stages a DPI edit locally without applying it", async () => {
    const service = serviceFor(snapshot());
    render(<App service={service} />);

    const input = await screen.findByRole("spinbutton", { name: "Stage 1 DPI" });
    fireEvent.change(input, { target: { value: "1600" } });

    await waitFor(() => expect(service.StageDPI).toHaveBeenCalledWith(expect.objectContaining({ DPI: expect.arrayContaining([1600]) })));
    expect(service.ApplyDPI).not.toHaveBeenCalled();
    expect(screen.getByText("Changes are staged locally. Select Apply to send them.")).toBeInTheDocument();
  });

  it("reports a successful Apply only after the service returns success", async () => {
    const service = serviceFor(snapshot(), { ApplyDPI: vi.fn().mockResolvedValue(snapshot({ Applied: configuration(1600), Pending: configuration(1600) })) });
    render(<App service={service} />);

    fireEvent.click(await screen.findByRole("button", { name: "Apply DPI" }));

    expect(await screen.findByText("DPI configuration applied and saved.")).toBeInTheDocument();
    expect(service.ApplyDPI).toHaveBeenCalledTimes(1);
  });

  it("preserves staged values and reports an unsuccessful Apply", async () => {
    const failed = snapshot({ Applied: configuration(), Pending: configuration(1600), Error: { Code: "apply_failed" } });
    const service = serviceFor(snapshot({ Pending: configuration(1600) }), { ApplyDPI: vi.fn().mockResolvedValue(failed) });
    render(<App service={service} />);

    fireEvent.click(await screen.findByRole("button", { name: "Apply DPI" }));

    expect(await screen.findByText("Apply failed. Your staged values are still available.")).toBeInTheDocument();
    expect(screen.getByRole("spinbutton", { name: "Stage 1 DPI" })).toHaveValue(1600);
  });

  it("restores the last applied DPI state when the desktop service restarts", async () => {
    const restored = snapshot({ Applied: configuration(1600), Pending: configuration(1600), Revision: 2 });
    render(<App service={serviceFor(restored)} />);

    expect(await screen.findByRole("spinbutton", { name: "Stage 1 DPI" })).toHaveValue(1600);
    expect(screen.getByRole("button", { name: "Apply DPI" })).toBeEnabled();
  });

  it("does not expose reset or deferred configuration controls", async () => {
    render(<App service={serviceFor(snapshot())} />);

    await screen.findByText("Device available");
    expect(screen.queryByRole("button", { name: /reset|macro|profile|remap|lighting/i })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Apply DPI" })).toBeEnabled();
  });
});
