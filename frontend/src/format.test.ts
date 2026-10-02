import { describe, expect, it } from "vitest";
import { durationLabel, progress, speedLabel } from "./format";
import { passageClock } from "./format";

describe("speedLabel", () => {
  it("shows two decimals with a comma", () => {
    expect(speedLabel(1)).toBe("1,00×");
    expect(speedLabel(0.75)).toBe("0,75×");
    expect(speedLabel(1.25)).toBe("1,25×");
  });
  it("falls back to normal speed for nonsense", () => {
    expect(speedLabel(0)).toBe("1,00×");
    expect(speedLabel(-1)).toBe("1,00×");
    expect(speedLabel(Number.NaN)).toBe("1,00×");
    expect(speedLabel(Number.POSITIVE_INFINITY)).toBe("1,00×");
  });
});

describe("progress", () => {
  it("is the share played", () => {
    expect(progress(30, 120)).toBe(0.25);
    expect(progress(0, 120)).toBe(0);
    expect(progress(120, 120)).toBe(1);
  });
  it("stays between 0 and 1", () => {
    expect(progress(130, 120)).toBe(1);
    expect(progress(-5, 120)).toBe(0);
  });
  it("is zero without a duration", () => {
    expect(progress(10, 0)).toBe(0);
    expect(progress(10, Number.NaN)).toBe(0);
    expect(progress(Number.NaN, 10)).toBe(0);
  });
});

describe("durationLabel", () => {
  it("shows minutes under an hour", () => {
    expect(durationLabel(42 * 60)).toBe("42min");
  });
  it("shows hours and minutes", () => {
    // 1:29:32 rounds to 90 minutes.
    expect(durationLabel(89 * 60 + 32)).toBe("1h30min");
    expect(durationLabel(3600 + 5 * 60)).toBe("1h05min");
  });
  it("is empty without a length", () => {
    expect(durationLabel(0)).toBe("");
    expect(durationLabel(Number.NaN)).toBe("");
  });
});

describe("passageClock", () => {
  it("writes what the Go side reads back", () => {
    expect(passageClock(4223)).toBe("1:10:23");
    expect(passageClock(623.5)).toBe("10:23,5");
    expect(passageClock(59.96)).toBe("1:00");
    expect(passageClock(0)).toBe("0:00");
  });
  it("never shows a negative or broken time", () => {
    expect(passageClock(-3)).toBe("0:00");
    expect(passageClock(Number.NaN)).toBe("0:00");
    expect(passageClock(Number.POSITIVE_INFINITY)).toBe("0:00");
  });
});
