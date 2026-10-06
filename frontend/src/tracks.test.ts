import { describe, expect, it } from "vitest";
import type { Track } from "./services/backend";
import { forLine, none, playingOptions } from "./tracks";

const subtitles = [
  { number: 4, label: "Inglês", language: "en", readable: true, note: "", warning: false },
  { number: 5, label: "Português", language: "pt-BR", readable: true, note: "", warning: false },
] as Track[];

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

describe("playingOptions", () => {
  it("offers none below, always", () => {
    expect(playingOptions("translation", subtitles, 5, "Nenhuma")[0]?.value).toBe(none);
  });

  it("offers none on top only while the line shows none", () => {
    expect(playingOptions("primary", subtitles, 4, "Nenhuma").map((option) => option.value)).toEqual(["4", "5"]);
    // A session started without a subtitle on top: the list shows what is on screen.
    expect(playingOptions("primary", subtitles, 0, "Nenhuma").map((option) => option.value)).toEqual([none, "4", "5"]);
  });
});
