import { describe, expect, it, vi } from "vitest";

const bindings = vi.hoisted(() => ({
  GetSnapshot: vi.fn(),
  RefreshStatus: vi.fn(),
  StageDPI: vi.fn(),
  ApplyDPI: vi.fn(),
}));

const runtime = vi.hoisted(() => ({ Events: { On: vi.fn().mockReturnValue(() => {}) } }));

vi.mock("../bindings/github.com/alejandro/attack-shark-linux/internal/desktop/service", () => bindings);
vi.mock("@wailsio/runtime", () => runtime);

import { desktopService } from "./wails-service";

describe("desktopService", () => {
  it("forwards each UI operation to its generated Wails binding", () => {
    const config = { DPI: [1600], ActiveStage: 0, StageMask: 1, LiftDistance: 1 };

    expect(desktopService.GetSnapshot).toBe(bindings.GetSnapshot);
    expect(desktopService.RefreshStatus).toBe(bindings.RefreshStatus);
    expect(desktopService.StageDPI).toBe(bindings.StageDPI);
    expect(desktopService.ApplyDPI).toBe(bindings.ApplyDPI);
    desktopService.StageDPI(config);
    expect(bindings.StageDPI).toHaveBeenCalledWith(config);
  });

  it("subscribes to the x6:status event through the Wails runtime", () => {
    const callback = vi.fn();
    const unsubscribe = desktopService.OnStatusEvent(callback);

    expect(runtime.Events.On).toHaveBeenCalledWith("x6:status", expect.any(Function));
    const handler = runtime.Events.On.mock.calls[0][1];
    handler({ data: { Connection: "dongle", Battery: 90 } });
    expect(callback).toHaveBeenCalledWith({ Connection: "dongle", Battery: 90 });

    expect(unsubscribe).toBe(runtime.Events.On.mock.results[0].value);
  });
});
