import { mockApi } from "@/test/api";
import { UpstreamEntry } from "@/pages/upstream";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { toast } from "sonner";
import { describe, expect, it, vi } from "vitest";
import { AddUpstream } from "./addUpstream";
import { UpstreamCard } from "./card";

const entry: UpstreamEntry = {
  upstreamName: "Cloudflare",
  dnsPing: "4ms",
  icmpPing: "3ms",
  name: "one.one.one.one",
  preferred: false,
  upstream: "1.1.1.1:53"
};

async function openDialog() {
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Add Upstream" }));
  return user;
}

describe("AddUpstream", () => {
  it("rejects an address without a port", async () => {
    const error = vi.spyOn(toast, "error");
    const { calls } = mockApi();
    const onAdd = vi.fn();
    render(<AddUpstream onAdd={onAdd} />);
    const user = await openDialog();

    await user.type(screen.getByLabelText("DNS Server Address"), "1.1.1.1{Enter}");
    expect(error).toHaveBeenCalledWith(expect.stringMatching(/^Invalid format/));
    expect(calls.some((c) => c.method === "POST")).toBe(false);
    expect(onAdd).not.toHaveBeenCalled();
  });

  it.each(["9.9.9.9:53", "[2620:fe::fe]:53"])("adds %s and passes a complete entry up", async (address) => {
    const { calls } = mockApi({ "POST upstream": {} });
    const onAdd = vi.fn();
    render(<AddUpstream onAdd={onAdd} />);
    const user = await openDialog();

    // user-event treats "[" as a key descriptor; "[[" types a literal bracket.
    await user.type(screen.getByLabelText("DNS Server Address"), address.replace("[", "[["));
    await user.click(screen.getAllByRole("button", { name: "Add Upstream" }).at(-1)!);

    await waitFor(() => expect(onAdd).toHaveBeenCalledOnce());
    expect(onAdd.mock.calls[0][0]).toMatchObject({ upstream: address, name: address, upstreamName: "", preferred: false });
    expect(calls.find((c) => c.method === "POST")?.body).toEqual({ upstream: address });
  });

  it("keeps the dialog open when the server refuses the upstream", async () => {
    const error = vi.spyOn(toast, "error");
    mockApi({ "POST upstream": () => [400, { message: "already exists" }] });
    const onAdd = vi.fn();
    render(<AddUpstream onAdd={onAdd} />);
    const user = await openDialog();

    await user.type(screen.getByLabelText("DNS Server Address"), "8.8.8.8:53{Enter}");
    await waitFor(() => expect(error).toHaveBeenCalled());
    expect(onAdd).not.toHaveBeenCalled();
    expect(screen.getByLabelText("DNS Server Address")).toBeInTheDocument();
  });
});

describe("UpstreamCard", () => {
  it("sets the upstream as preferred", async () => {
    const { calls } = mockApi({ "PUT preferredUpstream": { message: "ok" } });
    const user = userEvent.setup();
    render(<UpstreamCard upstream={entry} onRemove={vi.fn()} />);

    expect(screen.getByText("one.one.one.one Cloudflare")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Set Preferred" }));
    expect(await screen.findByRole("button", { name: "Preferred" })).toBeInTheDocument();
    expect(calls.find((c) => c.method === "PUT")?.body).toEqual({ upstream: "1.1.1.1:53" });
  });

  it("asks for confirmation before deleting", async () => {
    const { calls } = mockApi({ "DELETE upstream": { message: "deleted" } });
    const onRemove = vi.fn();
    const user = userEvent.setup();
    render(<UpstreamCard upstream={entry} onRemove={onRemove} />);

    const button = screen.getByRole("button", { name: /Delete/ });
    await user.click(button);
    expect(calls.some((c) => c.method === "DELETE")).toBe(false);

    await user.click(button);
    await waitFor(() => expect(onRemove).toHaveBeenCalledWith("1.1.1.1:53"));
    expect(calls.find((c) => c.method === "DELETE")?.path).toBe("upstream?upstream=1.1.1.1:53");
  });

  it("keeps the card when the server refuses the delete", async () => {
    const warning = vi.spyOn(toast, "warning");
    mockApi({ "DELETE upstream": () => [400, { message: "last upstream" }] });
    const onRemove = vi.fn();
    const user = userEvent.setup();
    render(<UpstreamCard upstream={entry} onRemove={onRemove} />);

    const button = screen.getByRole("button", { name: /Delete/ });
    await user.click(button);
    await user.click(button);
    await waitFor(() => expect(warning).toHaveBeenCalled());
    expect(onRemove).not.toHaveBeenCalled();
  });
});
