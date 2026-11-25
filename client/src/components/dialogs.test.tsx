import { APIKeyDialog } from "@/components/APIKeyDialog";
import PauseBlockingDialog, { NavActions } from "@/components/header/nav-actions";
import { SidebarProvider } from "@/components/ui/sidebar";
import { Dialog } from "@/components/ui/dialog";
import { Blacklist } from "@/pages/blacklist";
import { mockApi } from "@/test/api";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";

describe("API key dialog", () => {
  it("lists keys and generates a new one", async () => {
    const { calls } = mockApi({
      apiKey: [{ name: "ci", key: "gk_old", createdAt: "2026-11-01T00:00:00Z" }],
      "POST apiKey": "gk_new_secret"
    });
    const user = userEvent.setup();
    render(<APIKeyDialog open onOpenChange={() => {}} />);

    expect(await screen.findByText("ci")).toBeInTheDocument();
    await user.type(screen.getByRole("textbox"), "deploy");
    await user.click(screen.getByRole("button", { name: /Generate/ }));

    expect(await screen.findByText(/gk_new_secret/)).toBeInTheDocument();
    expect(calls.find((c) => c.method === "POST")?.body).toEqual({ name: "deploy" });
  });
});

describe("Pause blocking dialog", () => {
  it("pauses blocking for the selected duration", async () => {
    let paused = false;
    const { calls } = mockApi({
      pause: () => [200, paused ? { paused: true, timeLeft: 30 } : { paused: false, timeLeft: 0 }],
      "POST pause": () => {
        paused = true;
        return [200, {}];
      }
    });
    const user = userEvent.setup();
    render(
      <Dialog open>
        <PauseBlockingDialog onClose={vi.fn()} />
      </Dialog>
    );

    await user.click(await screen.findByText("30s"));
    await user.click(screen.getByRole("button", { name: "Pause" }));

    await waitFor(() => expect(calls.find((c) => c.method === "POST")?.body).toEqual({ time: 30 }));
    expect(await screen.findByText("remaining")).toBeInTheDocument();
  });
});

describe("Add list dialog", () => {
  it("subscribes to a new blocklist and shows it", async () => {
    const { calls } = mockApi({
      "POST addList": { active: true, blockedCount: 42, lastUpdated: 1764057600 }
    });
    const user = userEvent.setup();
    render(
      <MemoryRouter>
        <Blacklist />
      </MemoryRouter>
    );

    await user.click(await screen.findByRole("button", { name: /Add List/ }));
    await user.type(screen.getByLabelText("List Name"), "My list");
    await user.type(screen.getByLabelText("List URL"), "https://lists.example/hosts.txt");
    const submit = screen.getAllByRole("button", { name: "Add List" }).at(-1)!;
    await user.click(submit);

    await waitFor(() =>
      expect(calls.find((c) => c.path === "addList")?.body).toEqual({
        name: "My list",
        url: "https://lists.example/hosts.txt",
        active: true
      })
    );
    expect(await screen.findByText("My list")).toBeInTheDocument();
  });
});

describe("Header actions", () => {
  it("opens About with server info and asks before restarting", async () => {
    const { calls } = mockApi({ restart: {} });
    const user = userEvent.setup();
    const { container } = render(
      <SidebarProvider>
        <NavActions />
      </SidebarProvider>
    );

    const trigger = () => container.querySelector("button") as HTMLButtonElement;
    await user.click(trigger());
    await user.click(await screen.findByText("About"));
    expect(await screen.findByText("abc1234")).toBeInTheDocument();
    await user.keyboard("{Escape}");

    await user.click(trigger());
    await user.click(await screen.findByText("Restart"));
    expect(await screen.findByText("Restart Application")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Restart" }));
    await waitFor(() => expect(calls.some((c) => c.path === "restart")).toBe(true));
  });
});

describe("Add multiple lists", () => {
  it("parses pasted URLs into entries and submits the named ones", async () => {
    const { calls } = mockApi({ "POST addLists": { ignored: {} } });
    const user = userEvent.setup();
    const { container } = render(
      <MemoryRouter>
        <Blacklist />
      </MemoryRouter>
    );

    await user.click(await screen.findByRole("button", { name: /Add List/ }));
    await user.click(screen.getByRole("tab", { name: /Multiple/ }));
    const bulk = container.ownerDocument.getElementById("bulk-urls") as HTMLTextAreaElement;
    await user.type(bulk, "https://one.example/hosts{enter}https://two.example/hosts");
    await user.click(screen.getByRole("button", { name: /Parse URLs/ }));

    const names = screen.getAllByPlaceholderText("Enter list name");
    expect(names).toHaveLength(2);
    await user.type(names[0], "One");
    await user.type(names[1], "Two");
    await user.click(screen.getByRole("button", { name: "Add Lists" }));

    await waitFor(() =>
      expect(calls.find((c) => c.path === "addLists")?.body).toEqual({
        lists: [
          { name: "One", url: "https://one.example/hosts", active: true },
          { name: "Two", url: "https://two.example/hosts", active: true }
        ]
      })
    );
  });
});
