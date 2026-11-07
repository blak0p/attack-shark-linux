import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import "../../i18n";
import { DeviceStatusPanel } from "./DeviceStatusPanel";

afterEach(cleanup);

describe("DeviceStatusPanel install source", () => {
  it("shows only the version and no updates row for non-packaged builds", () => {
    render(<DeviceStatusPanel applicationVersion="1.8.0" />);
    expect(screen.getByText("1.8.0")).toBeTruthy();
    expect(screen.queryByText("Updates")).toBeNull();
  });

  it("marks a stable AUR build and names the package manager that updates it", () => {
    render(<DeviceStatusPanel applicationVersion="1.8.0" installSource={{ PackageManager: "pacman", Distribution: "AUR", Branch: "", Commit: "" }} />);
    expect(screen.getByText("1.8.0 · AUR")).toBeTruthy();
    expect(screen.getByText("Updates")).toBeTruthy();
    expect(screen.getByText("Managed by pacman")).toBeTruthy();
  });

  it("shows the branch and commit instead of the version for a -git build", () => {
    render(<DeviceStatusPanel applicationVersion="1.8.0.r1.g2bf7932" installSource={{ PackageManager: "pacman", Distribution: "AUR", Branch: "main", Commit: "2bf7932" }} />);
    expect(screen.getByText("main · 2bf7932 · AUR")).toBeTruthy();
    expect(screen.queryByText(/1\.8\.0/)).toBeNull();
    expect(screen.getByText("Managed by pacman")).toBeTruthy();
  });
});
