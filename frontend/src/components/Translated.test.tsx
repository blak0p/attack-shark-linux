import "@testing-library/jest-dom/vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { setLanguage } from "../i18n";
import { SectionNavigation } from "./workspace/SectionNavigation";
import { UnifiedUpdates } from "./UnifiedUpdates";
import { UpdateBanner } from "./UpdateBanner";

describe("Spanish rendering", () => {
  beforeEach(async () => { window.localStorage.clear(); await setLanguage("es"); });
  afterEach(async () => { cleanup(); await setLanguage("en"); });

  it("translates the update banner with interpolation", () => {
    render(<UpdateBanner update={{ Version: "1.2.3" } as never} applying={false} error="boom" onApply={() => {}} />);
    expect(screen.getByText("La versión 1.2.3 está lista.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Actualizar y reiniciar" })).toBeInTheDocument();
    expect(screen.getByRole("alert")).toHaveTextContent("Falló la actualización: boom");
  });

  it("translates the section navigation", () => {
    render(<SectionNavigation activeView="performance" onNavigate={() => {}} />);
    expect(screen.getByRole("navigation", { name: "Secciones del espacio de trabajo" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Rendimiento/ })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Reasignación de botones/ })).toBeInTheDocument();
  });

  it("translates the updates panel", () => {
    render(<UnifiedUpdates checking applying={false} error="x" onCheck={() => {}} onApply={() => {}} />);
    expect(screen.getByRole("heading", { name: "Actualizaciones" })).toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent("Buscando actualizaciones…");
  });
});

describe("Spanish rendering of panels", () => {
  beforeEach(async () => { window.localStorage.clear(); await setLanguage("es"); });
  afterEach(async () => { cleanup(); await setLanguage("en"); });

  it("pluralizes the sleep duration", async () => {
    const { SleepPanel } = await import("./panels/SleepPanel");
    const snapshot = (Pending: number) => ({ Pending, Applied: Pending, Firmware: "success", Persistence: "success", Error: { Code: "" } }) as never;
    const { rerender } = render(<SleepPanel snapshot={snapshot(1)} ready onStage={() => {}} onRetry={() => {}} />);
    expect(screen.getByText("Suspender después de: 1 minuto")).toBeInTheDocument();
    rerender(<SleepPanel snapshot={snapshot(5)} ready onStage={() => {}} onRetry={() => {}} />);
    expect(screen.getByText("Suspender después de: 5 minutos")).toBeInTheDocument();
  });

  it("translates the reset confirmation", async () => {
    const { ResetPanel } = await import("./panels/ResetPanel");
    render(<ResetPanel confirmation onConfirm={() => {}} onCancel={() => {}} />);
    expect(screen.getByRole("alertdialog", { name: "Confirmar restablecimiento de fábrica" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Cancelar restablecimiento de fábrica" })).toBeInTheDocument();
  });
});

describe("Spanish rendering of the remap panel", () => {
  beforeEach(async () => { window.localStorage.clear(); await setLanguage("es"); });
  afterEach(async () => { cleanup(); await setLanguage("en"); });

  it("translates headings, action labels, summary and status", async () => {
    const { ButtonRemapPanel } = await import("./panels/ButtonRemapPanel");
    const buttons = [{ Button: 1, Action: "left", PreservedDefault: "" }, { Button: 2, Action: null, PreservedDefault: "" }];
    const remap = { Pending: { Buttons: buttons }, Applied: { Buttons: buttons }, Actions: ["left", "mute"], Firmware: "success", Error: { Code: "" } } as never;
    render(<ButtonRemapPanel remap={remap} ready onStage={() => {}} />);
    expect(screen.getByRole("heading", { name: "Reasignación de botones" })).toBeInTheDocument();
    expect(screen.getByText("Botón 1")).toBeInTheDocument();
    expect(screen.getAllByText("Izquierdo").length).toBeGreaterThan(0);
    expect(screen.getByLabelText("Resumen de asignaciones de botones")).toHaveTextContent("Botón 1: Izquierdo, Botón 2: Predeterminado");
    expect(screen.getByRole("status", { name: "Estado de la reasignación de botones" })).toHaveTextContent("Reasignación de botones aplicada");
  });
});
