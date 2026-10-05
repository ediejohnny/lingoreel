import { describe, expect, it } from "vitest";
import type { Track } from "./services/backend";
import { none, playingOptions } from "./tracks";

const subtitles = [
  { number: 4, label: "Inglês", language: "en", readable: true, note: "", warning: false },
  { number: 5, label: "Português", language: "pt-BR", readable: true, note: "", warning: false },
] as Track[];

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
