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

  it("shows the active language and lists English and Spanish in the themed menu", () => {
    render(<LanguageSelect />);
    const trigger = screen.getByRole("button", { name: "Language" });
    expect(trigger).toHaveTextContent("English");
    fireEvent.click(trigger);
    expect(screen.getByRole("menuitem", { name: "English" })).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: "Español" })).toBeInTheDocument();
  });

  it("switches the interface language and persists it", async () => {
    render(<LanguageSelect />);
    fireEvent.click(screen.getByRole("button", { name: "Language" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Español" }));
    const trigger = await screen.findByRole("button", { name: "Idioma" });
    expect(trigger).toHaveTextContent("Español");
    expect(window.localStorage.getItem("x6.language")).toBe("es");
  });
});
