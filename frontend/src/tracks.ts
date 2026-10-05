import type { Option } from "./components/ui/Select";
import type { Line, Track } from "./services/backend";

/** none is a list's value for no track. */
export const none = "0";

/** trackOptions lists the subtitle tracks, after the choice of none when the line may have none. */
export function trackOptions(tracks: Track[], noneLabel: string, withNone = true): Option[] {
  return [
    ...(withNone ? [{ value: none, label: noneLabel }] : []),
    ...tracks.map((track) => ({ value: String(track.number), label: track.label, disabled: !track.readable })),
  ];
}

/**
 * playingOptions are a line's choices while the video plays. None is one
 * below; on top only while the line shows none (a session may start without
 * a subtitle there), as the session never takes the one on top away.
 */
export function playingOptions(line: Line, tracks: Track[], current: number, noneLabel: string): Option[] {
  return trackOptions(tracks, noneLabel, line === "translation" || current === 0);
}

/** noteOf is what the app found about the chosen track, for the note below the list. */
export function noteOf(tracks: Track[], value: string): { note?: string; warning: boolean } {
  const track = tracks.find((candidate) => String(candidate.number) === value);
  return { note: track?.note || undefined, warning: track?.warning ?? false };
}
