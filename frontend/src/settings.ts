// Pure helpers of the settings screen: its sections, capturing a shortcut,
// colors with transparency and telling unsaved changes.
import { t } from "./i18n";
import { chordOf, isModifier, type KeyPress } from "./keys";
import type { ProfileSettings } from "./services/backend";

export type { KeyPress } from "./keys";

/** SubtitleLine is one of the two subtitle lines: on top, or the translation below. */
export type SubtitleLine = "primary" | "translation";

/**
 * PreviewView is what the preview shows: the lines as they look, the karaoke
 * mode's fill or the line shown before its time, or a notice.
 */
export type PreviewView = "" | "fill" | "cue" | "notice";

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
  | "advanced"
  | "help"
  | "help-docs"
  | "help-about";

/** SectionItem is an entry of the settings menu; detail is a second, smaller line. */
export interface SectionItem {
  id: SectionId;
  label: string;
  detail?: string;
  children?: SectionItem[];
}

/**
 * settingsSections is the settings menu, grouped by subject:
 * each subtitle line has its own entry under "Legendas", named by where it
 * goes and, below, by its language.
 */
export function settingsSections(languages: Record<SubtitleLine, string>): SectionItem[] {
  return [
    { id: "general", label: t("ui.settings.sections.general") },
    { id: "playback", label: t("ui.settings.sections.playback") },
    {
      id: "subtitles",
      label: t("ui.settings.sections.subtitles"),
      children: [
        { id: "subtitles-primary", label: t("ui.settings.sections.subtitlesTop"), detail: languages.primary },
        { id: "subtitles-translation", label: t("ui.settings.sections.subtitlesBottom"), detail: languages.translation },
      ],
    },
    { id: "study", label: t("ui.settings.sections.study") },
    { id: "shortcuts", label: t("ui.settings.sections.shortcuts") },
    { id: "players", label: t("ui.settings.sections.players") },
    { id: "appearance", label: t("ui.settings.sections.appearance") },
    { id: "advanced", label: t("ui.settings.sections.advanced") },
    {
      id: "help",
      label: t("ui.settings.sections.help"),
      children: [
        { id: "help-docs", label: t("ui.help.title") },
        { id: "help-about", label: t("ui.settings.about.title") },
      ],
    },
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
    return line === "primary" ? t("ui.settings.line.primary") : t("ui.settings.line.translation");
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

/**
 * relabeled is a draft with the words the app gave it taken from another
 * reading of the profile: the shortcuts' names, groups and keys, in the
 * language of the moment. A key the draft changed keeps its own label.
 */
export function relabeled(draft: ProfileSettings, saved: ProfileSettings): ProfileSettings {
  const words = new Map(saved.shortcuts.map((item) => [item.command, item]));
  return {
    ...draft,
    shortcuts: draft.shortcuts.map((item) => {
      const fresh = words.get(item.command);
      if (!fresh) {
        return item;
      }
      return { ...item, name: fresh.name, category: fresh.category, label: fresh.chord === item.chord ? fresh.label : item.label };
    }),
  };
}

/** clampNumber keeps a typed number inside its range; text that is no number keeps the old value. */
export function clampNumber(text: string, min: number, max: number, fallback: number): number {
  const value = Number(text.replace(",", "."));
  if (text.trim() === "" || !Number.isFinite(value)) {
    return fallback;
  }
  return Math.min(Math.max(value, min), max);
}

/** NoticeSection is one license of the third-party notices: what it covers, and its text as its authors wrote it. */
export interface NoticeSection {
  title: string;
  text: string;
}

/** noticeRule opens each license in the notices, above the name and version of what it covers. */
const noticeRule = "=".repeat(78);

/** noticesOf splits the third-party notices into their licenses, so each shows under its own title. */
export function noticesOf(notices: string): NoticeSection[] {
  const lines = notices.split("\n");
  const sections: NoticeSection[] = [];
  for (let at = 0; at < lines.length; at++) {
    if (lines[at] !== noticeRule) {
      continue;
    }
    // The title, then a rule of dashes, then the license up to the next one.
    let end = at + 3;
    while (end < lines.length && lines[end] !== noticeRule) {
      end++;
    }
    sections.push({
      title: lines[at + 1] ?? "",
      text: lines
        .slice(at + 3, end)
        .join("\n")
        .trim(),
    });
    at = end - 1;
  }
  return sections;
}

/** sectionOfPage is the settings page a request names ("subtitles-primary"), or the first one for a name it does not know. */
export function sectionOfPage(page: string | undefined): SectionId {
  const ids: string[] = [];
  const walk = (items: SectionItem[]) =>
    items.forEach((item) => {
      ids.push(item.id);
      walk(item.children ?? []);
    });
  walk(settingsSections({ primary: "", translation: "" }));
  return page !== undefined && ids.includes(page) ? (page as SectionId) : "general";
}
