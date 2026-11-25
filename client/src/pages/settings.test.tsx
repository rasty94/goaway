import { mockApi } from "@/test/api";
import { settings } from "@/test/fixtures";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { Toaster } from "sonner";
import { describe, expect, it, vi } from "vitest";
import { Settings } from "./settings";

describe("Settings", () => {
  it("loads settings, flags unsaved changes and saves them", async () => {
    const { calls } = mockApi({ settings: { ...settings, dns: { ...settings.dns, cacheTTL: 1234 } }, "POST settings": {} });
    const user = userEvent.setup();
    render(
      <MemoryRouter>
        <Settings />
        <Toaster />
      </MemoryRouter>
    );

    const ttl = await screen.findByDisplayValue("1234");
    fireEvent.change(ttl, { target: { value: "600" } });

    await user.click(await screen.findByRole("button", { name: "Save" }));
    await waitFor(() => expect(calls.some((c) => c.method === "POST" && c.path === "settings")).toBe(true));
    const saved = calls.find((c) => c.method === "POST" && c.path === "settings")!.body as typeof settings;
    expect(saved.dns.cacheTTL).toBe(600);
  });

  it("validates and changes the password, then returns to login", async () => {
    const { calls } = mockApi({ "PUT password": {} });
    const user = userEvent.setup();
    render(
      <MemoryRouter initialEntries={["/settings"]}>
        <Routes>
          <Route path="/settings" element={<Settings />} />
          <Route path="/login" element={<p>login page</p>} />
        </Routes>
      </MemoryRouter>
    );

    await user.click(await screen.findByRole("button", { name: "Change Password" }));
    await user.type(screen.getByPlaceholderText("Enter current password"), "old");
    await user.type(screen.getByPlaceholderText("Enter new password"), "new-pass");
    await user.type(screen.getByPlaceholderText("Enter confirm password"), "typo");
    await user.click(screen.getByRole("button", { name: "Update" }));
    expect(await screen.findByText("Passwords don't match")).toBeInTheDocument();

    await user.clear(screen.getByPlaceholderText("Enter confirm password"));
    await user.type(screen.getByPlaceholderText("Enter confirm password"), "new-pass");
    await user.click(screen.getByRole("button", { name: "Update" }));
    expect(await screen.findByText("login page")).toBeInTheDocument();
    expect(calls.find((c) => c.path === "password")?.body).toEqual({ currentPassword: "old", newPassword: "new-pass" });
  });

  it("exports the database as a download", async () => {
    mockApi({ exportDatabase: () => [200, "SQLite format 3"] });
    URL.createObjectURL = vi.fn(() => "blob:db");
    URL.revokeObjectURL = vi.fn();
    const click = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => {});
    const user = userEvent.setup();
    render(
      <MemoryRouter>
        <Settings />
      </MemoryRouter>
    );

    await screen.findByText("Export database");
    const exportRow = screen.getByText("Export database").closest("div")!.parentElement!;
    await user.click(within(exportRow).getByRole("button"));
    await waitFor(() => expect(click).toHaveBeenCalled());
    expect(URL.createObjectURL).toHaveBeenCalled();
  });
});
