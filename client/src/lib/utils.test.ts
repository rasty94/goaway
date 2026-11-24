import { describe, expect, it } from "vitest";
import { cn } from "./utils";

describe("cn", () => {
  it("merges conditional and conflicting tailwind classes", () => {
    const isHidden = false;
    expect(cn("px-2 py-1", "px-4")).toBe("py-1 px-4");
    expect(cn("text-sm", isHidden && "hidden", "font-medium")).toBe(
      "text-sm font-medium"
    );
  });
});
