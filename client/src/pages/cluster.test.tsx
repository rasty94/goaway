import { apiDefaults } from "@/test/fixtures";
import { mockApi } from "@/test/api";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { toast } from "sonner";
import { describe, expect, it, vi } from "vitest";
import { Cluster } from "./cluster";

const node = (apiDefaults["ha/cluster"] as { nodes: Record<string, unknown>[] }).nodes[0];

const follower = {
  selfRole: "follower",
  activeNodes: 2,
  clusterId: "c1",
  nodes: [
    node,
    { ...node, id: "n2", address: "10.0.0.2", role: "follower", unreachable: true, needsResync: true },
    { ...node, id: "n3", address: "10.0.0.3", role: "follower", pendingEvents: 7 }
  ],
  proxyStats: { totalRequests: 200, errorRequests: 3, nodeRequests: { "10.0.0.1": 150, "10.0.0.2": 50 } }
};

describe("Cluster", () => {
  it("shows a primary node without resync controls", async () => {
    mockApi();
    render(<Cluster />);
    expect(await screen.findByText("Leader")).toBeInTheDocument();
    expect(screen.getByText("10.0.0.1")).toBeInTheDocument();
    expect(screen.getByText("Up to date")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Hot Resync/ })).not.toBeInTheDocument();
  });

  it("shows follower state, offline peers, queued events and proxy stats", async () => {
    mockApi({ "ha/cluster": follower });
    render(<Cluster />);
    expect(await screen.findByText("Follower")).toBeInTheDocument();
    expect(screen.getByText("Offline")).toBeInTheDocument();
    expect(screen.getByText("Out of sync")).toBeInTheDocument();
    expect(screen.getByText("7 queued")).toBeInTheDocument();
    expect(screen.getByText("150 q (75%)")).toBeInTheDocument();
  });

  it("resyncs from one out-of-sync peer and reports what was applied", async () => {
    const success = vi.spyOn(toast, "success");
    const { calls } = mockApi({
      "ha/cluster": follower,
      "POST ha/resync": { applied: { whitelistAdded: 2, resolutionsAdded: 1, staticLeasesAdded: 0 } }
    });
    const user = userEvent.setup();
    render(<Cluster />);

    await user.click(await screen.findByRole("button", { name: "Resync" }));
    await waitFor(() =>
      expect(success).toHaveBeenCalledWith("Logical hot resync completed", {
        description: "Whitelist +2, Resolutions +1, Leases +0"
      })
    );
    expect(calls.find((c) => c.path === "ha/resync")?.body).toEqual({ peer: "10.0.0.2" });
  });

  it("reports a failed hot resync", async () => {
    const error = vi.spyOn(toast, "error");
    mockApi({ "ha/cluster": follower, "POST ha/resync": () => [409, { error: "primary unreachable" }] });
    const user = userEvent.setup();
    render(<Cluster />);

    await user.click(await screen.findByRole("button", { name: /Hot Resync/ }));
    await waitFor(() => expect(error).toHaveBeenCalledWith("primary unreachable"));
  });

  it("reports when the cluster status cannot be loaded", async () => {
    const error = vi.spyOn(toast, "error");
    mockApi({ "ha/cluster": () => [500, { error: "boom" }] });
    render(<Cluster />);
    await waitFor(() => expect(error).toHaveBeenCalledWith("Failed to fetch cluster status"));
    expect(await screen.findByText("No discovered peers for this cluster.")).toBeInTheDocument();
  });
});
