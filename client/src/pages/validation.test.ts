import { describe, expect, it } from "vitest";
import { validateFQDN } from "./validation";

describe("validateFQDN", () => {
  it("accepts valid FQDNs and wildcards", () => {
    expect(validateFQDN("example.com.").isValid).toBe(true);
    expect(validateFQDN("sub.domain-1.example.org.").isValid).toBe(true);
    expect(validateFQDN("*.example.com.").isValid).toBe(true);
    expect(validateFQDN("  example.com.  ").isValid).toBe(true);
  });

  it("rejects empty, missing trailing dot, or bare dot", () => {
    expect(validateFQDN("").isValid).toBe(false);
    expect(validateFQDN("   ").isValid).toBe(false);
    expect(validateFQDN("example.com").isValid).toBe(false);
    expect(validateFQDN(".").isValid).toBe(false);
    expect(validateFQDN("*.").isValid).toBe(false);
  });

  it("rejects invalid characters, hyphens, labels, and TLDs", () => {
    expect(validateFQDN("-bad.example.com.").isValid).toBe(false);
    expect(validateFQDN("bad-.example.com.").isValid).toBe(false);
    expect(validateFQDN("bad_char.example.com.").isValid).toBe(false);
    expect(validateFQDN("localhost.").isValid).toBe(false);
    expect(validateFQDN("example.123.").isValid).toBe(false);
    expect(validateFQDN(`${"a".repeat(64)}.com.`).isValid).toBe(false);
    expect(validateFQDN(`${"a.".repeat(130)}com.`).isValid).toBe(false);
  });
});
