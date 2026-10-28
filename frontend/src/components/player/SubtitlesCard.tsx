import { useState } from "react";
import { t } from "../../i18n";
import type { Line, Playback } from "../../services/backend";
import { Card } from "../ui/Feedback";
import { KeyedTip } from "../ui/HelpTip";
import { Select } from "../ui/Select";
import { forLine, noteOf, trackOptions } from "../../tracks";
import { SubtitlePicker } from "./SubtitlePicker";

interface SubtitlesCardProps {
  state: Playback;
  onCommand: (command: () => Promise<void>) => void;
  commands: {
    switchSubtitle: (line: Line, track: number) => Promise<void>;
    switchAudio: (track: number) => Promise<void>;
    addSubtitleFile: (line: Line) => Promise<void>;
  };
}

/** SubtitlesCard switches the subtitles and the audio while the video plays, and adds subtitle files. */
export function SubtitlesCard({ state, onCommand, commands }: SubtitlesCardProps) {
  const { tracks } = state;
  const [adding, setAdding] = useState<Line | "">("");
  const add = (line: Line) =>
    onCommand(async () => {
      setAdding(line);
      try {
        await commands.addSubtitleFile(line);
      } finally {
        setAdding("");
      }
    });
  const primary = String(tracks.current.primary);
  const translation = String(tracks.current.translation);
  const loading = tracks.loading ? t("ui.playing.subtitles.loading") : undefined;

  return (
    <Card title={t("ui.playing.subtitles.title")} aside={<KeyedTip k="ui.playing.subtitles" />}>
      <div className="@container">
        <div className="grid grid-cols-[minmax(0,1fr)] gap-5 [&>*]:min-w-0 @md:grid-cols-2">
          <SubtitlePicker
            label={t("ui.playing.subtitles.primary")}
            value={primary}
            options={trackOptions(tracks.subtitles, t("ui.playing.subtitles.none"))}
            disabled={tracks.loading}
            onChange={(value) => onCommand(() => commands.switchSubtitle("primary", Number(value)))}
            onAdd={() => add("primary")}
            adding={adding === "primary"}
            {...noteOf(tracks.subtitles, primary)}
            {...(loading ? { note: loading, warning: false } : {})}
          />
          <SubtitlePicker
            label={t("ui.playing.subtitles.translation")}
            value={translation}
            options={trackOptions(forLine(tracks.subtitles, tracks.current.translation), t("ui.playing.subtitles.none"))}
            disabled={tracks.loading}
            onChange={(value) => onCommand(() => commands.switchSubtitle("translation", Number(value)))}
            onAdd={() => add("translation")}
            adding={adding === "translation"}
            {...noteOf(tracks.subtitles, translation)}
          />
        </div>
      </div>
      {tracks.audio.length > 1 && (
        <div className="mt-5">
          <Select
            label={t("ui.playing.audio.label")}
            value={String(tracks.current.audio)}
            options={[
              ...(tracks.current.audio === 0 ? [{ value: "0", label: t("ui.playing.audio.player") }] : []),
              ...tracks.audio.map((track) => ({ value: String(track.number), label: track.label })),
            ]}
            disabled={!tracks.audioSwitch}
            note={tracks.audioSwitch ? undefined : t("ui.playing.audio.fixed")}
            onChange={(value) => onCommand(() => commands.switchAudio(Number(value)))}
          />
        </div>
      )}
    </Card>
  );
}
