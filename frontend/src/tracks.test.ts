import { describe, expect, it } from "vitest";
import type { Track } from "./services/backend";
import { adopted, forLine } from "./tracks";

describe("forLine", () => {
  // The app's order: English from the best, then Portuguese from the best, then the rest.
  const ranked = [
    { number: 9, label: "", language: "en", readable: true, note: "", warning: false },
    { number: 4, label: "", language: "en-US", readable: true, note: "", warning: false },
    { number: 7, label: "", language: "pt-BR", readable: true, note: "", warning: false },
    { number: 5, label: "", language: "pt", readable: true, note: "", warning: false },
    { number: 8, label: "", language: "es", readable: true, note: "", warning: false },
  ] as Track[];
  const numbers = (tracks: Track[]) => tracks.map((track) => track.number);

  it("brings the line's language to the top, the app's order within it", () => {
    expect(numbers(forLine(ranked, 7))).toEqual([7, 5, 9, 4, 8]);
    expect(numbers(forLine(ranked, 9))).toEqual([9, 4, 7, 5, 8]);
  });

  it("keeps the app's order when the line has no track", () => {
    expect(numbers(forLine(ranked, 0))).toEqual([9, 4, 7, 5, 8]);
  });
});

describe("adopted", () => {
  it("keeps what the user chose and takes the new suggestion for the rest", () => {
    const chosen = { primary: "0", translation: "5", audio: "2" };
    const suggested = { primary: 1000, translation: 6, audio: 3 };
    expect(adopted(suggested, chosen, { primary: false, translation: true, audio: false })).toEqual({
      primary: "1000",
      translation: "5",
      audio: "3",
    });
  });
});
