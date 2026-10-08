import { afterEach, describe, expect, it } from "vitest";
import { fallback, languages, setLanguage } from "../i18n";
import { searchTopics, topics, type Keys } from "./topics";

const keys: Keys = [
  { command: "play-pause", label: "P" },
  { command: "seek-back", label: "←" },
  { command: "seek-forward", label: "→" },
  { command: "passage-mark-start", label: "Ctrl+F9" },
];

describe("topics", () => {
  afterEach(() => setLanguage(fallback));

  it("are written whole in every language, with nothing left unfilled", () => {
    for (const language of languages()) {
      setLanguage(language);
      const all = topics(keys);
      expect(all.length).toBeGreaterThanOrEqual(14);
      for (const topic of all) {
        for (const text of [topic.title, ...topic.paragraphs]) {
          expect(text, `${language} ${topic.id}`).not.toMatch(/ui\.help\.|\{[a-zA-Z]+\}/);
        }
      }
    }
  });

  it("name the profile's own keys, and say so when a command has none", () => {
    const watch = topics(keys).find((topic) => topic.id === "watch");
    expect(watch?.paragraphs[0]).toContain("(P)");
    expect(watch?.paragraphs[0]).toContain("← e →");
    const passages = topics(keys).find((topic) => topic.id === "passages");
    expect(passages?.paragraphs[0]).toContain("Ctrl+F9");
    expect(passages?.paragraphs[0]).toContain("(sem atalho)");
  });
});

describe("searchTopics", () => {
  afterEach(() => setLanguage(fallback));

  it("finds a topic by any of its words, accents aside, in the language shown", () => {
    const all = topics(keys);
    expect(searchTopics(all, "")).toEqual(all);
    for (const query of ["karaokê", "KARAOKE", "  karaoke  "]) {
      expect(searchTopics(all, query).map((topic) => topic.id)).toContain("karaoke");
    }
    expect(searchTopics(all, "sincronia atrasada").map((topic) => topic.id)).toContain("sync");
    expect(searchTopics(all, "xyzzy")).toEqual([]);
    setLanguage("en-US");
    expect(searchTopics(topics(keys), "karaoke").map((topic) => topic.id)).toContain("karaoke");
  });
});
