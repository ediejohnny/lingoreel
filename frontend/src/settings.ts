// Pure helpers of the settings screen: its sections, capturing a shortcut,
// colors with transparency and telling unsaved changes.
import { chordOf, isModifier, type KeyPress } from "./keys";

export type { KeyPress } from "./keys";

/** SubtitleLine is one of the two subtitle lines: on top, or the translation below. */
export type SubtitleLine = "primary" | "translation";

/** SectionId names a page of the settings screen. */
export type SectionId =
  | "general"
  | "playback"
  | "subtitles"
  | "subtitles-primary"
  | "subtitles-translation"
  | "study"
  | "shortcuts"
  | "players"
  | "appearance"
  | "advanced";

/** SectionItem is an entry of the settings menu; detail is a second, smaller line. */
export interface SectionItem {
  id: SectionId;
  label: string;
  detail?: string;
  children?: SectionItem[];
}

/**
 * settingsSections is the settings menu, grouped by subject (DESIGN §32):
 * each subtitle line has its own entry under "Legendas", named by where it
 * goes and, below, by its language.
 */
export function settingsSections(languages: Record<SubtitleLine, string>): SectionItem[] {
  return [
    { id: "general", label: "Geral" },
    { id: "playback", label: "Reprodução" },
    {
      id: "subtitles",
      label: "Legendas",
      children: [
        { id: "subtitles-primary", label: "De cima", detail: languages.primary },
        { id: "subtitles-translation", label: "De baixo", detail: languages.translation },
      ],
    },
    { id: "study", label: "Estudo" },
    { id: "shortcuts", label: "Atalhos" },
    { id: "players", label: "Player" },
    { id: "appearance", label: "Aparência" },
    { id: "advanced", label: "Avançado" },
  ];
}

/** lineOfSection is the subtitle line a page edits, or "" for a page of no single line. */
export function lineOfSection(section: SectionId): SubtitleLine | "" {
  if (section === "subtitles-primary") {
    return "primary";
  }
  if (section === "subtitles-translation") {
    return "translation";
  }
  return "";
}

/**
 * lineLanguage names a line's preferred language by its label; a line
 * without one says what it is for, and an unknown language keeps its tag.
 */
export function lineLanguage(line: SubtitleLine, tag: string, languages: readonly { value: string; label: string }[]): string {
  if (tag === "") {
    return line === "primary" ? "principal" : "tradução";
  }
  return languages.find((option) => option.value === tag)?.label ?? tag;
}

export type Capture = { kind: "wait" } | { kind: "cancel" } | { kind: "unsupported" } | { kind: "chord"; chord: string };

/**
 * captureOf reads a key going down while a shortcut is captured, as chordOf
 * does. Modifiers alone wait for a key; Esc alone cancels; the Windows key
 * is not allowed.
 */
export function captureOf(press: KeyPress): Capture {
  if (press.code === "AltRight") {
    return { kind: "chord", chord: "AltRight" };
  }
  if (isModifier(press.code)) {
    return press.metaKey ? { kind: "unsupported" } : { kind: "wait" };
  }
  if (press.metaKey) {
    return { kind: "unsupported" };
  }
  if (press.code === "Escape" && !press.ctrlKey && !press.shiftKey && !press.altKey) {
    return { kind: "cancel" };
  }
  return { kind: "chord", chord: chordOf(press) };
}

/** splitColor separates "#RRGGBB[AA]" into its color and its opacity (0 to 1). */
export function splitColor(color: string): { hex: string; alpha: number } {
  const match = /^#([0-9a-fA-F]{6})([0-9a-fA-F]{2})?$/.exec(color.trim());
  if (!match) {
    return { hex: "#000000", alpha: 1 };
  }
  const alpha = match[2] === undefined ? 1 : parseInt(match[2], 16) / 255;
  return { hex: `#${(match[1] ?? "000000").toUpperCase()}`, alpha };
}

/** joinColor writes a color and an opacity as the Go side reads them. */
export function joinColor(hex: string, alpha: number): string {
  const base = splitColor(hex).hex;
  const clamped = Number.isFinite(alpha) ? Math.min(Math.max(alpha, 0), 1) : 1;
  const byte = Math.round(clamped * 255);
  if (byte === 255) {
    return base;
  }
  return base + byte.toString(16).toUpperCase().padStart(2, "0");
}

/** sameSettings tells whether two drafts hold the same choices. */
export function sameSettings(a: unknown, b: unknown): boolean {
  return JSON.stringify(a) === JSON.stringify(b);
}

/** clampNumber keeps a typed number inside its range; text that is no number keeps the old value. */
export function clampNumber(text: string, min: number, max: number, fallback: number): number {
  const value = Number(text.replace(",", "."));
  if (text.trim() === "" || !Number.isFinite(value)) {
    return fallback;
  }
  return Math.min(Math.max(value, min), max);
}
