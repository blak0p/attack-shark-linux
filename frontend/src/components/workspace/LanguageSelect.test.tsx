import "@testing-library/jest-dom/vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { setLanguage } from "../../i18n";
import { LanguageSelect } from "./LanguageSelect";

describe("LanguageSelect", () => {
  beforeEach(async () => {
    window.localStorage.clear();
    await setLanguage("en");
  });
  afterEach(cleanup);

  it("lists English and Spanish and shows the active language", () => {
    render(<LanguageSelect />);
    const select = screen.getByRole("combobox", { name: "Language" });
    expect(select).toHaveValue("en");
    expect(screen.getByRole("option", { name: "English" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "Español" })).toBeInTheDocument();
  });

  it("switches the interface language and persists it", async () => {
    render(<LanguageSelect />);
    fireEvent.change(screen.getByRole("combobox", { name: "Language" }), { target: { value: "es" } });
    expect(await screen.findByRole("combobox", { name: "Idioma" })).toHaveValue("es");
    expect(window.localStorage.getItem("x6.language")).toBe("es");
  });
});
