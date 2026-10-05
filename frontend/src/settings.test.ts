import { describe, expect, it } from "vitest";
import {
  captureOf,
  clampNumber,
  joinColor,
  lineLanguage,
  lineOfSection,
  sameSettings,
  settingsSections,
  splitColor,
  type KeyPress,
  type SectionItem,
} from "./settings";

describe("settingsSections", () => {
  const sections = settingsSections({ primary: "Inglês", translation: "Português do Brasil" });
  const ids = (items: SectionItem[]): string[] => items.flatMap((item) => [item.id, ...ids(item.children ?? [])]);

  it("gives each subtitle line its own entry under Legendas, with its language", () => {
    const subtitles = sections.find((item) => item.id === "subtitles");
    expect(subtitles?.label).toBe("Legendas");
    expect(subtitles?.children).toEqual([
      { id: "subtitles-primary", label: "De cima", detail: "Inglês" },
      { id: "subtitles-translation", label: "De baixo", detail: "Português do Brasil" },
    ]);
  });

  it("lists every page once, in the order of DESIGN §32", () => {
    const all = ids(sections);
    expect(new Set(all).size).toBe(all.length);
    expect(sections.map((item) => item.label)).toEqual([
      "Geral",
      "Reprodução",
      "Legendas",
      "Estudo",
      "Atalhos",
      "Player",
      "Aparência",
      "Avançado",
    ]);
  });

  it("tells the line each page edits", () => {
    expect(lineOfSection("subtitles-primary")).toBe("primary");
    expect(lineOfSection("subtitles-translation")).toBe("translation");
    for (const other of ids(sections).filter((id) => !id.startsWith("subtitles-"))) {
      expect(lineOfSection(other as SectionItem["id"])).toBe("");
    }
  });
});

describe("lineLanguage", () => {
  const languages = [
    { value: "en", label: "Inglês" },
    { value: "pt-BR", label: "Português do Brasil" },
  ];

  it("names the language by its label", () => {
    expect(lineLanguage("primary", "en", languages)).toBe("Inglês");
    expect(lineLanguage("translation", "pt-BR", languages)).toBe("Português do Brasil");
  });

  it("says what a line without a language is for, and keeps an unknown tag", () => {
    expect(lineLanguage("translation", "", languages)).toBe("tradução");
    expect(lineLanguage("primary", "", languages)).toBe("principal");
    expect(lineLanguage("primary", "ja", languages)).toBe("ja");
  });
});

const press = (code: string, mods: Partial<KeyPress> = {}): KeyPress => ({
  code,
  ctrlKey: false,
  shiftKey: false,
  altKey: false,
  metaKey: false,
  ...mods,
});

describe("captureOf", () => {
  it("writes chords by the key's place, modifiers in a fixed order", () => {
    expect(captureOf(press("KeyP"))).toEqual({ kind: "chord", chord: "KeyP" });
    expect(captureOf(press("Comma", { shiftKey: true }))).toEqual({ kind: "chord", chord: "Shift+Comma" });
    expect(captureOf(press("Period", { shiftKey: true, ctrlKey: true }))).toEqual({ kind: "chord", chord: "Ctrl+Shift+Period" });
    expect(captureOf(press("Digit1", { altKey: true, ctrlKey: true }))).toEqual({ kind: "chord", chord: "Ctrl+Alt+Digit1" });
    expect(captureOf(press("Space"))).toEqual({ kind: "chord", chord: "Space" });
  });

  it("waits while only modifiers are down", () => {
    for (const code of ["ControlLeft", "ShiftRight", "AltLeft", "CapsLock", ""]) {
      expect(captureOf(press(code, { ctrlKey: true }))).toEqual({ kind: "wait" });
    }
  });

  it("takes the right Alt alone, even with the Ctrl AltGr adds", () => {
    expect(captureOf(press("AltRight", { ctrlKey: true, altKey: true }))).toEqual({ kind: "chord", chord: "AltRight" });
  });

  it("cancels on Esc alone but takes Esc with modifiers", () => {
    expect(captureOf(press("Escape"))).toEqual({ kind: "cancel" });
    expect(captureOf(press("Escape", { shiftKey: true }))).toEqual({ kind: "chord", chord: "Shift+Escape" });
  });

  it("refuses the Windows key", () => {
    expect(captureOf(press("KeyE", { metaKey: true }))).toEqual({ kind: "unsupported" });
    expect(captureOf(press("MetaLeft", { metaKey: true }))).toEqual({ kind: "unsupported" });
  });
});

describe("colors", () => {
  it("splits and joins colors with their opacity", () => {
    expect(splitColor("#ffd23f")).toEqual({ hex: "#FFD23F", alpha: 1 });
    expect(splitColor("#000000C8").hex).toBe("#000000");
    expect(splitColor("#000000C8").alpha).toBeCloseTo(200 / 255, 5);
    expect(joinColor("#000000", 200 / 255)).toBe("#000000C8");
    expect(joinColor("#FFFFFF", 1)).toBe("#FFFFFF");
    expect(joinColor("#ffffff", 0)).toBe("#FFFFFF00");
  });

  it("survives what is no color", () => {
    expect(splitColor("red")).toEqual({ hex: "#000000", alpha: 1 });
    expect(splitColor("#12345")).toEqual({ hex: "#000000", alpha: 1 });
    expect(joinColor("#123456", Number.NaN)).toBe("#123456");
    expect(joinColor("#123456", 7)).toBe("#123456");
    expect(joinColor("#123456", -1)).toBe("#12345600");
  });
});

describe("clampNumber", () => {
  it("reads commas and keeps the range", () => {
    expect(clampNumber("1,5", 0, 10, 3)).toBe(1.5);
    expect(clampNumber("500", 8, 200, 32)).toBe(200);
    expect(clampNumber("-4", 0, 10, 3)).toBe(0);
  });

  it("keeps the old value for what is no number", () => {
    expect(clampNumber("", 0, 10, 3)).toBe(3);
    expect(clampNumber("abc", 0, 10, 3)).toBe(3);
    expect(clampNumber("Infinity", 0, 10, 3)).toBe(3);
  });
});

describe("sameSettings", () => {
  it("compares drafts by their content", () => {
    expect(sameSettings({ a: 1, b: [1, 2] }, { a: 1, b: [1, 2] })).toBe(true);
    expect(sameSettings({ a: 1 }, { a: 2 })).toBe(false);
  });
});
