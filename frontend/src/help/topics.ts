// The documentation: what the user can do in the app, topic by topic, in
// the interface's language (help.json in each catalog folder). Every key is
// written whole, so the catalog test finds it; the keys of the commands are
// the profile's own.

import { t } from "../i18n";
import { both, keyLabel } from "../keys";
import type { SectionId } from "../settings";

/** Keys are the profile's shortcuts: each command and its keys as shown. */
export type Keys = { command: string; label: string }[];

/** Topic is one subject of the documentation; open is the settings page it is about, if any. */
export interface Topic {
  id: string;
  title: string;
  paragraphs: string[];
  open?: SectionId;
}

/** topics are the documentation's subjects in reading order, worded now, with the keys of the profile. */
export function topics(keys: Keys): Topic[] {
  const key = (command: string) => keyLabel(keys, command) || t("ui.help.noKey");
  const pair = (first: string, second: string) => both(keyLabel(keys, first), keyLabel(keys, second)) || t("ui.help.noKey");
  return [
    {
      id: "start",
      title: t("ui.help.start.title"),
      paragraphs: [t("ui.help.start.p1"), t("ui.help.start.p2"), t("ui.help.start.p3"), t("ui.help.start.p4")],
      open: "general",
    },
    {
      id: "watch",
      title: t("ui.help.watch.title"),
      paragraphs: [
        t("ui.help.watch.p1", {
          play: key("play-pause"),
          seek: pair("seek-back", "seek-forward"),
          frames: pair("frame-back", "frame-forward"),
          fullscreen: key("fullscreen"),
        }),
        t("ui.help.watch.p2", { volume: pair("volume-up", "volume-down") }),
        t("ui.help.watch.p3"),
        t("ui.help.watch.p4"),
      ],
      open: "playback",
    },
    {
      id: "subtitles",
      title: t("ui.help.subtitles.title"),
      paragraphs: [t("ui.help.subtitles.p1"), t("ui.help.subtitles.p2"), t("ui.help.subtitles.p3")],
      open: "subtitles",
    },
    {
      id: "sync",
      title: t("ui.help.sync.title"),
      paragraphs: [
        t("ui.help.sync.p1", {
          primary: pair("sync-primary-earlier", "sync-primary-later"),
          translation: pair("sync-translation-earlier", "sync-translation-later"),
        }),
        t("ui.help.sync.p2"),
        t("ui.help.sync.p3"),
      ],
      open: "subtitles",
    },
    {
      id: "modes",
      title: t("ui.help.modes.title"),
      paragraphs: [
        t("ui.help.modes.p1", { key: key("translation") }),
        t("ui.help.modes.p2"),
        t("ui.help.modes.p3", {
          phrase: key("repeat-phrase"),
          last:
            [keyLabel(keys, "repeat-last-3"), keyLabel(keys, "repeat-last-5"), keyLabel(keys, "repeat-last-10")]
              .filter((label) => label !== "")
              .join(" / ") || t("ui.help.noKey"),
        }),
      ],
      open: "study",
    },
    {
      id: "karaoke",
      title: t("ui.help.karaoke.title"),
      paragraphs: [t("ui.help.karaoke.p1"), t("ui.help.karaoke.p2", { key: key("replay-line") }), t("ui.help.karaoke.p3")],
      open: "study",
    },
    {
      id: "passages",
      title: t("ui.help.passages.title"),
      paragraphs: [
        t("ui.help.passages.p1", { start: key("passage-mark-start"), end: key("passage-mark-end") }),
        t("ui.help.passages.p2"),
        t("ui.help.passages.p3"),
      ],
    },
    {
      id: "editing",
      title: t("ui.help.editing.title"),
      paragraphs: [t("ui.help.editing.p1"), t("ui.help.editing.p2", { keys: pair("undo", "redo") }), t("ui.help.editing.p3")],
    },
    {
      id: "shortcuts",
      title: t("ui.help.shortcuts.title"),
      paragraphs: [t("ui.help.shortcuts.p1"), t("ui.help.shortcuts.p2"), t("ui.help.shortcuts.p3")],
      open: "shortcuts",
    },
    {
      id: "profiles",
      title: t("ui.help.profiles.title"),
      paragraphs: [t("ui.help.profiles.p1"), t("ui.help.profiles.p2"), t("ui.help.profiles.p3")],
      open: "appearance",
    },
    {
      id: "voice",
      title: t("ui.help.voice.title"),
      paragraphs: [t("ui.help.voice.p1"), t("ui.help.voice.p2"), t("ui.help.voice.p3"), t("ui.help.voice.p4")],
      open: "study",
    },
    {
      id: "computer",
      title: t("ui.help.computer.title"),
      paragraphs: [t("ui.help.computer.p1"), t("ui.help.computer.p2")],
      open: "general",
    },
    {
      id: "players",
      title: t("ui.help.players.title"),
      paragraphs: [t("ui.help.players.p1"), t("ui.help.players.p2")],
      open: "players",
    },
    {
      id: "storage",
      title: t("ui.help.storage.title"),
      paragraphs: [t("ui.help.storage.p1"), t("ui.help.storage.p2")],
      open: "advanced",
    },
    {
      id: "problems",
      title: t("ui.help.problems.title"),
      paragraphs: [t("ui.help.problems.p1"), t("ui.help.problems.p2"), t("ui.help.problems.p3")],
      open: "advanced",
    },
  ];
}

/** plain is a text for searching: lower case, without accents ("karaokê" → "karaoke"). */
function plain(text: string): string {
  return text
    .normalize("NFD")
    .replace(/\p{Diacritic}/gu, "")
    .toLowerCase();
}

/** searchTopics keeps the topics whose title or text holds every word of the query, accents aside. */
export function searchTopics(all: Topic[], query: string): Topic[] {
  const words = plain(query).split(/\s+/).filter(Boolean);
  if (words.length === 0) {
    return all;
  }
  return all.filter((topic) => {
    const text = plain([topic.title, ...topic.paragraphs].join(" "));
    return words.every((word) => text.includes(word));
  });
}
