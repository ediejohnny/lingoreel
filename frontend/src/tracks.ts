import type { Option } from "./components/ui/Select";
import type { Track } from "./services/backend";

/** none is a list's value for no track. */
export const none = "0";

/** trackOptions lists the subtitle tracks, after the choice of none: either line may show none. */
export function trackOptions(tracks: Track[], noneLabel: string): Option[] {
  return [
    { value: none, label: noneLabel },
    ...tracks.map((track) => ({ value: String(track.number), label: track.label, disabled: !track.readable })),
  ];
}

/**
 * forLine brings the language of a line's track to the top of its list, keeping the app's order (best first) within
 * each language: the app lists the top line's language first, so the line below would show it first too. Every track
 * stays in the list.
 */
export function forLine(tracks: Track[], lineTrack: number): Track[] {
  const base = (track: Track) => track.language.split("-")[0];
  const chosen = tracks.find((track) => track.number === lineTrack);
  if (!chosen || !base(chosen)) {
    return tracks;
  }
  const same = (track: Track) => base(track) === base(chosen);
  return [...tracks.filter(same), ...tracks.filter((track) => !same(track))];
}

/** noteOf is what the app found about the chosen track, for the note below the list. */
export function noteOf(tracks: Track[], value: string): { note?: string; warning: boolean } {
  const track = tracks.find((candidate) => String(candidate.number) === value);
  return { note: track?.note || undefined, warning: track?.warning ?? false };
}
