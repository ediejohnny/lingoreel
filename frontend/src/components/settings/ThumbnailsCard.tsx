import { Trash2 } from "lucide-react";
import { language, t } from "../../i18n";
import { backend, type ThumbnailSettings } from "../../services/backend";
import { Button } from "../ui/Button";
import { Banner, Card } from "../ui/Feedback";
import { Toggle } from "../ui/Field";
import { KeyedTip } from "../ui/HelpTip";
import { Segmented } from "../ui/Inputs";
import { FFmpegRow } from "./FFmpegRow";
import type { SectionProps } from "./section";

/** megabytes words a size on the disk in MB, with one decimal at most. */
function megabytes(bytes: number): string {
  return new Intl.NumberFormat(language(), { maximumFractionDigits: 1 }).format(bytes / 1_000_000);
}

/**
 * ThumbnailsCard is the seek bar's stills on this computer, for every profile: whether they show, whether they are
 * taken on the battery too, whether they are kept on the disk (with how much they take and clearing them), and the
 * FFmpeg that takes them. Each choice applies at once, to the video being watched too.
 */
export function ThumbnailsCard({ view, run }: Pick<SectionProps, "view" | "run">) {
  const thumbnails = view.thumbnails;
  const save = (next: Partial<ThumbnailSettings>) => void run(() => backend.setThumbnails({ ...thumbnails, ...next }));

  return (
    <Card title={t("ui.settings.thumbnails.title")} aside={<KeyedTip k="ui.settings.thumbnails" />}>
      <div className="flex flex-col gap-5">
        <Toggle label={t("ui.settings.thumbnails.on.label")} checked={thumbnails.on} onChange={(on) => save({ on })} />
        {thumbnails.battery && (
          <Toggle
            label={t("ui.settings.thumbnails.battery.label")}
            checked={thumbnails.onBattery}
            disabled={!thumbnails.on}
            onChange={(onBattery) => save({ onBattery })}
            help={<KeyedTip k="ui.settings.thumbnails.battery" />}
          />
        )}
        <Segmented
          label={t("ui.settings.thumbnails.keep.label")}
          value={thumbnails.onDisk ? "disk" : "memory"}
          options={[
            { value: "disk", label: t("ui.settings.thumbnails.keep.disk") },
            { value: "memory", label: t("ui.settings.thumbnails.keep.memory") },
          ]}
          disabled={!thumbnails.on}
          onChange={(keep) => save({ onDisk: keep === "disk" })}
          help={<KeyedTip k="ui.settings.thumbnails.keep" />}
        />
        {thumbnails.on && thumbnails.onDisk && thumbnails.crowded && <Banner tone="warning">{t("ui.settings.thumbnails.crowded")}</Banner>}
        <div className="flex flex-wrap items-center gap-3">
          <p className="text-sm text-fg-muted">{t("ui.settings.thumbnails.kept", { size: megabytes(thumbnails.keptBytes) })}</p>
          <Button
            icon={Trash2}
            disabled={thumbnails.keptBytes === 0}
            onClick={() => void run(() => backend.clearThumbnails(), t("ui.settings.thumbnails.cleared"))}
          >
            {t("ui.settings.thumbnails.clear")}
          </Button>
        </div>
        <FFmpegRow
          voice={view.voice}
          run={run}
          disabled={!thumbnails.on}
          found={(path) => t("ui.settings.thumbnails.ffmpeg.found", { path })}
          missing={t("ui.settings.thumbnails.ffmpeg.missing")}
          help={t("ui.settings.thumbnails.ffmpeg.help")}
        />
      </div>
    </Card>
  );
}
