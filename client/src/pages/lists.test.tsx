import { mockApi } from "@/test/api";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it } from "vitest";
import { Blacklist } from "./blacklist";
import { Whitelist } from "./whitelist";

describe("Blacklist", () => {
  it("lists subscriptions and sums blocked domains of active lists only", async () => {
    mockApi({
      lists: {
        "Zeta list": { url: "https://z.example/list.txt", active: true, blockedCount: 1500, lastUpdated: 1700000000 },
        "Alpha list": { url: "https://a.example/list.txt", active: false, blockedCount: 900, lastUpdated: 1700000000 }
      }
    });
    const { container } = render(
      <MemoryRouter>
        <Blacklist />
      </MemoryRouter>
    );
    expect(await screen.findByText("Zeta list")).toBeInTheDocument();
    expect(screen.getByText("Alpha list")).toBeInTheDocument();
    expect(container.textContent).toMatch(/1,?500/);
    expect(container.textContent).not.toMatch(/2,?400/);
  });
});

describe("Whitelist", () => {
  it("loads, adds and deletes domains", async () => {
    const { calls } = mockApi({ whitelist: ["keep.example"], "POST whitelist": {}, "DELETE whitelist": {} });
    const user = userEvent.setup();
    render(<Whitelist />);

    expect(await screen.findByText("keep.example")).toBeInTheDocument();

    await user.type(screen.getByPlaceholderText("example.com"), "new.example");
    await user.click(screen.getByRole("button", { name: "Add" }));
    expect(await screen.findByText("new.example")).toBeInTheDocument();
    expect(calls.find((c) => c.method === "POST")?.body).toMatchObject({ domain: "new.example" });

    const row = screen.getByText("keep.example").closest("tr, li, div") as HTMLElement;
    await user.click(within(row).getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(screen.queryByText("keep.example")).not.toBeInTheDocument());
    expect(calls.some((c) => c.method === "DELETE")).toBe(true);
  });

  it("filters by search term", async () => {
    mockApi({ whitelist: ["one.example", "two.example"] });
    const user = userEvent.setup();
    render(<Whitelist />);
    await screen.findByText("one.example");
    await user.type(screen.getByPlaceholderText("Search domains..."), "two");
    expect(screen.queryByText("one.example")).not.toBeInTheDocument();
    expect(screen.getByText("two.example")).toBeInTheDocument();
  });
});

describe("Blocklist details", () => {
  it("toggles, checks for updates, applies the update and deletes the list", async () => {
    const { calls } = mockApi({
      lists: { Ads: { url: "https://ads.example/hosts", active: true, blockedCount: 10, lastUpdated: 1764057600 } },
      toggleBlocklist: { message: "toggled" },
      fetchUpdatedList: { updateAvailable: true, diffAdded: ["new.ads.example"], diffRemoved: ["old.ads.example"] },
      runUpdateList: {},
      "DELETE list": {}
    });
    const user = userEvent.setup();
    render(
      <MemoryRouter>
        <Blacklist />
      </MemoryRouter>
    );

    await user.click(await screen.findByRole("button", { name: "View Details" }));
    await user.click(await screen.findByRole("button", { name: /Toggle/ }));
    await waitFor(() => expect(calls.some((c) => c.path === "toggleBlocklist?blocklist=Ads")).toBe(true));

    await user.click(screen.getByRole("button", { name: /^Update$/ }));
    expect(await screen.findByText("new.ads.example")).toBeInTheDocument();
    expect(screen.getByText("old.ads.example")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /Delete/ }));
    await waitFor(() => expect(calls.some((c) => c.method === "DELETE" && c.path.startsWith("list?name=Ads"))).toBe(true));
  });
});
