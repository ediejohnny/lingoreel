import { afterEach, describe, expect, it } from "vitest";
import { durationLabel, progress, sizeLabel, speedLabel } from "./format";
import { editClock, passageClock } from "./format";
import { fallback, setLanguage } from "./i18n";

describe("numbers in English", () => {
  afterEach(() => setLanguage(fallback));
  it("carry a decimal point, which the Go side also reads", () => {
    setLanguage("en-US");
    expect(speedLabel(1.25)).toBe("1.25×");
    expect(speedLabel(0)).toBe("1.00×");
    expect(passageClock(623.5)).toBe("10:23.5");
    expect(editClock(3723.456)).toBe("1:02:03.456");
    expect(durationLabel(89 * 60 + 32)).toBe("1h 30m");
  });
});

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

describe("sizeLabel", () => {
  it("counts megabytes as Windows does", () => {
    // The Portuguese speech pack: 355,018,566 bytes.
    expect(sizeLabel(355_018_566)).toBe("339 MB");
    expect(sizeLabel(1000)).toBe("1 MB");
    expect(sizeLabel(0)).toBe("0 MB");
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

describe("editClock", () => {
  it("writes milliseconds as the Go side shows them", () => {
    expect(editClock(3723.456)).toBe("1:02:03,456");
    expect(editClock(123.456)).toBe("2:03,456");
    expect(editClock(2.05)).toBe("0:02,050");
    expect(editClock(0.001)).toBe("0:00,001");
    expect(editClock(4.123)).toBe("0:04,123");
  });
  it("carries a rounded millisecond into the second", () => {
    expect(editClock(59.9996)).toBe("1:00,000");
    expect(editClock(3599.9999)).toBe("1:00:00,000");
  });
  it("never shows a negative or broken time", () => {
    expect(editClock(-0.5)).toBe("0:00,000");
    expect(editClock(Number.NaN)).toBe("0:00,000");
    expect(editClock(Number.NEGATIVE_INFINITY)).toBe("0:00,000");
  });
});
