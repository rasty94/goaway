import { mockApi } from "@/test/api";
import { render, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";
import App from "./app";

vi.mock("@tsparticles/react", () => ({
  default: () => null,
  ParticlesProvider: ({ children }: { children: React.ReactNode }) => children
}));

const routes = [
  "/", "/logs", "/blacklist", "/whitelist", "/resolution", "/prefetch", "/upstream", "/clients",
  "/users", "/cluster", "/import/pihole", "/settings", "/changelog", "/login", "/does-not-exist"
];

// Every route renders inside the real app shell (layout, sidebar, header) against fixture data.
describe.each(routes)("route %s", (route) => {
  it("renders", async () => {
    mockApi();
    const { container } = render(
      <MemoryRouter initialEntries={[route]}>
        <App />
      </MemoryRouter>
    );
    await waitFor(() => expect(container.textContent).not.toBe(""));
    // let mount-time fetches resolve so their render paths run too
    await new Promise((r) => setTimeout(r, 100));
  });
});
