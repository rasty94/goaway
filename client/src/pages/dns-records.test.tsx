import { mockApi } from "@/test/api";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { Prefetch } from "./prefetch";
import { Resolution } from "./resolution";

describe("Custom resolutions", () => {
  it("adds and deletes a mapping", async () => {
    vi.stubGlobal("confirm", () => true);
    const { calls } = mockApi({ "POST resolution": {}, "DELETE resolution": {} });
    const user = userEvent.setup();
    render(<Resolution />);

    expect(await screen.findByText("nas.lan")).toBeInTheDocument();
    await user.type(screen.getByPlaceholderText("example.local."), "printer.lan.");
    await user.type(screen.getByPlaceholderText("192.168.1.100 or internal.server.lan."), "192.168.0.9");
    await user.click(screen.getByRole("button", { name: "Save Resolution" }));

    await waitFor(() =>
      expect(calls.find((c) => c.method === "POST")?.body).toEqual({ domain: "printer.lan.", value: "192.168.0.9", type: "A" })
    );
    expect(await screen.findByText("printer.lan.")).toBeInTheDocument();

    await user.click(screen.getAllByRole("button", { name: "Delete" })[0]);
    await waitFor(() => expect(calls.some((c) => c.method === "DELETE")).toBe(true));
  });
});

describe("Prefetch", () => {
  it("lists entries and adds a domain", async () => {
    const { calls } = mockApi({ "POST prefetch": {} });
    const user = userEvent.setup();
    render(<Prefetch />);

    expect(await screen.findByText("example.com")).toBeInTheDocument();
    await user.type(screen.getByPlaceholderText("example.com."), "news.example.");
    await user.click(screen.getByRole("button", { name: "Add Prefetch" }));

    await waitFor(() => expect(calls.find((c) => c.method === "POST")?.body).toMatchObject({ domain: "news.example." }));
  });
});
