import { mockApi } from "@/test/api";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import Login from "./login";

// The particles canvas needs WebGL/canvas, which jsdom lacks.
vi.mock("@tsparticles/react", () => ({
  default: () => null,
  ParticlesProvider: ({ children }: { children: React.ReactNode }) => children
}));

function renderLogin() {
  return render(
    <MemoryRouter initialEntries={["/login"]}>
      <Routes>
        <Route path="/login" element={<Login quote="test quote" />} />
        <Route path="/" element={<p>home page</p>} />
      </Routes>
    </MemoryRouter>
  );
}

async function fillAndSubmit(user = userEvent.setup()) {
  await user.type(screen.getByLabelText("Username"), "admin");
  await user.type(screen.getByLabelText("Password"), "secret");
  await user.click(screen.getByRole("button", { name: "Sign In" }));
  return user;
}

describe("Login", () => {
  beforeEach(() => localStorage.clear());

  it("shows server version and hides SSO when OIDC is disabled", async () => {
    mockApi({ server: { version: "1.2.3", date: "2026-01-01" }, "auth/oidc/status": { enabled: false } });
    renderLogin();
    expect(await screen.findByText(/Version 1.2.3/)).toBeInTheDocument();
    expect(screen.queryByText(/Sign in with SSO/)).not.toBeInTheDocument();
  });

  it("offers SSO when OIDC is enabled", async () => {
    mockApi({ "auth/oidc/status": { enabled: true } });
    renderLogin();
    expect(await screen.findByRole("button", { name: /Sign in with SSO/ })).toBeInTheDocument();
  });

  it("logs in, stores the role and navigates home", async () => {
    const { calls } = mockApi({ "POST login": { role: "admin" } });
    renderLogin();
    await fillAndSubmit();
    expect(await screen.findByText("home page")).toBeInTheDocument();
    expect(localStorage.getItem("userRole")).toBe("admin");
    expect(calls.find((c) => c.path === "login")?.body).toEqual({ username: "admin", password: "secret" });
  });

  it("asks for a TOTP code when 2FA is required and sends it on retry", async () => {
    const { calls } = mockApi({
      "POST login": (body) =>
        (body as { totpCode?: string }).totpCode === "123456"
          ? [200, { role: "viewer" }]
          : [401, { requires2FA: true }]
    });
    renderLogin();
    const user = await fillAndSubmit();

    const code = await screen.findByLabelText(/2FA Code/);
    await user.type(code, "123456");
    await user.click(screen.getByRole("button", { name: "Sign In" }));

    expect(await screen.findByText("home page")).toBeInTheDocument();
    expect(calls.filter((c) => c.path === "login").at(-1)?.body).toMatchObject({ totpCode: "123456" });
  });

  it("stays on the page on invalid credentials", async () => {
    mockApi({ "POST login": () => [401, { error: "Invalid credentials" }] });
    renderLogin();
    await fillAndSubmit();
    await waitFor(() => expect(screen.getByRole("button", { name: "Sign In" })).toBeEnabled());
    expect(screen.queryByText("home page")).not.toBeInTheDocument();
  });

  it("prefills a remembered username", async () => {
    localStorage.setItem("loginUsername", "alice");
    mockApi();
    renderLogin();
    expect(screen.getByLabelText("Username")).toHaveValue("alice");
    expect(screen.getByRole("checkbox")).toBeChecked();
  });
});
