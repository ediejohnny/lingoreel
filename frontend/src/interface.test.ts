import { describe, expect, it } from "vitest";
import { interfaceVariables } from "./interface";

const none = { accent: "", fontFamily: "", textSize: 1 };

/** channels reads "#rrggbb" into numbers, for comparisons. */
function channels(color: string | undefined): number[] {
  return [1, 3, 5].map((at) => parseInt((color ?? "").slice(at, at + 2), 16));
}

describe("interfaceVariables", () => {
  it("keeps the theme's accent and the app's font when none is chosen", () => {
    expect(interfaceVariables(none, "light")).toEqual({ "--sp-text-size": "1" });
    expect(interfaceVariables({ ...none, textSize: 1.25 }, "dark")).toEqual({ "--sp-text-size": "1.25" });
  });

  it("makes the hover darker in the light theme and lighter in the dark one", () => {
    const look = { ...none, accent: "#FF8C00" };
    const light = interfaceVariables(look, "light");
    const dark = interfaceVariables(look, "dark");
    expect(light["--sp-accent"]).toBe("#ff8c00");
    const accent = channels("#ff8c00");
    channels(light["--sp-accent-hover"]).forEach((channel, i) => expect(channel).toBeLessThanOrEqual(accent[i] ?? 0));
    channels(dark["--sp-accent-hover"]).forEach((channel, i) => expect(channel).toBeGreaterThanOrEqual(accent[i] ?? 0));
    expect(light["--sp-accent-hover"]).not.toBe(dark["--sp-accent-hover"]);
    // The focus ring is the accent on light, its lighter hover on dark, as the themes' own.
    expect(light["--sp-focus"]).toBe("#ff8c00");
    expect(dark["--sp-focus"]).toBe(dark["--sp-accent-hover"]);
  });

  it("writes on the accent in the color that reads best", () => {
    // The themes' own blues get the text they have today.
    expect(interfaceVariables({ ...none, accent: "#1F66D1" }, "light")["--sp-on-accent"]).toBe("#ffffff");
    expect(interfaceVariables({ ...none, accent: "#5B9BFF" }, "dark")["--sp-on-accent"]).toBe("#0b1220");
    expect(interfaceVariables({ ...none, accent: "#FF8C00" }, "light")["--sp-on-accent"]).toBe("#0b1220");
    expect(interfaceVariables({ ...none, accent: "#5A1A8C" }, "dark")["--sp-on-accent"]).toBe("#ffffff");
  });

  it("puts a chosen font before the app's own, never breaking out of its quotes", () => {
    expect(interfaceVariables({ ...none, fontFamily: "Verdana" }, "light")["--sp-font"]).toBe(
      '"Verdana", "Segoe UI Variable Text", "Segoe UI", system-ui, sans-serif',
    );
    expect(interfaceVariables({ ...none, fontFamily: 'Bad"; color: red\\' }, "light")["--sp-font"]).toMatch(/^"Bad; color: red", /);
  });

  it("ignores an accent that is not a color", () => {
    expect(interfaceVariables({ ...none, accent: "laranja" }, "light")).toEqual({ "--sp-text-size": "1" });
  });
});
