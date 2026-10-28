import { describe, expect, it } from "vitest";
import { fitsAbove } from "./placement";

describe("fitsAbove", () => {
  it("opens below while the note fits there", () => {
    expect(fitsAbove(100, 128, 200, 800)).toBe(false);
  });
  it("opens above near the window's bottom, where there is room up", () => {
    expect(fitsAbove(700, 728, 200, 800)).toBe(true);
  });
  it("stays below when neither side has room, so the page can scroll to it", () => {
    expect(fitsAbove(100, 128, 700, 800)).toBe(false);
  });
});
