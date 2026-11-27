import { mockApi } from "@/test/api";
import { toast } from "sonner";
import { describe, expect, it, vi } from "vitest";
import { BlacklistDomain, WhitelistDomain } from "./domainActions";

describe("query log domain actions", () => {
  it("blacklisting first removes the domain from the whitelist", async () => {
    const success = vi.spyOn(toast, "success");
    const { calls } = mockApi({ "POST custom": {} });

    await BlacklistDomain("ads.example");

    expect(calls.map((c) => `${c.method} ${c.path}`)).toEqual([
      "DELETE whitelist?domain=ads.example",
      "POST custom"
    ]);
    expect(calls[1].body).toEqual({ domains: ["ads.example"] });
    expect(success).toHaveBeenCalledWith("Blacklisted ads.example");
  });

  it("reports a failed blacklist", async () => {
    const error = vi.spyOn(toast, "error");
    mockApi({ "POST custom": () => [500, { error: "db locked" }] });
    await BlacklistDomain("ads.example");
    expect(error).toHaveBeenCalledWith("Failed to block ads.example");
  });

  it("whitelisting first removes the domain from the blacklist", async () => {
    const { calls } = mockApi({ "POST whitelist": {} });

    await expect(WhitelistDomain("cdn.example")).resolves.toBe(true);
    expect(calls.map((c) => `${c.method} ${c.path}`)).toEqual([
      "DELETE blacklist?domain=cdn.example",
      "POST whitelist"
    ]);
    expect(calls[1].body).toEqual({ domain: "cdn.example" });
  });

  it("returns false when the whitelist request fails", async () => {
    mockApi({ "POST whitelist": () => [400, { error: "invalid domain" }] });
    await expect(WhitelistDomain("bad..example")).resolves.toBe(false);
  });
});
