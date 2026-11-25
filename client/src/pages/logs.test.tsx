import { mockApi } from "@/test/api";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it } from "vitest";
import { Logs } from "./logs";

const query = (domain: string, blocked: boolean) => ({
  domain,
  blocked,
  cached: false,
  client: { ip: "192.0.2.10", name: "laptop", mac: "" },
  ip: [{ ip: "93.184.216.34", rtype: "A" }],
  queryType: "A",
  responseTimeNS: 2_000_000,
  status: blocked ? "Blacklisted" : "NOERROR",
  timestamp: "2026-11-25T08:00:00Z",
  protocol: "UDP"
});

describe("Logs", () => {
  it("renders query rows from the API and sends the domain filter", async () => {
    const { calls } = mockApi({
      queries: { queries: [query("allowed.example", false), query("ads.example", true)], draw: "1", recordsFiltered: 2, recordsTotal: 2 }
    });
    const user = userEvent.setup();
    render(
      <MemoryRouter>
        <Logs />
      </MemoryRouter>
    );

    expect(await screen.findByText("allowed.example")).toBeInTheDocument();
    expect(screen.getByText("ads.example")).toBeInTheDocument();
    expect(calls.some((c) => c.path.startsWith("queries?page=1"))).toBe(true);

    const search = screen.getAllByRole("textbox")[0];
    await user.type(search, "ads");
    await waitFor(() => expect(calls.some((c) => c.path.includes("search=ads"))).toBe(true));
  });

  it("opens client details from a row and shows domains and history", async () => {
    const details = {
      allDomains: { "allowed.example": 7, "cdn.example": 3 },
      avgResponseTimeMs: 4.2,
      blockedRequests: 1,
      cachedRequests: 2,
      mostQueriedDomain: "allowed.example",
      totalRequests: 10,
      uniqueDomains: 2,
      clientInfo: { name: "laptop", ip: "192.0.2.10", mac: "aa:bb:cc:dd:ee:ff", vendor: "Acme", lastSeen: "2026-11-25T07:59:00Z", bypass: false }
    };
    const { calls } = mockApi({
      queries: { queries: [query("allowed.example", false)], draw: "1", recordsFiltered: 1, recordsTotal: 1 },
      "client/192.0.2.10/details": details,
      "client/192.0.2.10/history": { history: [{ domain: "cdn.example", timestamp: "2026-11-25T07:58:00Z" }] }
    });
    const user = userEvent.setup();
    render(
      <MemoryRouter>
        <Logs />
      </MemoryRouter>
    );

    await user.click(await screen.findByText("laptop | 192.0.2.10"));
    expect(await screen.findByText("Top Queried Domains")).toBeInTheDocument();
    expect(screen.getAllByText("allowed.example").length).toBeGreaterThan(1);
    expect(calls.some((c) => c.path === "client/192.0.2.10/history")).toBe(true);

    await user.click(screen.getByRole("tab", { name: "Domains" }));
    expect(await screen.findByText("All Queried Domains")).toBeInTheDocument();
    await user.click(screen.getByRole("tab", { name: "History" }));
    expect(await screen.findAllByText("cdn.example")).not.toHaveLength(0);
  });
});
