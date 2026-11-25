import { mockApi } from "@/test/api";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { PiholeImport } from "./pihole-import";

describe("Pi-hole import", () => {
  it("previews a backup, then applies only the new entries", async () => {
    const plan = {
      adlists: [{ name: "StevenBlack", url: "https://sb.example/hosts", enabled: true, exists: false }],
      blacklist: [{ pattern: "bad.example", exists: true }],
      whitelist: [{ pattern: "good.example", exists: false }],
      resolutions: [{ domain: "nas.lan", value: "192.168.0.5", type: "A", exists: false }],
      skipped: ["regex: ^ads"]
    };
    const { calls } = mockApi({
      "POST import/pihole/preview": plan,
      "POST import/pihole/apply": { created: { adlists: 1, whitelist: 1, resolutions: 1 }, skipped: { blacklist: 1 }, errors: [] }
    });
    const user = userEvent.setup();
    const { container } = render(<PiholeImport />);

    const input = container.querySelector('input[type="file"]') as HTMLInputElement;
    await user.upload(input, new File(["zip"], "teleporter.zip", { type: "application/zip" }));
    await user.click(screen.getByRole("button", { name: "Analyse" }));

    expect(await screen.findByText(/StevenBlack/)).toBeInTheDocument();
    expect(screen.getByText("regex: ^ads")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /^Import \d+ entries$/ }));
    await waitFor(() => expect(calls.some((c) => c.path === "import/pihole/apply")).toBe(true));
    const sent = calls.find((c) => c.path === "import/pihole/apply")!.body as { sections: Record<string, boolean> };
    expect(sent.sections).toMatchObject({ adlists: true, whitelist: true, resolutions: true });
  });
});
