import { mockApi } from "@/test/api";
import { settings } from "@/test/fixtures";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { DHCPSection } from "./DHCPSection";

describe("DHCP section", () => {
  it("shows leases, stops the server and creates, edits and deletes static leases", async () => {
    vi.stubGlobal("confirm", () => true);
    const { calls } = mockApi({
      "POST dhcp/stop": {},
      "POST dhcp/leases": {},
      "PUT dhcp/leases/1": {},
      "DELETE dhcp/leases/1": {}
    });
    const onSaveConfig = vi.fn().mockResolvedValue(undefined);
    const user = userEvent.setup();
    render(<DHCPSection dhcp={settings.dhcp} onDhcpChange={vi.fn()} onSaveConfig={onSaveConfig} />);

    expect(await screen.findByText("printer")).toBeInTheDocument();
    expect(screen.getByText("tablet")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Save DHCP settings" }));
    expect(onSaveConfig).toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "Stop server" }));
    await waitFor(() => expect(calls.some((c) => c.path === "dhcp/stop")).toBe(true));

    await user.type(screen.getByPlaceholderText("aa:bb:cc:dd:ee:ff"), "aa:bb:cc:dd:ee:09");
    await user.type(screen.getByPlaceholderText("192.168.0.10"), "192.168.0.60");
    await user.type(screen.getByPlaceholderText("office-printer"), "nas");
    await user.click(screen.getByRole("button", { name: "Create lease" }));
    await waitFor(() =>
      expect(calls.find((c) => c.method === "POST" && c.path === "dhcp/leases")?.body).toMatchObject({
        mac: "aa:bb:cc:dd:ee:09",
        ip: "192.168.0.60",
        hostname: "nas"
      })
    );

    await user.click(screen.getByRole("button", { name: "Edit" }));
    await user.click(screen.getByRole("button", { name: "Update lease" }));
    await waitFor(() => expect(calls.some((c) => c.method === "PUT" && c.path === "dhcp/leases/1")).toBe(true));

    await user.click(screen.getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(calls.some((c) => c.method === "DELETE" && c.path === "dhcp/leases/1")).toBe(true));
  });
});
