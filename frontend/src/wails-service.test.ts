import { describe, expect, it, vi } from "vitest";

const bindings = vi.hoisted(() => ({
  GetSnapshot: vi.fn(),
  RefreshStatus: vi.fn(),
  StageDPI: vi.fn(),
  ApplyDPI: vi.fn(),
}));

vi.mock("../bindings/github.com/alejandro/attack-shark-linux/internal/desktop/service", () => bindings);

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
});
