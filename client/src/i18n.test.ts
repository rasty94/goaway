import { describe, expect, it } from "vitest";
import i18n from "./i18n";

describe("i18n setup", () => {
  it("translates, switches language and interpolates", async () => {
    await i18n.changeLanguage("en");
    expect(i18n.t("sidebar.home")).toBe("Home");
    expect(i18n.t("networkMap.showing", { count: 2, total: 5 })).toBe(
      "Showing 2 of 5 clients"
    );

    await i18n.changeLanguage("es");
    expect(i18n.t("sidebar.home")).not.toBe("Home");
    expect(i18n.t("networkMap.showing", { count: 2, total: 5 })).toContain("2");

    // Unknown language falls back to English.
    await i18n.changeLanguage("xx");
    expect(i18n.t("sidebar.home")).toBe("Home");
  });
});
