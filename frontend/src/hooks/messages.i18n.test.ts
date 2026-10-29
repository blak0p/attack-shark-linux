import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { setLanguage } from "../i18n";
import { deviceAdmission } from "../macros/device-admission";
import { importMacroFile } from "../macros/macro-file";
import { useMacroLibrary } from "./useMacroLibrary";

describe("hook and validator messages in Spanish", () => {
  beforeEach(async () => { await setLanguage("es"); });
  afterEach(async () => { await setLanguage("en"); });

  it("translates device admission reasons", () => {
    expect(deviceAdmission([], 1).reason).toBe("La asignación requiere de 1 a 46 eventos de origen.");
    expect(deviceAdmission([{ type: "mouse_left", action: "down", delay_ms: 0 }], 0).reason).toBe("La repetición debe ser de 1 a 255.");
    const ok = deviceAdmission([{ type: "mouse_left", action: "down", delay_ms: 0 }, { type: "mouse_left", action: "up", delay_ms: 0 }]);
    expect(ok.reason).toBe("Apta para asignar después de guardar. La reproducción sigue sin verificarse.");
  });

  it("translates macro file failures", () => {
    expect(() => importMacroFile("{")).toThrow("JSON de macro inválido: está malformado o tiene datos al final.");
    expect(() => importMacroFile(JSON.stringify({ version: 2, name: "a", events: [] }))).toThrow("Versión de archivo de macro no admitida; se espera la versión 1.");
    expect(() => importMacroFile(JSON.stringify({ version: 1, name: "a", events: [{ type: "x", action: "down", delay_ms: 0 }] }))).toThrow("Evento 1 tiene un tipo no admitido.");
  });

  it("translates the macro library notice and validation error", async () => {
    const macro = { id: "m1", name: "a", events: [] };
    const service = {
      ListMacros: vi.fn().mockResolvedValue([macro]), ReadMacro: vi.fn().mockResolvedValue(macro),
      CreateMacro: vi.fn().mockResolvedValue(macro), UpdateMacro: vi.fn().mockResolvedValue(macro), DeleteMacro: vi.fn(),
    } as never;
    const { result } = renderHook(() => useMacroLibrary(service));
    await waitFor(() => expect(result.current.loaded).toBe(true));
    await act(async () => result.current.select("m1"));
    await waitFor(() => expect(result.current.draft).toBeDefined());
    await act(async () => result.current.save());
    await waitFor(() => expect(result.current.notice).toBe("Guardado en la biblioteca. No se hicieron cambios en el dispositivo."));
    act(() => result.current.addEvent());
    act(() => result.current.updateEvent(0, { delay_ms: "-1" }));
    expect(result.current.validationError).toBe("La demora del evento 1 debe ser un número entero no negativo de milisegundos.");
  });
});
