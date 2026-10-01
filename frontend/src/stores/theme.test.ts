import { describe, expect, it } from "vitest";
import { asTheme, nextTheme, themeLabel } from "./theme";

describe("asTheme", () => {
  it("keeps the known themes", () => {
    expect(asTheme("light")).toBe("light");
    expect(asTheme("dark")).toBe("dark");
    expect(asTheme("system")).toBe("system");
  });
  it("follows the system for anything else", () => {
    expect(asTheme("")).toBe("system");
    expect(asTheme("Dark")).toBe("system");
    expect(asTheme("purple")).toBe("system");
  });
});

describe("nextTheme", () => {
  it("cycles through the three themes", () => {
    expect(nextTheme("system")).toBe("light");
    expect(nextTheme("light")).toBe("dark");
    expect(nextTheme("dark")).toBe("system");
  });
});

describe("themeLabel", () => {
  it("names every theme in Portuguese", () => {
    expect(themeLabel("system")).toBe("Tema do Windows");
    expect(themeLabel("light")).toBe("Tema claro");
    expect(themeLabel("dark")).toBe("Tema escuro");
  });
});
