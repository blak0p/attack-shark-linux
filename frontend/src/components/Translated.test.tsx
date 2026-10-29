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
