import { afterEach, describe, expect, it } from "vitest";
import { adjustedApart, computerFacts, levelName } from "./computer";
import { fallback, setLanguage } from "./i18n";

const read = { processor: "Core 2 Duo", processors: 2, memoryGiB: 4, level: "basic", adjusted: "basic" };

describe("computerFacts", () => {
  afterEach(() => setLanguage(fallback));

  it("tell what was read and its level, and say so of a part not read", () => {
    expect(computerFacts(read).map((fact) => fact.value)).toEqual(["Core 2 Duo", "2", "4 GB", "Básico"]);
    const unread = computerFacts({ ...read, processor: "", processors: 0, memoryGiB: 0, level: "medium" });
    expect(unread.map((fact) => fact.value)).toEqual(["não lido", "não lido", "não lido", "Médio"]);
    setLanguage("en-US");
    expect(levelName("high")).toBe("High");
  });
});

describe("adjustedApart", () => {
  it("tells settings adjusted to a computer of another level, not ones never adjusted", () => {
    expect(adjustedApart(read)).toBe(false);
    expect(adjustedApart({ ...read, adjusted: "high" })).toBe(true);
    expect(adjustedApart({ ...read, adjusted: "" })).toBe(false);
  });
});
